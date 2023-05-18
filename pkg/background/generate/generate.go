package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/event"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	engineutils "github.com/kyverno/kyverno/pkg/utils/engine"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

type GenerateController struct {
	// clients
	client        dclient.Interface
	kyvernoClient versioned.Interface
	statusControl common.StatusControlInterface
	engine        engineapi.Engine

	// listers
	urLister      kyvernov1beta1listers.UpdateRequestNamespaceLister
	nsLister      corev1listers.NamespaceLister
	policyLister  kyvernov1listers.ClusterPolicyLister
	npolicyLister kyvernov1listers.PolicyLister

	configuration config.Configuration
	eventGen      event.Interface

	log logr.Logger
	jp  jmespath.Interface
}

// NewGenerateController returns an instance of the Generate-Request Controller
func NewGenerateController(
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	statusControl common.StatusControlInterface,
	engine engineapi.Engine,
	policyLister kyvernov1listers.ClusterPolicyLister,
	npolicyLister kyvernov1listers.PolicyLister,
	urLister kyvernov1beta1listers.UpdateRequestNamespaceLister,
	nsLister corev1listers.NamespaceLister,
	dynamicConfig config.Configuration,
	eventGen event.Interface,
	log logr.Logger,
	jp jmespath.Interface,
) *GenerateController {
	c := GenerateController{
		client:        client,
		kyvernoClient: kyvernoClient,
		statusControl: statusControl,
		engine:        engine,
		policyLister:  policyLister,
		npolicyLister: npolicyLister,
		urLister:      urLister,
		nsLister:      nsLister,
		configuration: dynamicConfig,
		eventGen:      eventGen,
		log:           log,
		jp:            jp,
	}
	return &c
}

func (c *GenerateController) ProcessUR(ur *kyvernov1beta1.UpdateRequest) error {
	logger := c.log.WithValues("name", ur.GetName(), "policy", ur.Spec.GetPolicyKey(), "resource", ur.Spec.GetResource().String())
	var err error
	var genResources []kyvernov1.ResourceSpec
	logger.Info("start processing UR", "ur", ur.Name, "resourceVersion", ur.GetResourceVersion())

	trigger, err := c.getTrigger(ur.Spec)
	if err != nil {
		logger.V(3).Info("the trigger resource does not exist or is pending creation, re-queueing", "details", err.Error())
		if err := updateRetryAnnotation(c.kyvernoClient, ur); err != nil {
			return err
		}
	}

	if trigger == nil {
		return nil
	}

	namespaceLabels := engineutils.GetNamespaceSelectorsFromNamespaceLister(trigger.GetKind(), trigger.GetNamespace(), c.nsLister, logger)
	genResources, err = c.applyGenerate(*trigger, *ur, namespaceLabels)
	if err != nil {
		// Need not update the status when policy doesn't apply on resource, because all the update requests are removed by the cleanup controller
		if strings.Contains(err.Error(), doesNotApply) {
			logger.V(4).Info("skipping updating status of update request")
			return nil
		}

		events := event.NewBackgroundFailedEvent(err, ur.Spec.Policy, "", event.GeneratePolicyController, trigger)
		c.eventGen.Add(events...)
	}

	return updateStatus(c.statusControl, *ur, err, genResources)
}

const doesNotApply = "policy does not apply to resource"

func (c *GenerateController) getTrigger(spec kyvernov1beta1.UpdateRequestSpec) (*unstructured.Unstructured, error) {
	admissionRequest := spec.Context.AdmissionRequestInfo.AdmissionRequest
	if admissionRequest == nil {
		return common.GetResource(c.client, spec, c.log)
	} else {
		operation := spec.Context.AdmissionRequestInfo.Operation
		if operation == admissionv1.Delete {
			return getTriggerForDeleteOperation(spec, c)
		} else if operation == admissionv1.Create {
			return getTriggerForCreateOperation(spec, c)
		} else {
			newResource, oldResource, err := admissionutils.ExtractResources(nil, *admissionRequest)
			if err != nil {
				c.log.Error(err, "failed to extract resources from admission review request")
				return nil, err
			}

			trigger := &newResource
			if newResource.Object == nil {
				trigger = &oldResource
			}
			return trigger, nil
		}
	}
}

