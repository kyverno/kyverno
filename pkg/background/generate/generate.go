package generate

import (
	contextdefault "context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/background/common"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/event"

	"github.com/go-logr/logr"
	pkgcommon "github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	kyvernoutils "github.com/kyverno/kyverno/pkg/utils"
	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	coreinformers "k8s.io/client-go/informers/core/v1"
	corelister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

type GenerateController struct {
	//	GenerateController updaterequest.GenerateController
	client *dclient.Client

	// typed client for Kyverno CRDs
	kyvernoClient *kyvernoclient.Clientset

	// grStatusControl is used to update GR status
	statusControl common.StatusControlInterface

	// event generator interface
	eventGen event.Interface

	log logr.Logger

	// grLister can list/get generate request from the shared informer's store
	grLister kyvernolister.GenerateRequestNamespaceLister

	// nsLister can list/get namespaces from the shared informer's store
	nsLister corelister.NamespaceLister

	// policyLister can list/get cluster policy from the shared informer's store
	policyLister kyvernolister.ClusterPolicyLister

	// policyLister can list/get Namespace policy from the shared informer's store
	npolicyLister kyvernolister.PolicyLister

	Config config.Interface
}

//NewGenerateController returns an instance of the Generate-Request Controller
func NewGenerateController(
	kyvernoClient *kyvernoclient.Clientset,
	client *dclient.Client,
	policyLister kyvernolister.ClusterPolicyLister,
	npolicyLister kyvernolister.PolicyLister,
	grLister kyvernolister.GenerateRequestNamespaceLister,
	eventGen event.Interface,
	namespaceInformer coreinformers.NamespaceInformer,
	log logr.Logger,
	dynamicConfig config.Interface,
) (*GenerateController, error) {

	c := GenerateController{
		client:        client,
		kyvernoClient: kyvernoClient,
		eventGen:      eventGen,
		log:           log,
		Config:        dynamicConfig,
		policyLister:  policyLister,
		npolicyLister: npolicyLister,
		grLister:      grLister,
	}

	c.statusControl = common.StatusControl{Client: kyvernoClient}
	c.nsLister = namespaceInformer.Lister()

	return &c, nil
}

func (c *GenerateController) ProcessGR(gr *kyverno.GenerateRequest) error {
	logger := c.log.WithValues("name", gr.Name, "policy", gr.Spec.Policy, "kind", gr.Spec.Resource.Kind, "apiVersion", gr.Spec.Resource.APIVersion, "namespace", gr.Spec.Resource.Namespace, "name", gr.Spec.Resource.Name)
	var err error
	var resource *unstructured.Unstructured
	var genResources []kyverno.ResourceSpec
	var precreatedResource bool

	// 1 - Check if the resource exists
	resource, err = getResource(c.client, gr.Spec.Resource, c.log)
	if err != nil {
		// Don't update status
		// re-queueing the GR by updating the annotation
		// retry - 5 times
		logger.V(3).Info("resource does not exist or is pending creation, re-queueing", "details", err.Error(), "retry")
		updateAnnotation := true
		grAnnotations := gr.Annotations

		if len(grAnnotations) == 0 {
			grAnnotations = make(map[string]string)
			grAnnotations["generate.kyverno.io/retry-count"] = "1"
		} else {
			if val, ok := grAnnotations["generate.kyverno.io/retry-count"]; ok {
				sleepCountInt64, err := strconv.ParseUint(val, 10, 32)
				if err != nil {
					logger.Error(err, "unable to convert retry-count")
					return err
				}

				sleepCountInt := int(sleepCountInt64) + 1
				if sleepCountInt > 5 {
					updateAnnotation = false
				} else {
					time.Sleep(time.Second * time.Duration(sleepCountInt))
					incrementedCountString := strconv.Itoa(sleepCountInt)
					grAnnotations["generate.kyverno.io/retry-count"] = incrementedCountString
				}

			} else {
				time.Sleep(time.Second * 1)
				grAnnotations["generate.kyverno.io/retry-count"] = "1"
			}
		}

		if updateAnnotation {
			gr.SetAnnotations(grAnnotations)
			_, err := c.kyvernoClient.KyvernoV1().GenerateRequests(config.KyvernoNamespace).Update(contextdefault.TODO(), gr, metav1.UpdateOptions{})
			if err != nil {
				logger.Error(err, "failed to update annotation in generate request for the resource", "generate request", gr.Name)
				return err
			}
		}

		return err
	}

	// trigger resource is being terminated
	if resource == nil {
		return nil
	}

	// 2 - Apply the generate policy on the resource
	namespaceLabels := pkgcommon.GetNamespaceSelectorsFromNamespaceLister(resource.GetKind(), resource.GetNamespace(), c.nsLister, logger)
	genResources, precreatedResource, err = c.applyGenerate(*resource, *gr, namespaceLabels)

	if err != nil {
		// Need not update the status when policy doesn't apply on resource, because all the generate requests are removed by the cleanup controller
		if strings.Contains(err.Error(), doesNotApply) {
			logger.V(4).Info("skipping updating status of generate request")
			return nil
		}

		// 3 - Report failure Events
		events := failedEvents(err, *gr, *resource)
		c.eventGen.Add(events...)
	}

	// 4 - Update Status
	return updateStatus(c.statusControl, *gr, err, genResources, precreatedResource)
}

const doesNotApply = "policy does not apply to resource"

func (c *GenerateController) applyGenerate(resource unstructured.Unstructured, gr kyverno.GenerateRequest, namespaceLabels map[string]string) ([]kyverno.ResourceSpec, bool, error) {
	logger := c.log.WithValues("name", gr.Name, "policy", gr.Spec.Policy, "kind", gr.Spec.Resource.Kind, "apiVersion", gr.Spec.Resource.APIVersion, "namespace", gr.Spec.Resource.Namespace, "name", gr.Spec.Resource.Name)
	// Get the list of rules to be applied
	// get policy
	// build context
	ctx := context.NewContext()

	logger.V(3).Info("applying generate policy rule")

	policy, err := c.getPolicySpec(gr)
	if err != nil {
		if apierrors.IsNotFound(err) {
			for _, e := range gr.Status.GeneratedResources {
				resp, err := c.client.GetResource(e.APIVersion, e.Kind, e.Namespace, e.Name)
				if err != nil && !apierrors.IsNotFound(err) {
					logger.Error(err, "failed to find generated resource", "name", e.Name)
					continue
				}

				if resp != nil && resp.GetLabels()["policy.kyverno.io/synchronize"] == "enable" {
					if err := c.client.DeleteResource(resp.GetAPIVersion(), resp.GetKind(), resp.GetNamespace(), resp.GetName(), false); err != nil {
						logger.Error(err, "generated resource is not deleted", "Resource", e.Name)
					}
				}
			}
			return nil, false, nil
		}

		logger.Error(err, "error in fetching policy")
		return nil, false, err
	}

	requestString := gr.Spec.Context.AdmissionRequestInfo.AdmissionRequest
	var request admissionv1.AdmissionRequest
	err = json.Unmarshal([]byte(requestString), &request)
	if err != nil {
		logger.Error(err, "error parsing the request string")
	}

	if gr.Spec.Context.AdmissionRequestInfo.Operation == admissionv1.Update {
		request.Operation = gr.Spec.Context.AdmissionRequestInfo.Operation
	}

	if err := ctx.AddRequest(&request); err != nil {
		logger.Error(err, "failed to load request in context")
		return nil, false, err
	}

	err = ctx.AddResource(resource.Object)
	if err != nil {
		logger.Error(err, "failed to load resource in context")
		return nil, false, err
	}

	err = ctx.AddUserInfo(gr.Spec.Context.UserRequestInfo)
	if err != nil {
		logger.Error(err, "failed to load SA in context")
		return nil, false, err
	}

	err = ctx.AddServiceAccount(gr.Spec.Context.UserRequestInfo.AdmissionUserInfo.Username)
	if err != nil {
		logger.Error(err, "failed to load UserInfo in context")
		return nil, false, err
	}

	if err := ctx.AddImageInfos(&resource); err != nil {
		logger.Error(err, "unable to add image info to variables context")
	}

	policyContext := &engine.PolicyContext{
		NewResource:         resource,
		Policy:              &policy,
		AdmissionInfo:       gr.Spec.Context.UserRequestInfo,
		ExcludeGroupRole:    c.Config.GetExcludeGroupRole(),
		ExcludeResourceFunc: c.Config.ToFilter,
		JSONContext:         ctx,
		NamespaceLabels:     namespaceLabels,
		Client:              c.client,
	}

	// check if the policy still applies to the resource
	engineResponse := engine.GenerateResponse(policyContext, gr)
	if len(engineResponse.PolicyResponse.Rules) == 0 {
		logger.V(4).Info(doesNotApply)
		return nil, false, errors.New(doesNotApply)
	}

	var applicableRules []string
	// Removing GR if rule is failed. Used when the generate condition failed but gr exist
	for _, r := range engineResponse.PolicyResponse.Rules {
		if r.Status != response.RuleStatusPass {
			logger.V(4).Info("querying all generate requests")
			selector := labels.SelectorFromSet(labels.Set(map[string]string{
				"generate.kyverno.io/policy-name":        engineResponse.PolicyResponse.Policy.Name,
				"generate.kyverno.io/resource-name":      engineResponse.PolicyResponse.Resource.Name,
				"generate.kyverno.io/resource-kind":      engineResponse.PolicyResponse.Resource.Kind,
				"generate.kyverno.io/resource-namespace": engineResponse.PolicyResponse.Resource.Namespace,
			}))
			grList, err := c.grLister.List(selector)
			if err != nil {
				logger.Error(err, "failed to get generate request for the resource", "kind", engineResponse.PolicyResponse.Resource.Kind, "name", engineResponse.PolicyResponse.Resource.Name, "namespace", engineResponse.PolicyResponse.Resource.Namespace)
				continue
			}

			for _, v := range grList {
				err := c.kyvernoClient.KyvernoV1().GenerateRequests(config.KyvernoNamespace).Delete(contextdefault.TODO(), v.GetName(), metav1.DeleteOptions{})
				if err != nil {
					logger.Error(err, "failed to delete generate request")
				}
			}
		} else {
			applicableRules = append(applicableRules, r.Name)
		}
	}

	// Apply the generate rule on resource
	return c.applyGeneratePolicy(logger, policyContext, gr, applicableRules)
}

// getPolicySpec gets the policy spec from the ClusterPolicy/Policy
func (c *GenerateController) getPolicySpec(gr kyverno.GenerateRequest) (kyverno.ClusterPolicy, error) {
	var policy kyverno.ClusterPolicy

	pNamespace, pName, err := cache.SplitMetaNamespaceKey(gr.Spec.Policy)
	if err != nil {
		return policy, err
	}

	if pNamespace == "" {
		policyObj, err := c.policyLister.Get(pName)
		if err != nil {
			return policy, err
		}
		return *policyObj, err
	} else {
		npolicyObj, err := c.npolicyLister.Policies(pNamespace).Get(pName)
		if err != nil {
			return policy, err
		}
		return kyverno.ClusterPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: pName,
			},
			Spec: npolicyObj.Spec,
		}, nil
	}
}

