package mutate

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/breaker"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	engineutils "github.com/kyverno/kyverno/pkg/utils/engine"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"go.uber.org/multierr"
	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

var ErrEmptyPatch error = fmt.Errorf("empty resource to patch")

type mutateExistingController struct {
	// clients
	client        dclient.Interface
	kyvernoClient versioned.Interface
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
	kyvernoClient versioned.Interface,
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
		kyvernoClient: kyvernoClient,
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

func (c *mutateExistingController) ProcessUR(ur *kyvernov2.UpdateRequest) error {
	logger := c.log.WithValues("name", ur.GetName(), "policy", ur.Spec.GetPolicyKey(), "resource", ur.Spec.GetResource().String())
	var errs []error

	logger.V(3).Info("processing mutate existing")
	policy, err := c.getPolicy(ur)
	if err != nil {
		logger.Error(err, "failed to get policy")
		return err
	}

	for _, rule := range policy.GetSpec().Rules {
		if !rule.HasMutateExisting() || ur.Spec.Rule != rule.Name {
			continue
		}

		var trigger *unstructured.Unstructured
		admissionRequest := ur.Spec.Context.AdmissionRequestInfo.AdmissionRequest
		if admissionRequest == nil {
			trigger, err = common.GetResource(c.client, ur.Spec.Resource, ur.Spec, c.log)
			if err != nil || trigger == nil {
				logger.WithName(rule.Name).Error(err, "failed to get trigger resource")
				if err := updateURStatus(c.statusControl, *ur, err); err != nil {
					return err
				}
				continue
			}
		} else {
			if admissionRequest.Operation == admissionv1.Create {
				trigger, err = common.GetResource(c.client, ur.Spec.Resource, ur.Spec, c.log)
				if err != nil || trigger == nil {
					if admissionRequest.SubResource == "" {
						logger.WithName(rule.Name).Error(err, "failed to get trigger resource")
						if err := updateURStatus(c.statusControl, *ur, err); err != nil {
							return err
						}
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

		namespaceLabels, err := engineutils.GetNamespaceSelectorsFromNamespaceLister(trigger.GetKind(), trigger.GetNamespace(), c.nsLister, []kyvernov1.PolicyInterface{policy}, logger)
		if err != nil {
			logger.WithName(rule.Name).Error(err, "failed to get namespace labels")
			errs = append(errs, err)
			continue
		}

		policyContext, err := common.NewBackgroundContext(logger, c.client, ur.Spec.Context, policy, trigger, c.configuration, c.jp, namespaceLabels)
		if err != nil {
			logger.WithName(rule.Name).Error(err, "failed to build policy context")
			errs = append(errs, err)
			continue
		}
		if admissionRequest != nil {
			var gvk schema.GroupVersionKind
			gvk, err = c.client.Discovery().GetGVKFromGVR(schema.GroupVersionResource(admissionRequest.Resource))
			if err != nil {
				logger.WithName(rule.Name).Error(err, "failed to get GVK from GVR", "GVR", admissionRequest.Resource.String())
				errs = append(errs, err)
				continue
			}
			policyContext = policyContext.WithResourceKind(gvk, admissionRequest.SubResource)
		}

		// A conflict means another writer changed the target between the read and the
		// update. The mutation is recomputed rather than replayed, because a patch may
		// be derived from the target's current content: re-sending it against a newer
		// resourceVersion would overwrite the other writer instead of merging with it.
		var er engineapi.EngineResponse
		var applyErrs []error
		var reports []targetMutation
		conflictErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			var conflict error
			er = c.engine.Mutate(context.TODO(), policyContext)
			reports, applyErrs, conflict = c.applyMutations(logger, rule.Name, er)
			return conflict
		})

		if c.needsReports(trigger) && reportutils.IsPolicyReportable(policy) {
			if err := c.createReports(context.TODO(), policyContext.NewResource(), er); err != nil {
				c.log.Error(err, "failed to create report")
			}
		}

		if conflictErr != nil {
			logger.WithName(rule.Name).Error(conflictErr, "failed to update target resource after retrying")
		}
		// applyErrs and reports come from the final attempt, so a conflict that
		// outlived the retries is already carried in them.
		errs = append(errs, applyErrs...)
		for _, r := range reports {
			c.report(r.err, policy, rule.Name, r.target)
		}
	}

	err = multierr.Combine(errs...)
	return updateURStatus(c.statusControl, *ur, err)
}

// targetMutation is the outcome of applying one rule response to its target
// resource, held until the enclosing retry settles so a retried attempt does not
// emit duplicate events.
type targetMutation struct {
	err    error
	target *unstructured.Unstructured
}

// applyMutations applies every rule response in er to its target resource. A
// conflict is returned to the caller instead of being collected, so the caller
// can recompute the mutation against the latest version of the target and try
// again; every other outcome is collected and reported once.
func (c *mutateExistingController) applyMutations(logger logr.Logger, ruleName string, er engineapi.EngineResponse) (reports []targetMutation, errs []error, conflict error) {
	for _, r := range er.PolicyResponse.Rules {
		patched, parentGVR, patchedSubresource := r.PatchedTarget()
		switch r.Status() {
		case engineapi.RuleStatusFail, engineapi.RuleStatusError, engineapi.RuleStatusWarn:
			err := fmt.Errorf("failed to mutate existing resource, rule %s, response %v: %s", r.Name(), r.Status(), r.Message())
			logger.Error(err, "")
			errs = append(errs, err)
			reports = append(reports, targetMutation{err: err, target: patched})

		case engineapi.RuleStatusSkip:
			err := fmt.Errorf("mutate existing rule skipped, rule %s, response %v: %s", r.Name(), r.Status(), r.Message())
			logger.V(4).Info(err.Error())

		case engineapi.RuleStatusPass:
			patchedNew := patched
			if patchedNew == nil {
				logger.Error(ErrEmptyPatch, "", "rule", r.Name(), "message", r.Message())
				errs = append(errs, ErrEmptyPatch)
				continue
			}

			patchedNew.SetResourceVersion(patched.GetResourceVersion())
			var updateErr error
			if patchedSubresource == "status" {
				_, updateErr = c.client.UpdateStatusResource(context.TODO(), patchedNew.GetAPIVersion(), patchedNew.GetKind(), patchedNew.GetNamespace(), patchedNew.Object, false)
			} else if patchedSubresource != "" {
				parentResourceGVR := parentGVR
				parentResourceGV := schema.GroupVersion{Group: parentResourceGVR.Group, Version: parentResourceGVR.Version}
				parentResourceGVK, err := c.client.Discovery().GetGVKFromGVR(parentResourceGV.WithResource(parentResourceGVR.Resource))
				if err != nil {
					logger.Error(err, "failed to get GVK from GVR", "GVR", parentResourceGVR.String())
					errs = append(errs, err)
					continue
				}
				_, updateErr = c.client.UpdateResource(context.TODO(), parentResourceGV.String(), parentResourceGVK.Kind, patchedNew.GetNamespace(), patchedNew.Object, false, patchedSubresource)
			} else {
				_, updateErr = c.client.UpdateResource(context.TODO(), patchedNew.GetAPIVersion(), patchedNew.GetKind(), patchedNew.GetNamespace(), patchedNew.Object, false)
			}

			if apierrors.IsConflict(updateErr) {
				logger.WithName(ruleName).V(3).Info("conflict updating target resource, recomputing the mutation and retrying",
					"namespace", patchedNew.GetNamespace(), "name", patchedNew.GetName())
				// Carried, not discarded: the caller keeps only the final attempt's
				// results, so a conflict that outlives the retries is reported like
				// any other terminal failure, and outcomes for targets already
				// applied in this attempt are not lost.
				errs = append(errs, updateErr)
				reports = append(reports, targetMutation{err: updateErr, target: patched})
				return reports, errs, updateErr
			}

			if updateErr != nil {
				errs = append(errs, updateErr)
				logger.WithName(ruleName).Error(updateErr, "failed to update target resource", "namespace", patchedNew.GetNamespace(), "name", patchedNew.GetName())
			} else {
				logger.WithName(ruleName).V(4).Info("successfully mutated existing resource", "namespace", patchedNew.GetNamespace(), "name", patchedNew.GetName())
			}
			reports = append(reports, targetMutation{err: updateErr, target: patched})
		}
	}
	return reports, errs, nil
}

func (c *mutateExistingController) getPolicy(ur *kyvernov2.UpdateRequest) (policy kyvernov1.PolicyInterface, err error) {
	pNamespace, pName, err := cache.SplitMetaNamespaceKey(ur.Spec.Policy)
	if err != nil {
		return nil, err
	}

	if pNamespace != "" {
		return c.npolicyLister.Policies(pNamespace).Get(pName)
	}

	return c.policyLister.Get(pName)
}

func (c *mutateExistingController) report(err error, policy kyvernov1.PolicyInterface, rule string, target *unstructured.Unstructured) {
	var events []event.Info

	if target == nil {
		c.log.WithName("mutateExisting").Info("cannot generate events for empty target resource", "policy", policy.GetName(), "rule", rule)
		return
	}

	if err != nil {
		events = event.NewBackgroundFailedEvent(err, engineapi.NewKyvernoPolicy(policy), rule, event.MutateExistingController,
			kyvernov1.ResourceSpec{Kind: target.GetKind(), Namespace: target.GetNamespace(), Name: target.GetName()})
	} else {
		events = event.NewBackgroundSuccessEvent(event.MutateExistingController, engineapi.NewKyvernoPolicy(policy),
			[]kyvernov1.ResourceSpec{{Kind: target.GetKind(), Namespace: target.GetNamespace(), Name: target.GetName()}})
	}

	c.eventGen.Add(events...)
}

func (c *mutateExistingController) needsReports(trigger *unstructured.Unstructured) bool {
	createReport := reportutils.ReportingCfg.MutateExistingReportsEnabled()
	if trigger == nil {
		return createReport
	}
	if !reportutils.IsGvkSupported(trigger.GroupVersionKind()) {
		createReport = false
	}

	return createReport
}

func (c *mutateExistingController) createReports(
	ctx context.Context,
	resource unstructured.Unstructured,
	engineResponses ...engineapi.EngineResponse,
) error {
	// Skip report creation for resources with incomplete identity metadata (for example subresources such as pods/exec).
	// Reports require both name and UID; if either is empty, the generated report would be invalid.
	if resource.GetName() == "" || resource.GetUID() == "" {
		c.log.V(3).Info("skipping report creation for subresource with empty name or uid", "gvk", resource.GroupVersionKind())
		return nil
	}

	report := reportutils.BuildMutateExistingReport(resource.GetNamespace(), resource.GroupVersionKind(), resource.GetName(), resource.GetUID(), engineResponses...)
	if len(report.GetResults()) > 0 {
		err := breaker.GetReportsBreaker().Do(ctx, func(ctx context.Context) error {
			_, err := reportutils.CreateEphemeralReport(ctx, report, c.kyvernoClient)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func updateURStatus(statusControl common.StatusControlInterface, ur kyvernov2.UpdateRequest, err error) error {
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