func getTriggerForDeleteOperation(spec kyvernov1beta1.UpdateRequestSpec, c *GenerateController) (*unstructured.Unstructured, error) {
	request := spec.Context.AdmissionRequestInfo.AdmissionRequest
	_, oldResource, err := admissionutils.ExtractResources(nil, *request)
	if err != nil {
		return nil, fmt.Errorf("failed to load resource from context: %w", err)
	}
	labels := oldResource.GetLabels()
	if labels[common.GeneratePolicyLabel] != "" {
		// non-trigger deletion, get trigger from ur spec
		c.log.V(4).Info("non-trigger resource is deleted, fetching the trigger from the UR spec", "trigger", spec.Resource.String())
		return common.GetResource(c.client, spec, c.log)
	}
	return &oldResource, nil
}

func getTriggerForCreateOperation(spec kyvernov1beta1.UpdateRequestSpec, c *GenerateController) (*unstructured.Unstructured, error) {
	admissionRequest := spec.Context.AdmissionRequestInfo.AdmissionRequest
	trigger, err := common.GetResource(c.client, spec, c.log)
	if err != nil || trigger == nil {
		if admissionRequest.SubResource == "" {
			return nil, err
		} else {
			c.log.V(4).Info("trigger resource not found for subresource, reverting to resource in AdmissionReviewRequest", "subresource", admissionRequest.SubResource)
			newResource, _, err := admissionutils.ExtractResources(nil, *admissionRequest)
			if err != nil {
				c.log.Error(err, "failed to extract resources from admission review request")
				return nil, err
			}
			trigger = &newResource
		}
	}
	return trigger, err
}

func (c *GenerateController) applyGenerate(resource unstructured.Unstructured, ur kyvernov1beta1.UpdateRequest, namespaceLabels map[string]string) ([]kyvernov1.ResourceSpec, error) {
	logger := c.log.WithValues("name", ur.GetName(), "policy", ur.Spec.GetPolicyKey(), "resource", ur.Spec.GetResource().String())
	logger.V(3).Info("applying generate policy rule")

	policy, err := c.getPolicySpec(ur)
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "error in fetching policy")
		return nil, err
	}

	if ur.Spec.DeleteDownstream || apierrors.IsNotFound(err) {
		err = c.deleteDownstream(policy, &ur)
		return nil, err
	}

	policyContext, err := common.NewBackgroundContext(logger, c.client, &ur, policy, &resource, c.configuration, c.jp, namespaceLabels)
	if err != nil {
		return nil, err
	}

	admissionRequest := ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest
	if admissionRequest != nil {
		var gvk schema.GroupVersionKind
		gvk, err = c.client.Discovery().GetGVKFromGVR(schema.GroupVersionResource(admissionRequest.Resource))
		if err != nil {
			return nil, err
		}
		policyContext = policyContext.WithResourceKind(gvk, admissionRequest.SubResource)
	}

	// check if the policy still applies to the resource
	engineResponse := c.engine.Generate(context.Background(), policyContext)
	if len(engineResponse.PolicyResponse.Rules) == 0 {
		logger.V(4).Info(doesNotApply)
		return nil, errors.New(doesNotApply)
	}

	var applicableRules []string
	// Removing UR if rule is failed. Used when the generate condition failed but ur exist
	for _, r := range engineResponse.PolicyResponse.Rules {
		if r.Status() != engineapi.RuleStatusPass {
			logger.V(4).Info("querying all update requests")
			selector := labels.SelectorFromSet(labels.Set(map[string]string{
				kyvernov1beta1.URGeneratePolicyLabel:       engineResponse.Policy().GetName(),
				kyvernov1beta1.URGenerateResourceNameLabel: engineResponse.Resource.GetName(),
				kyvernov1beta1.URGenerateResourceKindLabel: engineResponse.Resource.GetKind(),
				kyvernov1beta1.URGenerateResourceNSLabel:   engineResponse.Resource.GetNamespace(),
			}))
			urList, err := c.urLister.List(selector)
			if err != nil {
				logger.Error(err, "failed to get update request for the resource", "kind", engineResponse.Resource.GetKind(), "name", engineResponse.Resource.GetName(), "namespace", engineResponse.Resource.GetNamespace())
				continue
			}

			for _, v := range urList {
				err := c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Delete(context.TODO(), v.GetName(), metav1.DeleteOptions{})
				if err != nil {
					logger.Error(err, "failed to delete update request")
				}
			}
		} else {
			applicableRules = append(applicableRules, r.Name())
		}
	}

	// Apply the generate rule on resource
	return c.ApplyGeneratePolicy(logger, policyContext, ur, applicableRules)
}

