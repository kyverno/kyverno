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

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/background/common"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	urlister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	pkgcommon "github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/event"
	kyvernoutils "github.com/kyverno/kyverno/pkg/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	corelister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

type GenerateController struct {
	//	GenerateController updaterequest.GenerateController
	client dclient.Interface

	// typed client for Kyverno CRDs
	kyvernoClient kyvernoclient.Interface

	// urStatusControl is used to update UR status
	statusControl common.StatusControlInterface

	// event generator interface
	eventGen event.Interface

	log logr.Logger

	// urLister can list/get update request from the shared informer's store
	urLister urlister.UpdateRequestNamespaceLister

	// nsLister can list/get namespaces from the shared informer's store
	nsLister corelister.NamespaceLister

	// policyLister can list/get cluster policy from the shared informer's store
	policyLister kyvernolister.ClusterPolicyLister

	// policyLister can list/get Namespace policy from the shared informer's store
	npolicyLister kyvernolister.PolicyLister

	Config config.Configuration
}

//NewGenerateController returns an instance of the Generate-Request Controller
func NewGenerateController(
	kyvernoClient kyvernoclient.Interface,
	client dclient.Interface,
	policyLister kyvernolister.ClusterPolicyLister,
	npolicyLister kyvernolister.PolicyLister,
	urLister urlister.UpdateRequestNamespaceLister,
	eventGen event.Interface,
	nsLister corelister.NamespaceLister,
	log logr.Logger,
	dynamicConfig config.Configuration,
) (*GenerateController, error) {

	c := GenerateController{
		client:        client,
		kyvernoClient: kyvernoClient,
		eventGen:      eventGen,
		log:           log,
		Config:        dynamicConfig,
		policyLister:  policyLister,
		npolicyLister: npolicyLister,
		urLister:      urLister,
	}

	c.statusControl = common.NewStatusControl(kyvernoClient, urLister)
	c.nsLister = nsLister

	return &c, nil
}

func (c *GenerateController) ProcessUR(ur *kyvernov1beta1.UpdateRequest) error {
	logger := c.log.WithValues("name", ur.Name, "policy", ur.Spec.Policy, "kind", ur.Spec.Resource.Kind, "apiVersion", ur.Spec.Resource.APIVersion, "namespace", ur.Spec.Resource.Namespace, "name", ur.Spec.Resource.Name)
	var err error
	var resource *unstructured.Unstructured
	var genResources []kyverno.ResourceSpec
	var precreatedResource bool
	logger.Info("start processing UR", "ur", ur.Name, "resourceVersion", ur.GetResourceVersion())

	// 1 - Check if the trigger exists
	resource, err = common.GetResource(c.client, ur.Spec, c.log)
	if err != nil {
		// Don't update status
		// re-queueing the UR by updating the annotation
		// retry - 5 times
		logger.V(3).Info("resource does not exist or is pending creation, re-queueing", "details", err.Error(), "retry")
		urAnnotations := ur.Annotations

		if len(urAnnotations) == 0 {
			urAnnotations = map[string]string{
				urAnnotations[kyvernov1beta1.URGenerateRetryCountAnnotation]: "1",
			}
		} else {
			if val, ok := urAnnotations[kyvernov1beta1.URGenerateRetryCountAnnotation]; ok {
				sleepCountInt64, err := strconv.ParseUint(val, 10, 32)
				if err != nil {
					logger.Error(err, "unable to convert retry-count")
					return err
				}

				sleepCountInt := int(sleepCountInt64) + 1
				if sleepCountInt > 5 {
					if err := deleteGeneratedResources(logger, c.client, *ur); err != nil {
						return err
					}
					// - trigger-resource is deleted
					// - generated-resources are deleted
					// - > Now delete the UpdateRequest CR
					return c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).Delete(contextdefault.TODO(), ur.Name, metav1.DeleteOptions{})
				} else {
					time.Sleep(time.Second * time.Duration(sleepCountInt))
					incrementedCountString := strconv.Itoa(sleepCountInt)
					urAnnotations[kyvernov1beta1.URGenerateRetryCountAnnotation] = incrementedCountString
				}

			} else {
				time.Sleep(time.Second * 1)
				urAnnotations[kyvernov1beta1.URGenerateRetryCountAnnotation] = "1"
			}
		}

		ur.SetAnnotations(urAnnotations)
		_, err := c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).Update(contextdefault.TODO(), ur, metav1.UpdateOptions{})
		if err != nil {
			logger.Error(err, "failed to update annotation in update request for the resource", "update request", ur.Name, "resourceVersion", ur.GetResourceVersion())
			return err
		}

		return err
	}

	// trigger resource is being terminated
	if resource == nil {
		return nil
	}

	// 2 - Apply the generate policy on the resource
	namespaceLabels := pkgcommon.GetNamespaceSelectorsFromNamespaceLister(resource.GetKind(), resource.GetNamespace(), c.nsLister, logger)
	genResources, precreatedResource, err = c.applyGenerate(*resource, *ur, namespaceLabels)

	if err != nil {
		// Need not update the status when policy doesn't apply on resource, because all the update requests are removed by the cleanup controller
		if strings.Contains(err.Error(), doesNotApply) {
			logger.V(4).Info("skipping updating status of update request")
			return nil
		}

		// 3 - Report failure Events
		events := event.NewBackgroundFailedEvent(err, ur.Spec.Policy, "", event.GeneratePolicyController, resource)
		c.eventGen.Add(events...)
	}

	// 4 - Update Status
	return updateStatus(c.statusControl, *ur, err, genResources, precreatedResource)
}