func updateStatus(statusControl common.StatusControlInterface, gr kyverno.GenerateRequest, err error, genResources []kyverno.ResourceSpec, precreatedResource bool) error {
	if err != nil {
		return statusControl.Failed(gr, err.Error(), genResources)
	} else if precreatedResource {
		return statusControl.Skip(gr, genResources)
	}

	// Generate request successfully processed
	return statusControl.Success(gr, genResources)
}

func (c *GenerateController) applyGeneratePolicy(log logr.Logger, policyContext *engine.PolicyContext, gr kyverno.GenerateRequest, applicableRules []string) (genResources []kyverno.ResourceSpec, processExisting bool, err error) {
	// Get the response as the actions to be performed on the resource
	// - - substitute values
	policy := policyContext.Policy
	resource := policyContext.NewResource

	jsonContext := policyContext.JSONContext
	// To manage existing resources, we compare the creation time for the default resource to be generated and policy creation time

	ruleNameToProcessingTime := make(map[string]time.Duration)
	for _, rule := range autogen.ComputeRules(policy) {
		var err error
		if !rule.HasGenerate() {
			continue
		}

		if !kyvernoutils.ContainsString(applicableRules, rule.Name) {
			continue
		}

		startTime := time.Now()
		processExisting = false
		var genResource kyverno.ResourceSpec

		if len(rule.MatchResources.Kinds) > 0 {
			if len(rule.MatchResources.Annotations) == 0 && rule.MatchResources.Selector == nil {
				rcreationTime := resource.GetCreationTimestamp()
				pcreationTime := policy.GetCreationTimestamp()
				processExisting = rcreationTime.Before(&pcreationTime)
			}
		}

		// add configmap json data to context
		if err := engine.LoadContext(log, rule.Context, policyContext, rule.Name); err != nil {
			log.Error(err, "cannot add configmaps to context")
			return nil, processExisting, err
		}

		if rule, err = variables.SubstituteAllInRule(log, policyContext.JSONContext, rule); err != nil {
			log.Error(err, "variable substitution failed for rule %s", rule.Name)
			return nil, processExisting, err
		}

		if !processExisting {
			genResource, err = applyRule(log, c.client, rule, resource, jsonContext, policy.GetName(), gr)
			if err != nil {
				log.Error(err, "failed to apply generate rule", "policy", policy.GetName(),
					"rule", rule.Name, "resource", resource.GetName(), "suggestion", "users need to grant Kyverno's service account additional privileges")
				return nil, processExisting, err
			}
			ruleNameToProcessingTime[rule.Name] = time.Since(startTime)
			genResources = append(genResources, genResource)
		}
	}

	return genResources, processExisting, nil
}