// getPolicySpec gets the policy spec from the ClusterPolicy/Policy
func (c *GenerateController) getPolicySpec(ur kyvernov1beta1.UpdateRequest) (kyvernov1.PolicyInterface, error) {
	pNamespace, pName, err := cache.SplitMetaNamespaceKey(ur.Spec.Policy)
	if err != nil {
		return nil, err
	}

	if pNamespace == "" {
		policyObj, err := c.policyLister.Get(pName)
		if err != nil {
			return nil, err
		}
		return policyObj, err
	}
	npolicyObj, err := c.npolicyLister.Policies(pNamespace).Get(pName)
	if err != nil {
		return nil, err
	}
	return npolicyObj, nil
}

func updateStatus(statusControl common.StatusControlInterface, ur kyvernov1beta1.UpdateRequest, err error, genResources []kyvernov1.ResourceSpec) error {
	if err != nil {
		if _, err := statusControl.Failed(ur.GetName(), err.Error(), genResources); err != nil {
			return err
		}
	} else {
		if _, err := statusControl.Success(ur.GetName(), genResources); err != nil {
			return err
		}
	}
	return nil
}

func (c *GenerateController) ApplyGeneratePolicy(log logr.Logger, policyContext *engine.PolicyContext, ur kyvernov1beta1.UpdateRequest, applicableRules []string) (genResources []kyvernov1.ResourceSpec, err error) {
	// Get the response as the actions to be performed on the resource
	// - - substitute values
	policy := policyContext.Policy()
	resource := policyContext.NewResource()
	jsonContext := policyContext.JSONContext()
	// To manage existing resources, we compare the creation time for the default resource to be generated and policy creation time
	ruleNameToProcessingTime := make(map[string]time.Duration)
	applyRules := policy.GetSpec().GetApplyRules()
	applyCount := 0

	for _, rule := range autogen.ComputeRules(policy) {
		var err error
		if !rule.HasGenerate() {
			continue
		}

		if !slices.Contains(applicableRules, rule.Name) {
			continue
		}

		startTime := time.Now()
		var genResource []kyvernov1.ResourceSpec
		if applyRules == kyvernov1.ApplyOne && applyCount > 0 {
			break
		}

		// add configmap json data to context
		if err := c.engine.ContextLoader(policyContext.Policy(), rule)(context.TODO(), rule.Context, policyContext.JSONContext()); err != nil {
			log.Error(err, "cannot add configmaps to context")
			return nil, err
		}

		if rule, err = variables.SubstituteAllInRule(log, policyContext.JSONContext(), rule); err != nil {
			log.Error(err, "variable substitution failed for rule %s", rule.Name)
			return nil, err
		}

		genResource, err = applyRule(log, c.client, rule, resource, jsonContext, policy, ur)
		if err != nil {
			log.Error(err, "failed to apply generate rule", "policy", policy.GetName(),
				"rule", rule.Name, "resource", resource.GetName(), "suggestion", "users need to grant Kyverno's service account additional privileges")
			return nil, err
		}
		ruleNameToProcessingTime[rule.Name] = time.Since(startTime)
		genResources = append(genResources, genResource...)
		applyCount++
	}

	return genResources, nil
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

func getResourceInfoForDataAndClone(rule kyvernov1.Rule) (kind, name, namespace, apiversion string, err error) {
	if len(rule.Generation.CloneList.Kinds) == 0 {
		if kind = rule.Generation.Kind; kind == "" {
			return "", "", "", "", fmt.Errorf("%s", "kind can not be empty")
		}
		if name = rule.Generation.Name; name == "" {
			return "", "", "", "", fmt.Errorf("%s", "name can not be empty")
		}
	}
	namespace = rule.Generation.Namespace
	apiversion = rule.Generation.APIVersion
	return
}

func applyRule(log logr.Logger, client dclient.Interface, rule kyvernov1.Rule, trigger unstructured.Unstructured, ctx enginecontext.EvalInterface, policy kyvernov1.PolicyInterface, ur kyvernov1beta1.UpdateRequest) ([]kyvernov1.ResourceSpec, error) {
	rdatas := []GenerateResponse{}
	var cresp, dresp map[string]interface{}
	var err error
	var mode ResourceMode
	var noGenResource kyvernov1.ResourceSpec
	var newGenResources []kyvernov1.ResourceSpec

	genKind, genName, genNamespace, genAPIVersion, err := getResourceInfoForDataAndClone(rule)
	if err != nil {
		newGenResources = append(newGenResources, noGenResource)
		return newGenResources, err
	}

	logger := log.WithValues("genKind", genKind, "genAPIVersion", genAPIVersion, "genNamespace", genNamespace, "genName", genName)

	if rule.Generation.Clone.Name != "" {
		cresp, mode, err = manageClone(logger, genAPIVersion, genKind, genNamespace, genName, policy, ur, rule, client)
		rdatas = append(rdatas, GenerateResponse{
			Data:          cresp,
			Action:        mode,
			GenName:       genName,
			GenKind:       genKind,
			GenNamespace:  genNamespace,
			GenAPIVersion: genAPIVersion,
			Error:         err,
		})
	} else if len(rule.Generation.CloneList.Kinds) != 0 {
		rdatas = manageCloneList(logger, genNamespace, ur, policy, rule, client)
	} else {
		dresp, mode, err = manageData(logger, genAPIVersion, genKind, genNamespace, genName, rule.Generation.RawData, rule.Generation.Synchronize, ur, client)
		rdatas = append(rdatas, GenerateResponse{
			Data:          dresp,
			Action:        mode,
			GenName:       genName,
			GenKind:       genKind,
			GenNamespace:  genNamespace,
			GenAPIVersion: genAPIVersion,
			Error:         err,
		})
	}

	for _, rdata := range rdatas {
		if rdata.Error != nil {
			logger.Error(err, "failed to generate resource", "mode", rdata.Action)
			newGenResources = append(newGenResources, noGenResource)
			return newGenResources, err
		}

		logger.V(3).Info("applying generate rule", "mode", rdata.Action)

		// skip processing the response in case of skip action
		if rdata.Action == Skip {
			continue
		}

		if rdata.Data == nil && rdata.Action == Update {
			logger.V(4).Info("no changes required for generate target resource")
			newGenResources = append(newGenResources, noGenResource)
			return newGenResources, nil
		}

		// build the resource template
		newResource := &unstructured.Unstructured{}
		newResource.SetUnstructuredContent(rdata.Data)
		newResource.SetName(rdata.GenName)
		newResource.SetNamespace(rdata.GenNamespace)
		if newResource.GetKind() == "" {
			newResource.SetKind(rdata.GenKind)
		}

		newResource.SetAPIVersion(rdata.GenAPIVersion)
		common.ManageLabels(newResource, trigger, policy, rule.Name)
		if rdata.Action == Create {
			newResource.SetResourceVersion("")
			_, err = client.CreateResource(context.TODO(), rdata.GenAPIVersion, rdata.GenKind, rdata.GenNamespace, newResource, false)
			if err != nil {
				if !apierrors.IsAlreadyExists(err) {
					newGenResources = append(newGenResources, noGenResource)
					return newGenResources, err
				}
			}
			logger.V(2).Info("created generate target resource")
			newGenResources = append(newGenResources, newGenResource(rdata.GenAPIVersion, rdata.GenKind, rdata.GenNamespace, rdata.GenName))
		} else if rdata.Action == Update {
			generatedObj, err := client.GetResource(context.TODO(), rdata.GenAPIVersion, rdata.GenKind, rdata.GenNamespace, rdata.GenName)
			if err != nil {
				logger.Error(err, fmt.Sprintf("generated resource not found  name:%v namespace:%v kind:%v", genName, genNamespace, genKind))
				logger.V(2).Info(fmt.Sprintf("creating generate resource name:name:%v namespace:%v kind:%v", genName, genNamespace, genKind))
				_, err = client.CreateResource(context.TODO(), rdata.GenAPIVersion, rdata.GenKind, rdata.GenNamespace, newResource, false)
				if err != nil {
					newGenResources = append(newGenResources, noGenResource)
					return newGenResources, err
				}
				newGenResources = append(newGenResources, newGenResource(rdata.GenAPIVersion, rdata.GenKind, rdata.GenNamespace, rdata.GenName))
			} else {
				// if synchronize is true - update the label and generated resource with generate policy data
				if rule.Generation.Synchronize {
					logger.V(4).Info("updating existing resource")
					if rdata.GenAPIVersion == "" {
						generatedResourceAPIVersion := generatedObj.GetAPIVersion()
						newResource.SetAPIVersion(generatedResourceAPIVersion)
					}
					if rdata.GenNamespace == "" {
						newResource.SetNamespace("default")
					}

					if _, err := ValidateResourceWithPattern(logger, generatedObj.Object, newResource.Object); err != nil {
						_, err = client.UpdateResource(context.TODO(), rdata.GenAPIVersion, rdata.GenKind, rdata.GenNamespace, newResource, false)
						if err != nil {
							logger.Error(err, "failed to update resource")
							newGenResources = append(newGenResources, noGenResource)
							return newGenResources, err
						}
					}
				}
			}
			logger.V(3).Info("updated generate target resource")
		}
	}
	return newGenResources, nil
}

func newGenResource(genAPIVersion, genKind, genNamespace, genName string) kyvernov1.ResourceSpec {
	// Resource to be generated
	newGenResource := kyvernov1.ResourceSpec{
		APIVersion: genAPIVersion,
		Kind:       genKind,
		Namespace:  genNamespace,
		Name:       genName,
	}
	return newGenResource
}

func manageData(log logr.Logger, apiVersion, kind, namespace, name string, data interface{}, synchronize bool, ur kyvernov1beta1.UpdateRequest, client dclient.Interface) (map[string]interface{}, ResourceMode, error) {
	resource, err := datautils.ToMap(data)
	if err != nil {
		return nil, Skip, err
	}

	obj, err := client.GetResource(context.TODO(), apiVersion, kind, namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) && len(ur.Status.GeneratedResources) != 0 && !synchronize {
			log.V(4).Info("synchronize is disable - skip re-create", "resource", obj)
			return nil, Skip, nil
		}
		if apierrors.IsNotFound(err) {
			return resource, Create, nil
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
	updateObj.SetUnstructuredContent(resource)
	updateObj.SetResourceVersion(obj.GetResourceVersion())
	return updateObj.UnstructuredContent(), Update, nil
}

func manageClone(log logr.Logger, apiVersion, kind, namespace, name string, policy kyvernov1.PolicyInterface, ur kyvernov1beta1.UpdateRequest, rule kyvernov1.Rule, client dclient.Interface) (map[string]interface{}, ResourceMode, error) {
	clone := rule.Generation
	// resource namespace can be nil in case of clusters scope resource
	rNamespace := clone.Clone.Namespace
	if rNamespace == "" {
		log.V(4).Info("resource namespace %s , optional in case of cluster scope resource", rNamespace)
	}

	rName := clone.Clone.Name
	if rName == "" {
		return nil, Skip, fmt.Errorf("failed to find source name")
	}

	if rNamespace == namespace && rName == name {
		log.V(4).Info("skip resource self-clone")
		return nil, Skip, nil
	}

	// check if the resource as reference in clone exists?
	obj, err := client.GetResource(context.TODO(), apiVersion, kind, rNamespace, rName)
	if err != nil {
		return nil, Skip, fmt.Errorf("source resource %s %s/%s/%s not found. %v", apiVersion, kind, rNamespace, rName, err)
	}

	if err := updateSourceLabel(client, obj, ur.Spec.Resource, policy, rule); err != nil {
		log.Error(err, "failed to add labels to the source", "kind", obj.GetKind(), "namespace", obj.GetNamespace(), "name", obj.GetName())
	}

	// check if cloned resource exists
	cobj, err := client.GetResource(context.TODO(), apiVersion, kind, namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) && len(ur.Status.GeneratedResources) != 0 && !clone.Synchronize {
			log.V(4).Info("synchronization is disabled, recreation will be skipped", "resource", cobj)
			return nil, Skip, nil
		}
	}

	// remove ownerReferences when cloning resources to other namespace
	if rNamespace != namespace && obj.GetOwnerReferences() != nil {
		obj.SetOwnerReferences(nil)
	}

	// check if resource to be generated exists
	newResource, err := client.GetResource(context.TODO(), apiVersion, kind, namespace, name)
	if err == nil {
		obj.SetUID(newResource.GetUID())
		obj.SetSelfLink(newResource.GetSelfLink())
		obj.SetCreationTimestamp(newResource.GetCreationTimestamp())
		obj.SetManagedFields(newResource.GetManagedFields())
		obj.SetResourceVersion(newResource.GetResourceVersion())
		if datautils.DeepEqual(obj, newResource) {
			return nil, Skip, nil
		}
		return obj.UnstructuredContent(), Update, nil
	}

	// create the resource based on the reference clone
	return obj.UnstructuredContent(), Create, nil
}

func manageCloneList(log logr.Logger, namespace string, ur kyvernov1beta1.UpdateRequest, policy kyvernov1.PolicyInterface, rule kyvernov1.Rule, client dclient.Interface) []GenerateResponse {
	var response []GenerateResponse
	clone := rule.Generation
	rNamespace := clone.CloneList.Namespace
	if rNamespace == "" {
		log.V(4).Info("resource namespace %s , optional in case of cluster scope resource", rNamespace)
	}

	kinds := clone.CloneList.Kinds
	if len(kinds) == 0 {
		response = append(response, GenerateResponse{
			Data:   nil,
			Action: Skip,
			Error:  fmt.Errorf("failed to find kinds list"),
		})
	}

	for _, kind := range kinds {
		apiVersion, kind := kubeutils.GetKindFromGVK(kind)
		resources, err := client.ListResource(context.TODO(), apiVersion, kind, rNamespace, clone.CloneList.Selector)
		if err != nil {
			response = append(response, GenerateResponse{
				Data:   nil,
				Action: Skip,
				Error:  fmt.Errorf("failed to list resource %s %s/%s. %v", apiVersion, kind, rNamespace, err),
			})
		}

		for _, rName := range resources.Items {
			if rNamespace == namespace {
				log.V(4).Info("skip resource self-clone")
				response = append(response, GenerateResponse{
					Data:   nil,
					Action: Skip,
					Error:  nil,
				})
			}

			// check if the resource as reference in clone exists?
			obj, err := client.GetResource(context.TODO(), apiVersion, kind, rNamespace, rName.GetName())
			if err != nil {
				log.Error(err, "failed to get resource", apiVersion, "apiVersion", kind, "kind", rNamespace, "rNamespace", rName.GetName(), "name")
				response = append(response, GenerateResponse{
					Data:   nil,
					Action: Skip,
					Error:  fmt.Errorf("source resource %s %s/%s/%s not found. %v", apiVersion, kind, rNamespace, rName.GetName(), err),
				})
				return response
			}

			if err := updateSourceLabel(client, obj, ur.Spec.Resource, policy, rule); err != nil {
				log.Error(err, "failed to add labels to the source", "kind", obj.GetKind(), "namespace", obj.GetNamespace(), "name", obj.GetName())
			}

			// check if cloned resource exists
			cobj, err := client.GetResource(context.TODO(), apiVersion, kind, namespace, rName.GetName())
			if apierrors.IsNotFound(err) && len(ur.Status.GeneratedResources) != 0 && !clone.Synchronize {
				log.V(4).Info("synchronization is disabled, recreation will be skipped", "resource", cobj)
				response = append(response, GenerateResponse{
					Data:   nil,
					Action: Skip,
					Error:  nil,
				})
			}

			// remove ownerReferences when cloning resources to other namespace
			if rNamespace != namespace && obj.GetOwnerReferences() != nil {
				obj.SetOwnerReferences(nil)
			}

			// check if resource to be generated exists
			newResource, err := client.GetResource(context.TODO(), apiVersion, kind, namespace, rName.GetName())
			if err == nil && newResource != nil {
				obj.SetUID(newResource.GetUID())
				obj.SetSelfLink(newResource.GetSelfLink())
				obj.SetCreationTimestamp(newResource.GetCreationTimestamp())
				obj.SetManagedFields(newResource.GetManagedFields())
				obj.SetResourceVersion(newResource.GetResourceVersion())

				if datautils.DeepEqual(obj, newResource) {
					response = append(response, GenerateResponse{
						Data:   nil,
						Action: Skip,
						Error:  nil,
					})
				} else {
					response = append(response, GenerateResponse{
						Data:          obj.UnstructuredContent(),
						Action:        Update,
						GenKind:       kind,
						GenName:       rName.GetName(),
						GenNamespace:  namespace,
						GenAPIVersion: apiVersion,
						Error:         nil,
					})
				}
			}
			// create the resource based on the reference clone
			response = append(response, GenerateResponse{
				Data:          obj.UnstructuredContent(),
				Action:        Create,
				GenKind:       kind,
				GenName:       rName.GetName(),
				GenNamespace:  namespace,
				GenAPIVersion: apiVersion,
				Error:         nil,
			})
		}
	}
	return response
}

type GenerateResponse struct {
	Data                                          map[string]interface{}
	Action                                        ResourceMode
	GenKind, GenName, GenNamespace, GenAPIVersion string
	Error                                         error
}

// ResourceMode defines the mode for generated resource
type ResourceMode string

const (
	// Skip : failed to process rule, will not update the resource
	Skip ResourceMode = "SKIP"
	// Create : create a new resource
	Create = "CREATE"
	// Update : update/overwrite the new resource
	Update = "UPDATE"
)

func GetUnstrRule(rule *kyvernov1.Generation) (*unstructured.Unstructured, error) {
	ruleData, err := json.Marshal(rule)
	if err != nil {
		return nil, err
	}
	return kubeutils.BytesToUnstructured(ruleData)
}

func (c *GenerateController) ApplyResource(resource *unstructured.Unstructured) error {
	kind, _, namespace, apiVersion, err := getResourceInfo(resource.Object)
	if err != nil {
		return err
	}

	_, err = c.client.CreateResource(context.TODO(), apiVersion, kind, namespace, resource, false)
	if err != nil {
		return err
	}

	return nil
}

// NewGenerateControllerWithOnlyClient returns an instance of Controller with only the client.
func NewGenerateControllerWithOnlyClient(client dclient.Interface, engine engineapi.Engine) *GenerateController {
	c := GenerateController{
		client: client,
		engine: engine,
	}
	return &c
}

// GetUnstrResource converts ResourceSpec object to type Unstructured
func (c *GenerateController) GetUnstrResource(genResourceSpec kyvernov1.ResourceSpec) (*unstructured.Unstructured, error) {
	resource, err := c.client.GetResource(context.TODO(), genResourceSpec.APIVersion, genResourceSpec.Kind, genResourceSpec.Namespace, genResourceSpec.Name)
	if err != nil {
		return nil, err
	}
	return resource, nil
}