const doesNotApply = "policy does not apply to resource"

func (c *GenerateController) applyGenerate(resource unstructured.Unstructured, ur kyvernov1beta1.UpdateRequest, namespaceLabels map[string]string) ([]kyverno.ResourceSpec, bool, error) {
	logger := c.log.WithValues("name", ur.GetName(), "policy", ur.Spec.Policy, "kind", ur.Spec.Resource.Kind, "apiVersion", ur.Spec.Resource.APIVersion, "namespace", ur.Spec.Resource.Namespace, "name", ur.Spec.Resource.Name)
	logger.V(3).Info("applying generate policy rule")

	policy, err := c.getPolicySpec(ur)
	if err != nil {
		if apierrors.IsNotFound(err) {
			for _, e := range ur.Status.GeneratedResources {
				if err := c.cleanupClonedResource(e); err != nil {
					logger.Error(err, "failed to clean up cloned resource on policy deletion")
				}
			}
			return nil, false, nil
		}

		logger.Error(err, "error in fetching policy")
		return nil, false, err
	}

	policyContext, precreatedResource, err := common.NewBackgroundContext(c.client, &ur, &policy, &resource, c.Config, namespaceLabels, logger)
	if err != nil {
		return nil, precreatedResource, err
	}

	// check if the policy still applies to the resource
	engineResponse := engine.GenerateResponse(policyContext, ur)
	if len(engineResponse.PolicyResponse.Rules) == 0 {
		logger.V(4).Info(doesNotApply)
		return nil, false, errors.New(doesNotApply)
	}

	var applicableRules []string
	// Removing UR if rule is failed. Used when the generate condition failed but ur exist
	for _, r := range engineResponse.PolicyResponse.Rules {
		if r.Status != response.RuleStatusPass {
			logger.V(4).Info("querying all update requests")
			selector := labels.SelectorFromSet(labels.Set(map[string]string{
				kyvernov1beta1.URGeneratePolicyLabel:       engineResponse.PolicyResponse.Policy.Name,
				kyvernov1beta1.URGenerateResourceNameLabel: engineResponse.PolicyResponse.Resource.Name,
				kyvernov1beta1.URGenerateResourceKindLabel: engineResponse.PolicyResponse.Resource.Kind,
				kyvernov1beta1.URGenerateResourceNSLabel:   engineResponse.PolicyResponse.Resource.Namespace,
			}))
			urList, err := c.urLister.List(selector)
			if err != nil {
				logger.Error(err, "failed to get update request for the resource", "kind", engineResponse.PolicyResponse.Resource.Kind, "name", engineResponse.PolicyResponse.Resource.Name, "namespace", engineResponse.PolicyResponse.Resource.Namespace)
				continue
			}

			for _, v := range urList {
				err := c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).Delete(contextdefault.TODO(), v.GetName(), metav1.DeleteOptions{})
				if err != nil {
					logger.Error(err, "failed to delete update request")
				}
			}
		} else {
			applicableRules = append(applicableRules, r.Name)
		}
	}

	// Apply the generate rule on resource
	return c.applyGeneratePolicy(logger, policyContext, ur, applicableRules)
}

