package generate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
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
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/context/resolvers"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/registryclient"
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
	rclient       registryclient.Client

	// listers
	urLister      kyvernov1beta1listers.UpdateRequestNamespaceLister
	nsLister      corev1listers.NamespaceLister
	policyLister  kyvernov1listers.ClusterPolicyLister
	npolicyLister kyvernov1listers.PolicyLister

	configuration          config.Configuration
	informerCacheResolvers resolvers.ConfigmapResolver
	eventGen               event.Interface

	log logr.Logger
}

// NewGenerateController returns an instance of the Generate-Request Controller
func NewGenerateController(
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	statusControl common.StatusControlInterface,
	rclient registryclient.Client,
	policyLister kyvernov1listers.ClusterPolicyLister,
	npolicyLister kyvernov1listers.PolicyLister,
	urLister kyvernov1beta1listers.UpdateRequestNamespaceLister,
	nsLister corev1listers.NamespaceLister,
	dynamicConfig config.Configuration,
	informerCacheResolvers resolvers.ConfigmapResolver,
	eventGen event.Interface,
	log logr.Logger,
) *GenerateController {
	c := GenerateController{
		client:                 client,
		kyvernoClient:          kyvernoClient,
		statusControl:          statusControl,
		rclient:                rclient,
		policyLister:           policyLister,
		npolicyLister:          npolicyLister,
		urLister:               urLister,
		nsLister:               nsLister,
		configuration:          dynamicConfig,
		informerCacheResolvers: informerCacheResolvers,
		eventGen:               eventGen,
		log:                    log,
	}
	return &c
}

func (c *GenerateController) ProcessUR(ur *kyvernov1beta1.UpdateRequest) error {
	logger := c.log.WithValues("name", ur.Name, "policy", ur.Spec.Policy, "kind", ur.Spec.Resource.Kind, "apiVersion", ur.Spec.Resource.APIVersion, "namespace", ur.Spec.Resource.Namespace, "name", ur.Spec.Resource.Name)
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
					return c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Delete(context.TODO(), ur.Name, metav1.DeleteOptions{})
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
		_, err := c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Update(context.TODO(), ur, metav1.UpdateOptions{})
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

	policyContext, precreatedResource, err := common.NewBackgroundContext(c.client, &ur, &policy, &resource, c.configuration, c.informerCacheResolvers, namespaceLabels, logger)
	if err != nil {
		return nil, precreatedResource, err
	}

	// check if the policy still applies to the resource
	engineResponse := engine.GenerateResponse(c.rclient, policyContext, ur)
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
	syncEnabled := labels["policy.kyverno.io/synchronize"] == "enable"
	clone := labels["generate.kyverno.io/clone-policy-name"] != ""

	if syncEnabled && !clone {
		if err := c.client.DeleteResource(context.TODO(), target.GetAPIVersion(), target.GetKind(), target.GetNamespace(), target.GetName(), false); err != nil {
			return fmt.Errorf("cloned resource is not deleted %s/%s: %v", targetSpec.Namespace, targetSpec.Name, err)
		}
	}
	return nil
}

// getPolicySpec gets the policy spec from the ClusterPolicy/Policy
func (c *GenerateController) getPolicySpec(ur kyvernov1beta1.UpdateRequest) (kyvernov1.ClusterPolicy, error) {
	var policy kyvernov1.ClusterPolicy

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
	}
	npolicyObj, err := c.npolicyLister.Policies(pNamespace).Get(pName)
	if err != nil {
		return policy, err
	}
	return kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: pName,
		},
		Spec: npolicyObj.Spec,
	}, nil
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
		if err := engine.LoadContext(context.TODO(), log, c.rclient, rule.Context, policyContext, rule.Name); err != nil {
			log.Error(err, "cannot add configmaps to context")
			return nil, processExisting, err
		}

		if rule, err = variables.SubstituteAllInRule(log, policyContext.JSONContext(), rule); err != nil {
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
			genResources = append(genResources, genResource...)
		}

		if policy.GetSpec().IsGenerateExistingOnPolicyUpdate() {
			processExisting = false
		}

		applyCount++
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
	var newGenResources []kyvernov1.ResourceSpec

	genKind, genName, genNamespace, genAPIVersion, err := getResourceInfoForDataAndClone(rule)
	if err != nil {
		newGenResources = append(newGenResources, noGenResource)
		return newGenResources, err
	}

	logger := log.WithValues("genKind", genKind, "genAPIVersion", genAPIVersion, "genNamespace", genNamespace, "genName", genName)

	if rule.Generation.Clone.Name != "" {
		cresp, mode, err = manageClone(logger, genAPIVersion, genKind, genNamespace, genName, policy.GetName(), ur, rule.Generation, client)
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
		rdatas = manageCloneList(logger, genNamespace, policy.GetName(), ur, rule.Generation, client)
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
		if rdata.Action == Create {
			if rule.Generation.Synchronize {
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
					label["policy.kyverno.io/synchronize"] = "enable"
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
					currentSynclabel := currentGeneratedResourcelabel["policy.kyverno.io/synchronize"]

					// update only if the labels mismatches
					if (!rule.Generation.Synchronize && currentSynclabel == "enable") ||
						(rule.Generation.Synchronize && currentSynclabel == "disable") {
						logger.V(4).Info("updating label in existing resource")
						currentGeneratedResourcelabel["policy.kyverno.io/synchronize"] = "disable"
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

func manageClone(log logr.Logger, apiVersion, kind, namespace, name, policy string, ur kyvernov1beta1.UpdateRequest, clone kyvernov1.Generation, client dclient.Interface) (map[string]interface{}, ResourceMode, error) {
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

func manageCloneList(log logr.Logger, namespace, policy string, ur kyvernov1beta1.UpdateRequest, clone kyvernov1.Generation, client dclient.Interface) []GenerateResponse {
	var response []GenerateResponse

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
				log.Error(err, "failed to get resoruce", apiVersion, "apiVersion", kind, "kind", rNamespace, "rNamespace", rName.GetName(), "name")
				response = append(response, GenerateResponse{
					Data:   nil,
					Action: Skip,
					Error:  fmt.Errorf("source resource %s %s/%s/%s not found. %v", apiVersion, kind, rNamespace, rName.GetName(), err),
				})
				return response
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
func NewGenerateControllerWithOnlyClient(client dclient.Interface) *GenerateController {
	c := GenerateController{
		client: client,
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

func deleteGeneratedResources(log logr.Logger, client dclient.Interface, ur kyvernov1beta1.UpdateRequest) error {
	for _, genResource := range ur.Status.GeneratedResources {
		err := client.DeleteResource(context.TODO(), genResource.APIVersion, genResource.Kind, genResource.Namespace, genResource.Name, false)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}

		log.V(3).Info("generated resource deleted", "genKind", ur.Spec.Resource.Kind, "genNamespace", ur.Spec.Resource.Namespace, "genName", ur.Spec.Resource.Name)
	}
	return nil
}
