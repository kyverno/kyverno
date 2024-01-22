package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
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
	"github.com/kyverno/kyverno/pkg/engine/validate"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/event"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	engineutils "github.com/kyverno/kyverno/pkg/utils/engine"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/pkg/errors"
	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
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
		if err := updateStatus(c.statusControl, *ur, err, nil); err != nil {
			return err
		}
		return nil
	}

	if trigger == nil {
		return nil
	}

	namespaceLabels := engineutils.GetNamespaceSelectorsFromNamespaceLister(trigger.GetKind(), trigger.GetNamespace(), c.nsLister, logger)
	genResources, err = c.applyGenerate(*trigger, *ur, namespaceLabels)
	if err != nil {
		if strings.Contains(err.Error(), doesNotApply) {
			ur.Status.State = kyvernov1beta1.Completed
			logger.V(4).Info(fmt.Sprintf("%s, updating UR status to Completed", err.Error()))
			_, err := c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), ur, metav1.UpdateOptions{})
			return err
		}

		policy, err := c.getPolicySpec(*ur)
		if err != nil {
			return err
		}

		events := event.NewBackgroundFailedEvent(err, policy, ur.Spec.Rule, event.GeneratePolicyController,
			kyvernov1.ResourceSpec{Kind: trigger.GetKind(), Namespace: trigger.GetNamespace(), Name: trigger.GetName()})
		c.eventGen.Add(events...)
	}

	if err = updateStatus(c.statusControl, *ur, err, genResources); err != nil {
		return err
	}
	return err
}

const doesNotApply = "policy does not apply to resource"

func (c *GenerateController) getTrigger(spec kyvernov1beta1.UpdateRequestSpec) (*unstructured.Unstructured, error) {
	admissionRequest := spec.Context.AdmissionRequestInfo.AdmissionRequest
	if admissionRequest == nil {
		return common.GetResource(c.client, spec, c.log)
	} else {
		operation := spec.Context.AdmissionRequestInfo.Operation
		if operation == admissionv1.Delete {
			return c.getTriggerForDeleteOperation(spec)
		} else if operation == admissionv1.Create {
			return c.getTriggerForCreateOperation(spec)
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

func (c *GenerateController) getTriggerForDeleteOperation(spec kyvernov1beta1.UpdateRequestSpec) (*unstructured.Unstructured, error) {
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

func (c *GenerateController) getTriggerForCreateOperation(spec kyvernov1beta1.UpdateRequestSpec) (*unstructured.Unstructured, error) {
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
				kyvernov1beta1.URGenerateResourceKindLabel: engineResponse.Resource.GetKind(),
				kyvernov1beta1.URGenerateResourceNSLabel:   engineResponse.Resource.GetNamespace(),
			}))
			// get update requests that have the resource UID label
			requirement, err := labels.NewRequirement(kyvernov1beta1.URGenerateResourceUIDLabel, selection.Equals, []string{string(engineResponse.Resource.GetUID())})
			if err != nil {
				logger.Error(err, "failed to add the resource UID label")
			}
			selectorWithResUID := selector.Add(*requirement)
			urList, err := c.urLister.List(selectorWithResUID)
			if err != nil {
				logger.Error(err, "failed to get update request for the resource", "kind", engineResponse.Resource.GetKind(), "name", engineResponse.Resource.GetName(), "namespace", engineResponse.Resource.GetNamespace())
				continue
			}

			if len(urList) == 0 {
				// get update requests that have the resource name label
				requirement, err = labels.NewRequirement(kyvernov1beta1.URGenerateResourceNameLabel, selection.Equals, []string{engineResponse.Resource.GetName()})
				if err != nil {
					logger.Error(err, "failed to add the resource name label")
					continue
				}
				selectorWithResName := selector.Add(*requirement)
				urList, err = c.urLister.List(selectorWithResName)
				if err != nil {
					logger.Error(err, "failed to get update request for the resource", "kind", engineResponse.Resource.GetKind(), "name", engineResponse.Resource.GetName(), "namespace", engineResponse.Resource.GetNamespace())
					continue
				}
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
	genResources, err := c.ApplyGeneratePolicy(logger, policyContext, ur, applicableRules)

	// generate events.
	if err == nil {
		for _, res := range genResources {
			e := event.NewResourceGenerationEvent(ur.Spec.Policy, ur.Spec.Rule, event.GeneratePolicyController, res)
			c.eventGen.Add(e)
		}

		e := event.NewBackgroundSuccessEvent(event.GeneratePolicyController, policy, genResources)
		c.eventGen.Add(e...)
	}

	return genResources, err
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
			log.Error(err, "failed to apply generate rule", "policy", policy.GetName(), "rule", rule.Name, "resource", resource.GetName())
			return nil, err
		}
		ruleNameToProcessingTime[rule.Name] = time.Since(startTime)
		genResources = append(genResources, genResource...)
		applyCount++
	}

	return genResources, nil
}