func getResourceInfo(object map[string]interface{}) (kind, name, namespace, apiversion string, err error) {
	if kind, _, err = unstructured.NestedString(object, "kind"); err != nil {
		return "", "", "", "", err
	}

	if name, _, err = unstructured.NestedString(object, "name"); err != nil {
		return "", "", "", "", err
	}

	if namespace, _, err = unstructured.NestedString(object, "namespace"); err != nil {
		return "", "", "", "", err
	}

	if apiversion, _, err = unstructured.NestedString(object, "apiVersion"); err != nil {
		return "", "", "", "", err
	}

	return
}

func applyRule(log logr.Logger, client *dclient.Client, rule kyverno.Rule, resource unstructured.Unstructured, ctx context.EvalInterface, policy string, gr kyverno.GenerateRequest) (kyverno.ResourceSpec, error) {
	var rdata map[string]interface{}
	var err error
	var mode ResourceMode
	var noGenResource kyverno.ResourceSpec
	genUnst, err := getUnstrRule(rule.Generation.DeepCopy())
	if err != nil {
		return noGenResource, err
	}

	genKind, genName, genNamespace, genAPIVersion, err := getResourceInfo(genUnst.Object)
	if err != nil {
		return noGenResource, err
	}

	logger := log.WithValues("genKind", genKind, "genAPIVersion", genAPIVersion, "genNamespace", genNamespace, "genName", genName)

	// Resource to be generated
	newGenResource := kyverno.ResourceSpec{
		APIVersion: genAPIVersion,
		Kind:       genKind,
		Namespace:  genNamespace,
		Name:       genName,
	}

	genData, _, err := unstructured.NestedMap(genUnst.Object, "data")
	if err != nil {
		return noGenResource, fmt.Errorf("failed to read `data`: %v", err.Error())
	}

	genClone, _, err := unstructured.NestedMap(genUnst.Object, "clone")
	if err != nil {
		return noGenResource, fmt.Errorf("failed to read `clone`: %v", err.Error())
	}

	if genClone != nil && len(genClone) != 0 {
		rdata, mode, err = manageClone(logger, genAPIVersion, genKind, genNamespace, genName, policy, genClone, client)
	} else {
		rdata, mode, err = manageData(logger, genAPIVersion, genKind, genNamespace, genName, genData, client)
	}

	if err != nil {
		logger.Error(err, "failed to generate resource", "mode", mode)
		return newGenResource, err
	}

	logger.V(3).Info("applying generate rule", "mode", mode)

	if rdata == nil && mode == Update {
		logger.V(4).Info("no changes required for generate target resource")
		return newGenResource, nil
	}

	// build the resource template
	newResource := &unstructured.Unstructured{}
	newResource.SetUnstructuredContent(rdata)
	newResource.SetName(genName)
	newResource.SetNamespace(genNamespace)
	if newResource.GetKind() == "" {
		newResource.SetKind(genKind)
	}

	newResource.SetAPIVersion(genAPIVersion)
	// manage labels
	// - app.kubernetes.io/managed-by: kyverno
	// "kyverno.io/generated-by-kind": kind (trigger resource)
	// "kyverno.io/generated-by-namespace": namespace (trigger resource)
	// "kyverno.io/generated-by-name": name (trigger resource)
	common.ManageLabels(newResource, resource)
	// Add Synchronize label
	label := newResource.GetLabels()
	label["policy.kyverno.io/policy-name"] = policy
	label["policy.kyverno.io/gr-name"] = gr.Name
	delete(label, "generate.kyverno.io/clone-policy-name")
	if mode == Create {
		if rule.Generation.Synchronize {
			label["policy.kyverno.io/synchronize"] = "enable"
		} else {
			label["policy.kyverno.io/synchronize"] = "disable"
		}

		// Reset resource version
		newResource.SetResourceVersion("")
		newResource.SetLabels(label)
		// Create the resource
		_, err = client.CreateResource(genAPIVersion, genKind, genNamespace, newResource, false)
		if err != nil {
			return noGenResource, err
		}

		logger.V(2).Info("created generate target resource")

	} else if mode == Update {

		generatedObj, err := client.GetResource(genAPIVersion, genKind, genNamespace, genName)
		if err != nil {
			logger.Error(err, fmt.Sprintf("generated resource not found  name:%v namespace:%v kind:%v", genName, genNamespace, genKind))
			logger.V(2).Info(fmt.Sprintf("creating generate resource name:name:%v namespace:%v kind:%v", genName, genNamespace, genKind))
			_, err = client.CreateResource(genAPIVersion, genKind, genNamespace, newResource, false)
			if err != nil {
				return noGenResource, err
			}
		} else {
			// if synchronize is true - update the label and generated resource with generate policy data
			if rule.Generation.Synchronize {
				logger.V(4).Info("updating existing resource")
				label["policy.kyverno.io/synchronize"] = "enable"
				newResource.SetLabels(label)

				if genAPIVersion == "" {
					generatedResourceAPIVersion := generatedObj.GetAPIVersion()
					newResource.SetAPIVersion(generatedResourceAPIVersion)
				}
				if genNamespace == "" {
					newResource.SetNamespace("default")
				}

				if _, err := ValidateResourceWithPattern(logger, generatedObj.Object, newResource.Object); err != nil {
					_, err = client.UpdateResource(genAPIVersion, genKind, genNamespace, newResource, false)
					if err != nil {
						logger.Error(err, "failed to update resource")
						return noGenResource, err
					}
				}
			} else {
				currentGeneratedResourcelabel := generatedObj.GetLabels()
				currentSynclabel := currentGeneratedResourcelabel["policy.kyverno.io/synchronize"]

				// update only if the labels mismatches
				if (!rule.Generation.Synchronize && currentSynclabel == "enable") ||
					(rule.Generation.Synchronize && currentSynclabel == "disable") {
					logger.V(4).Info("updating label in existing resource")
					currentGeneratedResourcelabel["policy.kyverno.io/synchronize"] = "disable"
					generatedObj.SetLabels(currentGeneratedResourcelabel)

					_, err = client.UpdateResource(genAPIVersion, genKind, genNamespace, generatedObj, false)
					if err != nil {
						logger.Error(err, "failed to update label in existing resource")
						return noGenResource, err
					}
				}
			}
		}
		logger.V(3).Info("updated generate target resource")
	}

	return newGenResource, nil
}

