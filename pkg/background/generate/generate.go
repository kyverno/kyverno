package generate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
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
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/event"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	engineutils "github.com/kyverno/kyverno/pkg/utils/engine"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"golang.org/x/exp/slices"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
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
}

type forEachGenerator struct {
	rule          kyvernov1.Rule
	policyContext *engine.PolicyContext
	foreach       []kyvernov1.ForEachGeneration
	nesting       int
	client        dclient.Interface
	log           logr.Logger
	engine        engineapi.Engine
	resource      unstructured.Unstructured
	ur            kyvernov1beta1.UpdateRequest
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
	}
	return &c
}

func (c *GenerateController) ProcessUR(ur *kyvernov1beta1.UpdateRequest) error {
	logger := c.log.WithValues("name", ur.GetName(), "policy", ur.Spec.GetPolicyKey(), "resource", ur.Spec.GetResource().String())
	var err error
	var resource *unstructured.Unstructured
	var genResources []kyvernov1.ResourceSpec
	var precreatedResource bool
	logger.Info("start processing UR", "ur", ur.Name, "resourceVersion", ur.GetResourceVersion())

	// 1 - Check if the trigger exists
	resource, err = common.GetResource(c.client, ur.Spec, c.log)
	if err != nil {
		// Don't update status
		// re-queueing the UR by updating the annotation
		// retry - 5 times
		logger.V(3).Info("resource does not exist or is pending creation, re-queueing", "details", err.Error())
		retry, urAnnotations, err := increaseRetryAnnotation(ur)
		if err != nil {
			return err
		}
		if retry > 5 {
			err = c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Delete(context.TODO(), ur.GetName(), metav1.DeleteOptions{})
			if err != nil {
				logger.Error(err, "exceeds retry limit, failed to delete the UR", "update request", ur.Name, "retry", retry, "resourceVersion", ur.GetResourceVersion())
				return err
			}
		}

		ur.SetAnnotations(urAnnotations)
		_, err = c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Update(context.TODO(), ur, metav1.UpdateOptions{})
		if err != nil {
			logger.Error(err, "failed to update annotation in update request for the resource", "update request", ur.Name, "resourceVersion", ur.GetResourceVersion(), "annotations", urAnnotations, "retry", retry)
			return err
		}
	}

	// trigger resource is being terminated
	if resource == nil {
		return nil
	}

	// 2 - Apply the generate policy on the resource
	namespaceLabels := engineutils.GetNamespaceSelectorsFromNamespaceLister(resource.GetKind(), resource.GetNamespace(), c.nsLister, logger)
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

func (c *GenerateController) applyGenerate(resource unstructured.Unstructured, ur kyvernov1beta1.UpdateRequest, namespaceLabels map[string]string) ([]kyvernov1.ResourceSpec, bool, error) {
	logger := c.log.WithValues("name", ur.GetName(), "policy", ur.Spec.GetPolicyKey(), "resource", ur.Spec.GetResource().String())
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

	policyContext, precreatedResource, err := common.NewBackgroundContext(c.client, &ur, policy, &resource, c.configuration, namespaceLabels, logger)
	if err != nil {
		return nil, precreatedResource, err
	}

	// check if the policy still applies to the resource
	engineResponse := c.engine.GenerateResponse(context.Background(), policyContext, ur)
	if len(engineResponse.PolicyResponse.Rules) == 0 {
		logger.V(4).Info(doesNotApply)
		return nil, false, errors.New(doesNotApply)
	}

	var applicableRules []string
	// Removing UR if rule is failed. Used when the generate condition failed but ur exist
	for _, r := range engineResponse.PolicyResponse.Rules {
		if r.Status != engineapi.RuleStatusPass {
			logger.V(4).Info("querying all update requests")
			selector := labels.SelectorFromSet(labels.Set(map[string]string{
				kyvernov1beta1.URGeneratePolicyLabel:       engineResponse.Policy.GetName(),
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
			applicableRules = append(applicableRules, r.Name)
		}
	}

	// Apply the generate rule on resource
	return c.ApplyGeneratePolicy(logger, policyContext, ur, applicableRules)
}

// cleanupClonedResource deletes cloned resource if sync is not enabled for the clone policy
func (c *GenerateController) cleanupClonedResource(targetSpec kyvernov1.ResourceSpec) error {
	target, err := c.client.GetResource(context.TODO(), targetSpec.APIVersion, targetSpec.Kind, targetSpec.Namespace, targetSpec.Name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to find generated resource %s/%s: %v", targetSpec.Namespace, targetSpec.Name, err)
		}
	}

	if target == nil {
		return nil
	}

	labels := target.GetLabels()
	syncEnabled := labels[LabelSynchronize] == "enable"
	clone := labels[LabelClonePolicyName] != ""

	if syncEnabled && !clone {
		if err := c.client.DeleteResource(context.TODO(), target.GetAPIVersion(), target.GetKind(), target.GetNamespace(), target.GetName(), false); err != nil {
			return fmt.Errorf("cloned resource is not deleted %s/%s: %v", targetSpec.Namespace, targetSpec.Name, err)
		}
	}
	return nil
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

func updateStatus(statusControl common.StatusControlInterface, ur kyvernov1beta1.UpdateRequest, err error, genResources []kyvernov1.ResourceSpec, precreatedResource bool) error {
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

func (c *GenerateController) ApplyGeneratePolicy(log logr.Logger, policyContext *engine.PolicyContext, ur kyvernov1beta1.UpdateRequest, applicableRules []string) (genResources []kyvernov1.ResourceSpec, processExisting bool, err error) {
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
		processExisting = false
		var genResource []kyvernov1.ResourceSpec

		if len(rule.MatchResources.Kinds) > 0 {
			if len(rule.MatchResources.Annotations) == 0 && rule.MatchResources.Selector == nil {
				rcreationTime := resource.GetCreationTimestamp()
				pcreationTime := policy.GetCreationTimestamp()
				processExisting = rcreationTime.Before(&pcreationTime)
			}
		}

		if applyRules == kyvernov1.ApplyOne && applyCount > 0 {
			break
		}

		// add configmap json data to context
		if err := c.engine.ContextLoader(policyContext.Policy(), rule)(context.TODO(), rule.Context, policyContext.JSONContext()); err != nil {
			log.Error(err, "cannot add configmaps to context")
			return nil, processExisting, err
		}

		if ur.Spec.DeleteDownstream {
			pKey := common.PolicyKey(policy.GetNamespace(), policy.GetName())
			err = c.deleteResource(pKey, rule, ur)
			return nil, false, err
		}

		if policy.GetSpec().IsGenerateExistingOnPolicyUpdate() || !processExisting {
			if rule.Generation.ForEachGeneration != nil {
				g := &forEachGenerator{
					rule:          rule,
					foreach:       rule.Generation.ForEachGeneration,
					policyContext: policyContext,
					log:           log,
					engine:        c.engine,
					client:        c.client,
					resource:      resource,
					ur:            ur,
					nesting:       0,
				}
				genResource, err = g.generateForEach(context.TODO())
			} else {
				if rule, err = variables.SubstituteAllInRule(log, policyContext.JSONContext(), rule); err != nil {
					log.Error(err, "variable substitution failed for rule %s", rule.Name)
					return nil, processExisting, err
				}
				genResource, err = applyRule(log, c.client, rule, resource, jsonContext, policy, ur)
			}
			if err != nil {
				log.Error(err, "failed to apply generate rule", "policy", policy.GetName(),
					"rule", rule.Name, "resource", resource.GetName(), "suggestion", "users need to grant Kyverno's service account additional privileges")
				return nil, processExisting, err
			}
			ruleNameToProcessingTime[rule.Name] = time.Since(startTime)
			genResources = append(genResources, genResource...)
		}

		if policy.GetSpec().IsGenerateExistingOnPolicyUpdate() {
			processExisting = false
		}

		applyCount++
	}

	return genResources, processExisting, nil
}

func (f *forEachGenerator) generateForEach(ctx context.Context) ([]kyvernov1.ResourceSpec, error) {
	var applyCount int
	log := f.log
	var newGenResources []kyvernov1.ResourceSpec

	preconditionsPassed, err := engine.InternalCheckPrecondition(f.log, f.policyContext, f.rule.GetAnyAllConditions())
	if err != nil {
		return newGenResources, err
	}

	if !preconditionsPassed {
		log.Info("preconditions not met")
		return newGenResources, nil
	}

	for _, fe := range f.foreach {
		var elements []interface{}
		elements, err = engine.EvaluateList(fe.List, f.policyContext.JSONContext())
		if err != nil {
			err = fmt.Errorf("%v failed to evaluate list %s", err, fe.List)
			return newGenResources, err
		}
		newGenResources, err = f.generateElements(ctx, fe, elements)
		if err != nil {
			return newGenResources, err
		}
		applyCount++
	}
	msg := fmt.Sprintf("%d elements processed", applyCount)
	log.Info(msg)
	return newGenResources, nil
}

func (f *forEachGenerator) generateElements(ctx context.Context, foreach kyvernov1.ForEachGeneration, elements []interface{}) ([]kyvernov1.ResourceSpec, error) {
	f.policyContext.JSONContext().Checkpoint()
	defer f.policyContext.JSONContext().Restore()
	var newGenResources []kyvernov1.ResourceSpec

	for i, element := range elements {
		if element == nil {
			continue
		}

		// TODO - this needs to be refactored. The engine should not have a dependency to the CLI code
		store.SetForEachElement(i)

		f.policyContext.JSONContext().Reset()
		falseVar := false
		policyContext := f.policyContext.Copy()
		if err := engine.AddElementToContext(policyContext, element, i, f.nesting, &falseVar); err != nil {
			err = fmt.Errorf("%v failed to add element to context", err)
			return newGenResources, err
		}

		for _, subResource := range foreach.GenerateSubResources {
			if err := f.engine.ContextLoader(policyContext.Policy(), f.rule)(ctx, subResource.Context, f.policyContext.JSONContext()); err != nil {
				return newGenResources, err
			}

			preconditionsPassed, err := engine.InternalCheckPrecondition(f.log, policyContext, subResource.AnyAllConditions)
			if err != nil {
				return newGenResources, err
			}

			if !preconditionsPassed {
				f.log.Info("generate.foreach.[preconditions] not met", "elementIndex", i, "for subResource", subResource)
				continue
			}
			tempNewGenResources, err := f.forEach(subResource)
			if err != nil {
				f.log.Error(err, "could not apply generate with", "element", element)
				return newGenResources, err
			}
			newGenResources = append(newGenResources, tempNewGenResources...)
		}
	}
	return newGenResources, nil
}

func subResourceGetResourceInfoForDataAndClone(se kyvernov1.GenerateSubResource) (kind, name, namespace, apiversion string, err error) {
	if kind = se.Kind; kind == "" {
		return "", "", "", "", fmt.Errorf("%s", "kind can not be empty")
	}
	if name = se.Name; name == "" {
		return "", "", "", "", fmt.Errorf("%s", "name can not be empty")
	}
	namespace = se.Namespace
	apiversion = se.APIVersion
	return
}

func (f *forEachGenerator) forEach(subResource kyvernov1.GenerateSubResource) ([]kyvernov1.ResourceSpec, error) {
	log, client, rule, resource, ctx, policy, ur := f.log, f.client, f.rule, f.resource, f.policyContext.JSONContext(), f.policyContext.Policy(), f.ur
	var noGenResource kyvernov1.ResourceSpec
	var newGenResources []kyvernov1.ResourceSpec
	rdatas := []GenerateResponse{}
	var cresp, dresp map[string]interface{}
	var mode ResourceMode

	tempSubResource, err := substituteAllInSubResource(subResource, ctx, log)
	subResource = *tempSubResource
	if err != nil {
		return newGenResources, nil
	}

	genKind, genName, genNamespace, genAPIVersion, err := subResourceGetResourceInfoForDataAndClone(subResource)
	logger := log.WithValues("genKind", genKind, "genAPIVersion", genAPIVersion, "genNamespace", genNamespace, "genName", genName)
	if err != nil {
		logger.Error(err, "failed to generate resource")
		return newGenResources, err
	}
	if subResource.Clone.Name != "" {
		cresp, mode, err = manageClone(logger, genAPIVersion, genKind, genNamespace, genName, policy, ur, rule, subResource, client)
		rdatas = append(rdatas, GenerateResponse{
			Data:          cresp,
			Action:        mode,
			GenName:       genName,
			GenKind:       genKind,
			GenNamespace:  genNamespace,
			GenAPIVersion: genAPIVersion,
			Error:         err,
		})
	} else {
		dresp, mode, err = manageData(logger, genAPIVersion, genKind, genNamespace, genName, subResource.RawData, subResource.Synchronize, ur, client)
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
			break
		}

		logger.V(3).Info("applying generate rule", "mode", rdata.Action)

		// skip processing the response in case of skip action
		if rdata.Action == Skip {
			continue
		}

		if rdata.Data == nil && rdata.Action == Update {
			logger.V(4).Info("no changes required for generate target resource")
			newGenResources = append(newGenResources, noGenResource)
			break
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
		// manage labels
		// - app.kubernetes.io/managed-by: kyverno
		// "kyverno.io/generated-by-kind": kind (trigger resource)
		// "kyverno.io/generated-by-namespace": namespace (trigger resource)
		// "kyverno.io/generated-by-name": name (trigger resource)
		common.ManageLabels(newResource, resource, policy, rule.Name)
		// Add Synchronize label
		label := newResource.GetLabels()

		// Add background gen-rule label if generate rule applied on existing resource
		if policy.GetSpec().IsGenerateExistingOnPolicyUpdate() {
			label["kyverno.io/background-gen-rule"] = rule.Name
		}

		label["policy.kyverno.io/policy-name"] = policy.GetName()
		label["policy.kyverno.io/gr-name"] = ur.Name
		if rdata.Action == Create {
			if subResource.Synchronize {
				label["policy.kyverno.io/synchronize"] = "enable"
			} else {
				label["policy.kyverno.io/synchronize"] = "disable"
			}

			// Reset resource version
			newResource.SetResourceVersion("")
			newResource.SetLabels(label)

			// Create the resource
			_, err = client.CreateResource(context.TODO(), rdata.GenAPIVersion, rdata.GenKind, rdata.GenNamespace, newResource, false)
			if err != nil {
				if !apierrors.IsAlreadyExists(err) {
					logger.Error(err, "failed to create resource")
					newGenResources = append(newGenResources, noGenResource)
					break
				}
			}
			logger.V(2).Info("created generate target resource")
			newGenResources = append(newGenResources, newGenResource(rdata.GenAPIVersion, rdata.GenKind, rdata.GenNamespace, rdata.GenName))
		} else if rdata.Action == Update {
			var generatedObj *unstructured.Unstructured
			generatedObj, err = client.GetResource(context.TODO(), rdata.GenAPIVersion, rdata.GenKind, rdata.GenNamespace, rdata.GenName)
			if err != nil {
				logger.Error(err, fmt.Sprintf("generated resource not found  name:%v namespace:%v kind:%v", genName, genNamespace, genKind))
				logger.V(2).Info(fmt.Sprintf("creating generate resource name:name:%v namespace:%v kind:%v", genName, genNamespace, genKind))
				_, err = client.CreateResource(context.TODO(), rdata.GenAPIVersion, rdata.GenKind, rdata.GenNamespace, newResource, false)
				if err != nil {
					logger.Error(err, "failed to update resource")
					newGenResources = append(newGenResources, noGenResource)
					break
				}
				newGenResources = append(newGenResources, newGenResource(rdata.GenAPIVersion, rdata.GenKind, rdata.GenNamespace, rdata.GenName))
			} else {
				// if synchronize is true - update the label and generated resource with generate policy data
				if subResource.Synchronize {
					logger.V(4).Info("updating existing resource")
					label["policy.kyverno.io/synchronize"] = "enable"
					newResource.SetLabels(label)

					if rdata.GenAPIVersion == "" {
						generatedResourceAPIVersion := generatedObj.GetAPIVersion()
						newResource.SetAPIVersion(generatedResourceAPIVersion)
					}
					if rdata.GenNamespace == "" {
						newResource.SetNamespace("default")
					}

					if _, err = ValidateResourceWithPattern(logger, generatedObj.Object, newResource.Object); err != nil {
						_, err = client.UpdateResource(context.TODO(), rdata.GenAPIVersion, rdata.GenKind, rdata.GenNamespace, newResource, false)
						if err != nil {
							logger.Error(err, "failed to update resource")
							newGenResources = append(newGenResources, noGenResource)
							break
						}
					}
				} else {
					currentGeneratedResourcelabel := generatedObj.GetLabels()
					currentSynclabel := currentGeneratedResourcelabel["policy.kyverno.io/synchronize"]

					// update only if the labels mismatches
					if (!subResource.Synchronize && currentSynclabel == "enable") ||
						(subResource.Synchronize && currentSynclabel == "disable") {
						logger.V(4).Info("updating label in existing resource")
						currentGeneratedResourcelabel["policy.kyverno.io/synchronize"] = "disable"
						generatedObj.SetLabels(currentGeneratedResourcelabel)

						_, err = client.UpdateResource(context.TODO(), rdata.GenAPIVersion, rdata.GenKind, rdata.GenNamespace, generatedObj, false)
						if err != nil {
							logger.Error(err, "failed to update label in existing resource")
							newGenResources = append(newGenResources, noGenResource)
							break
						}
					}
				}
			}
			logger.V(3).Info("updated generate target resource")
		}
	}
	return newGenResources, nil
}

func substituteAllInSubResource(sr kyvernov1.GenerateSubResource, ctx enginecontext.EvalInterface, logger logr.Logger) (*kyvernov1.GenerateSubResource, error) {
	jsonObj, err := datautils.ToMap(sr)
	if err != nil {
		return nil, err
	}

	var data interface{}
	data, err = variables.SubstituteAll(logger, ctx, jsonObj)
	if err != nil {
		return nil, err
	}

	var bytes []byte
	bytes, err = json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var updatedListSubResource kyvernov1.GenerateSubResource
	if err = json.Unmarshal(bytes, &updatedListSubResource); err != nil {
		return nil, err
	}

	return &updatedListSubResource, nil
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

func applyRule(log logr.Logger, client dclient.Interface, rule kyvernov1.Rule, resource unstructured.Unstructured, ctx enginecontext.EvalInterface, policy kyvernov1.PolicyInterface, ur kyvernov1beta1.UpdateRequest) ([]kyvernov1.ResourceSpec, error) {
	rdatas := []GenerateResponse{}
	var cresp, dresp map[string]interface{}
	var err error
	var mode ResourceMode
	var noGenResource kyvernov1.ResourceSpec
	var noSubResource kyvernov1.GenerateSubResource
	var newGenResources []kyvernov1.ResourceSpec

	genKind, genName, genNamespace, genAPIVersion, err := getResourceInfoForDataAndClone(rule)
	if err != nil {
		newGenResources = append(newGenResources, noGenResource)
		return newGenResources, err
	}

	logger := log.WithValues("genKind", genKind, "genAPIVersion", genAPIVersion, "genNamespace", genNamespace, "genName", genName)

	if rule.Generation.Clone.Name != "" {
		cresp, mode, err = manageClone(logger, genAPIVersion, genKind, genNamespace, genName, policy, ur, rule, noSubResource, client)
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
		common.ManageLabels(newResource, resource, policy, rule.Name)
		// Add Synchronize label
		label := newResource.GetLabels()

		// Add background gen-rule label if generate rule applied on existing resource
		if policy.GetSpec().IsGenerateExistingOnPolicyUpdate() {
			label[LabelBackgroundGenRuleName] = rule.Name
		}

		label[LabelDataPolicyName] = policy.GetName()
		label[LabelURName] = ur.Name
		if rdata.Action == Create {
			if rule.Generation.Synchronize {
				label[LabelSynchronize] = "enable"
			} else {
				label[LabelSynchronize] = "disable"
			}

			// Reset resource version
			newResource.SetResourceVersion("")
			newResource.SetLabels(label)

			// Create the resource
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
					label[LabelSynchronize] = "enable"
					newResource.SetLabels(label)

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
				} else {
					currentGeneratedResourcelabel := generatedObj.GetLabels()
					currentSynclabel := currentGeneratedResourcelabel[LabelSynchronize]

					// update only if the labels mismatches
					if (!rule.Generation.Synchronize && currentSynclabel == "enable") ||
						(rule.Generation.Synchronize && currentSynclabel == "disable") {
						logger.V(4).Info("updating label in existing resource")
						currentGeneratedResourcelabel[LabelSynchronize] = "disable"
						generatedObj.SetLabels(currentGeneratedResourcelabel)

						_, err = client.UpdateResource(context.TODO(), rdata.GenAPIVersion, rdata.GenKind, rdata.GenNamespace, generatedObj, false)
						if err != nil {
							logger.Error(err, "failed to update label in existing resource")
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

func manageClone(log logr.Logger, apiVersion, kind, namespace, name string, policy kyvernov1.PolicyInterface, ur kyvernov1beta1.UpdateRequest, rule kyvernov1.Rule, sr kyvernov1.GenerateSubResource, client dclient.Interface) (map[string]interface{}, ResourceMode, error) {
	var rNamespace string
	var rName string
	clone := rule.Generation

	// resource namespace can be nil in case of clusters scope resource
	if len(clone.ForEachGeneration) > 0 {
		rNamespace = sr.Clone.Namespace
		rName = sr.Clone.Name
	} else {
		rNamespace = clone.Clone.Namespace
		rName = clone.Clone.Name
	}

	if rNamespace == "" {
		log.V(4).Info("resource namespace %s , optional in case of cluster scope resource", rNamespace)
	}

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
		if reflect.DeepEqual(obj, newResource) {
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

				if reflect.DeepEqual(obj, newResource) {
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

func (c *GenerateController) deleteResource(policyKey string, rule kyvernov1.Rule, ur kyvernov1beta1.UpdateRequest) error {
	if policyKey != ur.Spec.Policy {
		return nil
	}

	if rule.Name == ur.Spec.Rule {
		return c.client.DeleteResource(context.TODO(), rule.Generation.GetAPIVersion(), rule.Generation.GetKind(), rule.Generation.GetNamespace(), rule.Generation.GetName(), false)
	}

	return nil
}