func applyRule(log logr.Logger, client dclient.Interface, rule kyvernov1.Rule, trigger unstructured.Unstructured, ctx enginecontext.EvalInterface, policy kyvernov1.PolicyInterface, ur kyvernov1beta1.UpdateRequest) ([]kyvernov1.ResourceSpec, error) {
	responses := []generateResponse{}
	var err error
	var newGenResources []kyvernov1.ResourceSpec

	target := rule.Generation.ResourceSpec
	logger := log.WithValues("target", target.String())

	if rule.Generation.Clone.Name != "" {
		resp := manageClone(logger.WithValues("type", "clone"), target, kyvernov1.ResourceSpec{}, policy, ur, rule, client)
		responses = append(responses, resp)
	} else if len(rule.Generation.CloneList.Kinds) != 0 {
		responses = manageCloneList(logger.WithValues("type", "cloneList"), target.GetNamespace(), ur, policy, rule, client)
	} else {
		resp := manageData(logger.WithValues("type", "data"), target, rule.Generation.RawData, rule.Generation.Synchronize, ur, client)
		responses = append(responses, resp)
	}

	for _, response := range responses {
		targetMeta := response.GetTarget()
		if response.GetError() != nil {
			logger.Error(response.GetError(), "failed to generate resource", "mode", response.GetAction())
			return newGenResources, err
		}

		if response.GetAction() == Skip {
			continue
		}

		logger.V(3).Info("applying generate rule", "mode", response.GetAction())
		if response.GetData() == nil && response.GetAction() == Update {
			logger.V(4).Info("no changes required for generate target resource")
			return newGenResources, nil
		}

		newResource := &unstructured.Unstructured{}
		newResource.SetUnstructuredContent(response.GetData())
		newResource.SetName(targetMeta.GetName())
		newResource.SetNamespace(targetMeta.GetNamespace())
		if newResource.GetKind() == "" {
			newResource.SetKind(targetMeta.GetKind())
		}

		newResource.SetAPIVersion(targetMeta.GetAPIVersion())
		common.ManageLabels(newResource, trigger, policy, rule.Name)
		if response.GetAction() == Create {
			newResource.SetResourceVersion("")
			if policy.GetSpec().UseServerSideApply {
				_, err = client.ApplyResource(context.TODO(), targetMeta.GetAPIVersion(), targetMeta.GetKind(), targetMeta.GetNamespace(), targetMeta.GetName(), newResource, false, "generate")
			} else {
				_, err = client.CreateResource(context.TODO(), targetMeta.GetAPIVersion(), targetMeta.GetKind(), targetMeta.GetNamespace(), newResource, false)
			}
			if err != nil {
				if !apierrors.IsAlreadyExists(err) {
					return newGenResources, err
				}
			}
			logger.V(2).Info("created generate target resource")
			newGenResources = append(newGenResources, targetMeta)
		} else if response.GetAction() == Update {
			generatedObj, err := client.GetResource(context.TODO(), targetMeta.GetAPIVersion(), targetMeta.GetKind(), targetMeta.GetNamespace(), targetMeta.GetName())
			if err != nil {
				logger.V(2).Info("target resource not found, creating new target")
				if policy.GetSpec().UseServerSideApply {
					_, err = client.ApplyResource(context.TODO(), targetMeta.GetAPIVersion(), targetMeta.GetKind(), targetMeta.GetNamespace(), targetMeta.GetName(), newResource, false, "generate")
				} else {
					_, err = client.CreateResource(context.TODO(), targetMeta.GetAPIVersion(), targetMeta.GetKind(), targetMeta.GetNamespace(), newResource, false)
				}
				if err != nil {
					return newGenResources, err
				}
				newGenResources = append(newGenResources, targetMeta)
			} else {
				if !rule.Generation.Synchronize {
					logger.V(4).Info("synchronize disabled, skip syncing changes")
					continue
				}
				if err := validate.MatchPattern(logger, generatedObj.Object, newResource.Object); err == nil {
					logger.V(4).Info("patterns match, skipping updates")
					continue
				}
				logger.V(4).Info("updating existing resource")
				if targetMeta.GetAPIVersion() == "" {
					generatedResourceAPIVersion := generatedObj.GetAPIVersion()
					newResource.SetAPIVersion(generatedResourceAPIVersion)
				}
				if targetMeta.GetNamespace() == "" {
					newResource.SetNamespace("default")
				}

				if policy.GetSpec().UseServerSideApply {
					_, err = client.ApplyResource(context.TODO(), targetMeta.GetAPIVersion(), targetMeta.GetKind(), targetMeta.GetNamespace(), targetMeta.GetName(), newResource, false, "generate")
				} else {
					_, err = client.UpdateResource(context.TODO(), targetMeta.GetAPIVersion(), targetMeta.GetKind(), targetMeta.GetNamespace(), newResource, false)
				}
				if err != nil {
					logger.Error(err, "failed to update resource")
					return newGenResources, err
				}
			}
			logger.V(3).Info("updated generate target resource")
		}
	}
	return newGenResources, nil
}

func GetUnstrRule(rule *kyvernov1.Generation) (*unstructured.Unstructured, error) {
	ruleData, err := json.Marshal(rule)
	if err != nil {
		return nil, err
	}
	return kubeutils.BytesToUnstructured(ruleData)
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