// cleanupClonedResource deletes cloned resource if sync is not enabled for the clone policy
func (c *GenerateController) cleanupClonedResource(targetSpec kyverno.ResourceSpec) error {
	target, err := c.client.GetResource(targetSpec.APIVersion, targetSpec.Kind, targetSpec.Namespace, targetSpec.Name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to find generated resource %s/%s: %v", targetSpec.Namespace, targetSpec.Name, err)
		}
	}

	if target == nil {
		return nil
	}

	labels := target.GetLabels()
	syncEnabled := labels["policy.kyverno.io/synchronize"] == "enable"
	clone := labels["generate.kyverno.io/clone-policy-name"] != ""

	if syncEnabled && !clone {
		if err := c.client.DeleteResource(target.GetAPIVersion(), target.GetKind(), target.GetNamespace(), target.GetName(), false); err != nil {
			return fmt.Errorf("cloned resource is not deleted %s/%s: %v", targetSpec.Namespace, targetSpec.Name, err)
		}
	}
	return nil
}

// getPolicySpec gets the policy spec from the ClusterPolicy/Policy
func (c *GenerateController) getPolicySpec(ur kyvernov1beta1.UpdateRequest) (kyverno.ClusterPolicy, error) {
	var policy kyverno.ClusterPolicy

	pNamespace, pName, err := cache.SplitMetaNamespaceKey(ur.Spec.Policy)
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

func updateStatus(statusControl common.StatusControlInterface, ur kyvernov1beta1.UpdateRequest, err error, genResources []kyverno.ResourceSpec, precreatedResource bool) error {
	if err != nil {
		if _, err := statusControl.Failed(ur.GetName(), err.Error(), genResources); err != nil {
			return err
		}
	} else if precreatedResource {
		if _, err := statusControl.Skip(ur.GetName(), genResources); err != nil {
			return err
		}
	} else {
		if _, err := statusControl.Success(ur.GetName(), genResources); err != nil {
			return err
		}
	}
	return nil
}

func (c *GenerateController) applyGeneratePolicy(log logr.Logger, policyContext *engine.PolicyContext, ur kyvernov1beta1.UpdateRequest, applicableRules []string) (genResources []kyverno.ResourceSpec, processExisting bool, err error) {
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

		if policy.GetSpec().IsGenerateExistingOnPolicyUpdate() || !processExisting {
			genResource, err = applyRule(log, c.client, rule, resource, jsonContext, policy, ur)
			if err != nil {
				log.Error(err, "failed to apply generate rule", "policy", policy.GetName(),
					"rule", rule.Name, "resource", resource.GetName(), "suggestion", "users need to grant Kyverno's service account additional privileges")
				return nil, processExisting, err
			}
			ruleNameToProcessingTime[rule.Name] = time.Since(startTime)
			genResources = append(genResources, genResource)
		}

		if policy.GetSpec().IsGenerateExistingOnPolicyUpdate() {
			processExisting = false
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

func applyRule(log logr.Logger, client dclient.Interface, rule kyverno.Rule, resource unstructured.Unstructured, ctx context.EvalInterface, policy kyverno.PolicyInterface, ur kyvernov1beta1.UpdateRequest) (kyverno.ResourceSpec, error) {
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

	if len(genClone) != 0 {
		rdata, mode, err = manageClone(logger, genAPIVersion, genKind, genNamespace, genName, policy.GetName(), genClone, client)
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

	// Add background gen-rule label if generate rule applied on existing resource
	if policy.GetSpec().IsGenerateExistingOnPolicyUpdate() {
		label["kyverno.io/background-gen-rule"] = rule.Name
	}

	label["policy.kyverno.io/policy-name"] = policy.GetName()
	label["policy.kyverno.io/gr-name"] = ur.Name
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

func manageData(log logr.Logger, apiVersion, kind, namespace, name string, data map[string]interface{}, client dclient.Interface) (map[string]interface{}, ResourceMode, error) {
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

func manageClone(log logr.Logger, apiVersion, kind, namespace, name, policy string, clone map[string]interface{}, client dclient.Interface) (map[string]interface{}, ResourceMode, error) {
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

func deleteGeneratedResources(log logr.Logger, client dclient.Interface, ur kyvernov1beta1.UpdateRequest) error {
	for _, genResource := range ur.Status.GeneratedResources {
		err := client.DeleteResource("", genResource.Kind, genResource.Namespace, genResource.Name, false)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}

		log.V(3).Info("generated resource deleted", "genKind", ur.Spec.Resource.Kind, "genNamespace", ur.Spec.Resource.Namespace, "genName", ur.Spec.Resource.Name)
	}
	return nil
}
