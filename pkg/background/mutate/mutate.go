package mutate

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/background/common"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/utils"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	engineutils "github.com/kyverno/kyverno/pkg/utils/engine"
	"go.uber.org/multierr"
	yamlv2 "gopkg.in/yaml.v2"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

var ErrEmptyPatch error = fmt.Errorf("empty resource to patch")

type mutateExistingController struct {
	// clients
	client        dclient.Interface
	statusControl common.StatusControlInterface
	engine        engineapi.Engine

	// listers
	policyLister  kyvernov1listers.ClusterPolicyLister
	npolicyLister kyvernov1listers.PolicyLister
	nsLister      corev1listers.NamespaceLister

	configuration config.Configuration
	eventGen      event.Interface

	log logr.Logger
	jp  jmespath.Interface
}

// NewMutateExistingController returns an instance of the MutateExistingController
func NewMutateExistingController(
	client dclient.Interface,
	statusControl common.StatusControlInterface,
	engine engineapi.Engine,
	policyLister kyvernov1listers.ClusterPolicyLister,
	npolicyLister kyvernov1listers.PolicyLister,
	nsLister corev1listers.NamespaceLister,
	dynamicConfig config.Configuration,
	eventGen event.Interface,
	log logr.Logger,
	jp jmespath.Interface,
) *mutateExistingController {
	c := mutateExistingController{
		client:        client,
		statusControl: statusControl,
		engine:        engine,
		policyLister:  policyLister,
		npolicyLister: npolicyLister,
		nsLister:      nsLister,
		configuration: dynamicConfig,
		eventGen:      eventGen,
		log:           log,
		jp:            jp,
	}
	return &c
}

func (c *mutateExistingController) ProcessUR(ur *kyvernov1beta1.UpdateRequest) error {
	logger := c.log.WithValues("name", ur.GetName(), "policy", ur.Spec.GetPolicyKey(), "resource", ur.Spec.GetResource().String())
	var errs []error

	policy, err := c.getPolicy(ur)
	if err != nil {
		logger.Error(err, "failed to get policy")
		return err
	}

	for _, rule := range policy.GetSpec().Rules {
		if !rule.IsMutateExisting() || ur.Spec.Rule != rule.Name {
			continue
		}

		var trigger *unstructured.Unstructured
		admissionRequest := ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest
		if admissionRequest == nil {
			trigger, err = common.GetResource(c.client, ur.Spec, c.log)
			if err != nil || trigger == nil {
				logger.WithName(rule.Name).Error(err, "failed to get trigger resource")
				errs = append(errs, err)
				continue
			}
		} else {
			if admissionRequest.Operation == admissionv1.Create {
				trigger, err = common.GetResource(c.client, ur.Spec, c.log)
				if err != nil || trigger == nil {
					if admissionRequest.SubResource == "" {
						logger.WithName(rule.Name).Error(err, "failed to get trigger resource")
						errs = append(errs, err)
						continue
					} else {
						logger.WithName(rule.Name).Info("trigger resource not found for subresource, reverting to resource in AdmissionReviewRequest", "subresource", admissionRequest.SubResource)
						newResource, _, err := admissionutils.ExtractResources(nil, *admissionRequest)
						if err != nil {
							logger.WithName(rule.Name).Error(err, "failed to extract resources from admission review request")
							errs = append(errs, err)
							continue
						}
						trigger = &newResource
					}
				}
			} else {
				newResource, oldResource, err := admissionutils.ExtractResources(nil, *admissionRequest)
				if err != nil {
					logger.WithName(rule.Name).Error(err, "failed to extract resources from admission review request")
					errs = append(errs, err)
					continue
				}

				trigger = &newResource
				if newResource.Object == nil {
					trigger = &oldResource
				}
			}
		}

		namespaceLabels := engineutils.GetNamespaceSelectorsFromNamespaceLister(trigger.GetKind(), trigger.GetNamespace(), c.nsLister, logger)
		policyContext, err := common.NewBackgroundContext(logger, c.client, ur, policy, trigger, c.configuration, c.jp, namespaceLabels)
		if err != nil {
			logger.WithName(rule.Name).Error(err, "failed to build policy context")
			errs = append(errs, err)
			continue
		}
		if admissionRequest != nil {
			var gvk schema.GroupVersionKind
			gvk, err = c.client.Discovery().GetGVKFromGVR(schema.GroupVersionResource(admissionRequest.Resource))
			if err != nil {
				logger.WithName(rule.Name).Error(err, "failed to get GVK from GVR", "GVR", admissionRequest.Resource)
				errs = append(errs, err)
				continue
			}
			policyContext = policyContext.WithResourceKind(gvk, admissionRequest.SubResource)
		}

		er := c.engine.Mutate(context.TODO(), policyContext)
		for _, r := range er.PolicyResponse.Rules {
			patched, parentGVR, patchedSubresource := r.PatchedTarget()
			switch r.Status() {
			case engineapi.RuleStatusFail, engineapi.RuleStatusError, engineapi.RuleStatusWarn:
				err := fmt.Errorf("failed to mutate existing resource, rule response%v: %s", r.Status(), r.Message())
				logger.Error(err, "")
				errs = append(errs, err)
				c.report(err, ur.Spec.Policy, rule.Name, patched)

			case engineapi.RuleStatusSkip:
				logger.Info("mutate existing rule skipped", "rule", r.Name(), "message", r.Message())
				c.report(err, ur.Spec.Policy, rule.Name, patched)

			case engineapi.RuleStatusPass:

				patchedNew, err := addAnnotation(policy, patched, r)
				if err != nil {
					logger.Error(err, "failed to apply patches")
					errs = append(errs, err)
				}

				if patchedNew == nil {
					logger.Error(ErrEmptyPatch, "", "rule", r.Name(), "message", r.Message())
					errs = append(errs, err)
					continue
				}

				if r.Status() == engineapi.RuleStatusPass {
					patchedNew.SetResourceVersion(patched.GetResourceVersion())
					var updateErr error
					if patchedSubresource == "status" {
						_, updateErr = c.client.UpdateStatusResource(context.TODO(), patchedNew.GetAPIVersion(), patchedNew.GetKind(), patchedNew.GetNamespace(), patchedNew.Object, false)
					} else if patchedSubresource != "" {
						parentResourceGVR := parentGVR
						parentResourceGV := schema.GroupVersion{Group: parentResourceGVR.Group, Version: parentResourceGVR.Version}
						parentResourceGVK, err := c.client.Discovery().GetGVKFromGVR(parentResourceGV.WithResource(parentResourceGVR.Resource))
						if err != nil {
							logger.Error(err, "failed to get GVK from GVR", "GVR", parentResourceGVR)
							errs = append(errs, err)
							continue
						}
						_, updateErr = c.client.UpdateResource(context.TODO(), parentResourceGV.String(), parentResourceGVK.Kind, patchedNew.GetNamespace(), patchedNew.Object, false, patchedSubresource)
					} else {
						_, updateErr = c.client.UpdateResource(context.TODO(), patchedNew.GetAPIVersion(), patchedNew.GetKind(), patchedNew.GetNamespace(), patchedNew.Object, false)
					}
					if updateErr != nil {
						errs = append(errs, updateErr)
						logger.WithName(rule.Name).Error(updateErr, "failed to update target resource", "namespace", patchedNew.GetNamespace(), "name", patchedNew.GetName())
					} else {
						logger.WithName(rule.Name).V(4).Info("successfully mutated existing resource", "namespace", patchedNew.GetNamespace(), "name", patchedNew.GetName())
					}

					c.report(updateErr, ur.Spec.Policy, rule.Name, patched)
				}
			}
		}
	}

	err = multierr.Combine(errs...)
	return updateURStatus(c.statusControl, *ur, err)
}