func manageData(log logr.Logger, apiVersion, kind, namespace, name string, data map[string]interface{}, client *dclient.Client) (map[string]interface{}, ResourceMode, error) {
	obj, err := client.GetResource(apiVersion, kind, namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return data, Create, nil
		}

		log.Error(err, "failed to get resource")
		return nil, Skip, err
	}

	log.V(3).Info("found target resource", "resource", obj)
	if data == nil {
		log.V(3).Info("data is nil - skipping update", "resource", obj)
		return nil, Skip, nil
	}

	updateObj := &unstructured.Unstructured{}
	updateObj.SetUnstructuredContent(data)
	updateObj.SetResourceVersion(obj.GetResourceVersion())
	return updateObj.UnstructuredContent(), Update, nil
}

func manageClone(log logr.Logger, apiVersion, kind, namespace, name, policy string, clone map[string]interface{}, client *dclient.Client) (map[string]interface{}, ResourceMode, error) {
	rNamespace, _, err := unstructured.NestedString(clone, "namespace")
	if err != nil {
		return nil, Skip, fmt.Errorf("failed to find source namespace: %v", err)
	}

	rName, _, err := unstructured.NestedString(clone, "name")
	if err != nil {
		return nil, Skip, fmt.Errorf("failed to find source name: %v", err)
	}

	if rNamespace == namespace && rName == name {
		log.V(4).Info("skip resource self-clone")
		return nil, Skip, nil
	}

	// check if the resource as reference in clone exists?
	obj, err := client.GetResource(apiVersion, kind, rNamespace, rName)
	if err != nil {
		return nil, Skip, fmt.Errorf("source resource %s %s/%s/%s not found. %v", apiVersion, kind, rNamespace, rName, err)
	}
	// remove ownerReferences when cloning resources to other namespace
	if rNamespace != namespace && obj.GetOwnerReferences() != nil {
		obj.SetOwnerReferences(nil)
	}

	// check if resource to be generated exists
	newResource, err := client.GetResource(apiVersion, kind, namespace, name)
	if err == nil {
		obj.SetUID(newResource.GetUID())
		obj.SetSelfLink(newResource.GetSelfLink())
		obj.SetCreationTimestamp(newResource.GetCreationTimestamp())
		obj.SetManagedFields(newResource.GetManagedFields())
		obj.SetResourceVersion(newResource.GetResourceVersion())
		if reflect.DeepEqual(obj, newResource) {
			return nil, Skip, nil
		}
		return obj.UnstructuredContent(), Update, nil
	}

	// create the resource based on the reference clone
	return obj.UnstructuredContent(), Create, nil

}

// ResourceMode defines the mode for generated resource
type ResourceMode string

const (
	//Skip : failed to process rule, will not update the resource
	Skip ResourceMode = "SKIP"
	//Create : create a new resource
	Create = "CREATE"
	//Update : update/overwrite the new resource
	Update = "UPDATE"
)

func getUnstrRule(rule *kyverno.Generation) (*unstructured.Unstructured, error) {
	ruleData, err := json.Marshal(rule)
	if err != nil {
		return nil, err
	}
	return utils.ConvertToUnstructured(ruleData)
}