func (c *mutateExistingController) getPolicy(ur *kyvernov1beta1.UpdateRequest) (policy kyvernov1.PolicyInterface, err error) {
	pNamespace, pName, err := cache.SplitMetaNamespaceKey(ur.Spec.Policy)
	if err != nil {
		return nil, err
	}

	if pNamespace != "" {
		return c.npolicyLister.Policies(pNamespace).Get(pName)
	}

	return c.policyLister.Get(pName)
}

func (c *mutateExistingController) report(err error, policy, rule string, target *unstructured.Unstructured) {
	var events []event.Info

	if target == nil {
		c.log.WithName("mutateExisting").Info("cannot generate events for empty target resource", "policy", policy, "rule", rule)
	}

	if err != nil {
		events = event.NewBackgroundFailedEvent(err, policy, rule, event.MutateExistingController, target)
	} else {
		events = event.NewBackgroundSuccessEvent(policy, rule, event.MutateExistingController, target)
	}

	c.eventGen.Add(events...)
}

func updateURStatus(statusControl common.StatusControlInterface, ur kyvernov1beta1.UpdateRequest, err error) error {
	if err != nil {
		if _, err := statusControl.Failed(ur.GetName(), err.Error(), nil); err != nil {
			return err
		}
	} else {
		if _, err := statusControl.Success(ur.GetName(), nil); err != nil {
			return err
		}
	}
	return nil
}

func addAnnotation(policy kyvernov1.PolicyInterface, patched *unstructured.Unstructured, r engineapi.RuleResponse) (patchedNew *unstructured.Unstructured, err error) {
	if patched == nil {
		return
	}

	patchedNew = patched
	var rulePatches []utils.RulePatch

	for _, patch := range r.Patches() {
		var patchmap map[string]interface{}
		if err := json.Unmarshal(patch, &patchmap); err != nil {
			return nil, fmt.Errorf("failed to parse JSON patch bytes: %v", err)
		}

		rp := struct {
			RuleName string `json:"rulename"`
			Op       string `json:"op"`
			Path     string `json:"path"`
		}{
			RuleName: r.Name(),
			Op:       patchmap["op"].(string),
			Path:     patchmap["path"].(string),
		}

		rulePatches = append(rulePatches, rp)
	}

	annotationContent := make(map[string]string)
	policyName := policy.GetName()
	if policy.GetNamespace() != "" {
		policyName = policy.GetNamespace() + "/" + policy.GetName()
	}

	for _, rulePatch := range rulePatches {
		annotationContent[rulePatch.RuleName+"."+policyName+".kyverno.io"] = utils.OperationToPastTense[rulePatch.Op] + " " + rulePatch.Path
	}

	if len(annotationContent) == 0 {
		return
	}

	result, _ := yamlv2.Marshal(annotationContent)

	ann := patchedNew.GetAnnotations()
	if ann == nil {
		ann = make(map[string]string)
	}
	ann[utils.PolicyAnnotation] = string(result)
	patchedNew.SetAnnotations(ann)

	return
}
