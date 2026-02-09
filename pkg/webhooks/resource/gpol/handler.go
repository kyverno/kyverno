package gpol

import (
	"context"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

type handler struct {
	urGenerator updaterequest.Generator
	gpolLister  policiesv1beta1listers.GeneratingPolicyLister
	ngpolLister policiesv1beta1listers.NamespacedGeneratingPolicyLister
}

func New(
	urGenerator updaterequest.Generator,
	gpolLister policiesv1beta1listers.GeneratingPolicyLister,
	ngpolLister policiesv1beta1listers.NamespacedGeneratingPolicyLister,
) *handler {
	return &handler{
		urGenerator: urGenerator,
		gpolLister:  gpolLister,
		ngpolLister: ngpolLister,
	}
}

func (h *handler) Generate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, _ string, _ time.Time) handlers.AdmissionResponse {
	// Skip background generation for dry-run requests to honor the SideEffects: NoneOnDryRun contract.
	if admissionutils.IsDryRun(request.AdmissionRequest) {
		return admissionutils.Response(request.UID, nil)
	}

	var policies []string
	if params := httprouter.ParamsFromContext(ctx); params != nil {
		if params := strings.Split(strings.TrimLeft(params.ByName("policies"), "/"), "/"); len(params) != 0 {
			policies = params
		}
	}

	go func(policies []string, request handlers.AdmissionRequest, logger logr.Logger) {
		admissionRequest := request.AdmissionRequest
		userInfo := kyvernov2.RequestInfo{
			AdmissionUserInfo: *request.UserInfo.DeepCopy(),
			Roles:             request.Roles,
			ClusterRoles:      request.ClusterRoles,
		}
		for _, policy := range policies {
			trigger, oldTrigger, err := admissionutils.ExtractResources(nil, admissionRequest)
			if err != nil {
				logger.Error(err, "failed to extract resources from admission request")
				break
			}
			if trigger.Object == nil {
				trigger = oldTrigger
			}
			triggerSpec := kyvernov1.ResourceSpec{
				APIVersion: trigger.GetAPIVersion(),
				Kind:       trigger.GetKind(),
				Namespace:  trigger.GetNamespace(),
				Name:       trigger.GetName(),
				UID:        trigger.GetUID(),
			}
			if request.Operation == admissionv1.Delete {
				gpol, err := h.gpolLister.Get(policy)
				if err != nil {
					logger.Error(err, "failed to get generating policy", "policy", policy)
					continue
				}
				// in case of delete operation, if the policy matches the delete operation, we need to fire the generation
				// otherwise, we need to delete the downstream resources
				deleteDownstream := true
				for _, rule := range gpol.Spec.MatchConstraints.ResourceRules {
					for _, op := range rule.Operations {
						if op == admissionregistrationv1.Delete {
							deleteDownstream = false
							break
						}
					}
				}
				if deleteDownstream {
					// delete downstream on trigger deletion in case synchronization is enabled
					if gpol.Spec.SynchronizationEnabled() {
						logger.V(4).Info("creating the UR to delete downstream on trigger's deletion", "operation", request.Operation, "policy", policy, "trigger", triggerSpec.String())
						urSpec := buildURSpecNew(kyvernov2.CELGenerate, policy, triggerSpec, true, false)
						urSpec.Context = buildURContext(admissionRequest, userInfo)
						if err := h.urGenerator.Apply(ctx, urSpec); err != nil {
							logger.Error(err, "failed to create update request for generate policy", "policy", policy)
						} else {
							logger.V(4).Info("update request created for generate policy", "policy", policy)
						}
					}
				} else {
					// fire generation on trigger deletion
					logger.V(4).Info("creating the UR to generate downstream on trigger's deletion", "operation", request.Operation, "policy", policy, "trigger", triggerSpec.String())
					urSpec := buildURSpecNew(kyvernov2.CELGenerate, policy, triggerSpec, false, false)
					urSpec.Context = buildURContext(admissionRequest, userInfo)
					if err := h.urGenerator.Apply(ctx, urSpec); err != nil {
						logger.Error(err, "failed to create update request for generate policy", "policy", policy)
					} else {
						logger.V(4).Info("update request created for generate policy", "policy", policy)
					}
				}
			} else {
				synchronize := false
				if request.Operation == admissionv1.Update {
					gpol, err := h.gpolLister.Get(policy)
					if err != nil {
						logger.Error(err, "failed to get generating policy", "policy", policy)
					} else {
						synchronize = gpol.Spec.SynchronizationEnabled()
					}
				}
				logger.V(4).Info("creating the UR to generate downstream on trigger's operation", "operation", request.Operation, "policy", policy, "synchronize", synchronize)
				urSpec := buildURSpecNew(kyvernov2.CELGenerate, policy, triggerSpec, false, synchronize)
				urSpec.Context = buildURContext(admissionRequest, userInfo)
				if err := h.urGenerator.Apply(ctx, urSpec); err != nil {
					logger.Error(err, "failed to create update request for generate policy", "policy", policy)
				} else {
					logger.V(4).Info("update request created for generate policy", "policy", policy)
				}
			}
		}
	}(policies, request, logger)

	return admissionutils.Response(request.UID, nil)
}

func (h *handler) GenerateNamespaced(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, _ string, _ time.Time) handlers.AdmissionResponse {
	if request.Namespace == "" {
		return admissionutils.ResponseSuccess(request.UID)
	}
	// Skip background generation for dry-run requests to honor the SideEffects: NoneOnDryRun contract.
	if admissionutils.IsDryRun(request.AdmissionRequest) {
		return admissionutils.Response(request.UID, nil)
	}
	var policies []string
	if params := httprouter.ParamsFromContext(ctx); params != nil {
		if params := strings.Split(strings.TrimLeft(params.ByName("policies"), "/"), "/"); len(params) != 0 {
			policies = params
		}
	}

	go func(policies []string, request handlers.AdmissionRequest, logger logr.Logger, namespace string) {
		admissionRequest := request.AdmissionRequest
		userInfo := kyvernov2.RequestInfo{
			AdmissionUserInfo: *request.UserInfo.DeepCopy(),
			Roles:             request.Roles,
			ClusterRoles:      request.ClusterRoles,
		}
		for _, policy := range policies {
			trigger, oldTrigger, err := admissionutils.ExtractResources(nil, admissionRequest)
			if err != nil {
				logger.Error(err, "failed to extract resources from admission request")
				break
			}
			if trigger.Object == nil {
				trigger = oldTrigger
			}
			triggerSpec := kyvernov1.ResourceSpec{
				APIVersion: trigger.GetAPIVersion(),
				Kind:       trigger.GetKind(),
				Namespace:  trigger.GetNamespace(),
				Name:       trigger.GetName(),
				UID:        trigger.GetUID(),
			}
			if request.Operation == admissionv1.Delete {
				ngpol, err := h.ngpolLister.NamespacedGeneratingPolicies(namespace).Get(policy)
				if err != nil {
					logger.Error(err, "failed to get namespaced generating policy", "policy", policy, "namespace", namespace)
					continue
				}
				deleteDownstream := true
				for _, rule := range ngpol.Spec.MatchConstraints.ResourceRules {
					for _, op := range rule.Operations {
						if op == admissionregistrationv1.Delete {
							deleteDownstream = false
							break
						}
					}
				}
				if deleteDownstream {
					if ngpol.Spec.SynchronizationEnabled() {
						logger.V(4).Info("creating the UR to delete downstream on trigger's deletion", "operation", request.Operation, "policy", policy, "namespace", namespace, "trigger", triggerSpec.String())
						policyKey := namespace + "/" + policy
						urSpec := buildURSpecNew(kyvernov2.CELGenerate, policyKey, triggerSpec, true, false)
						urSpec.Context = buildURContext(admissionRequest, userInfo)
						if err := h.urGenerator.Apply(ctx, urSpec); err != nil {
							logger.Error(err, "failed to create update request for namespaced generate policy", "policy", policy)
						} else {
							logger.V(4).Info("update request created for namespaced generate policy", "policy", policy)
						}
					}
				} else {
					logger.V(4).Info("creating the UR to generate downstream on trigger's deletion", "operation", request.Operation, "policy", policy, "namespace", namespace, "trigger", triggerSpec.String())
					policyKey := namespace + "/" + policy
					urSpec := buildURSpecNew(kyvernov2.CELGenerate, policyKey, triggerSpec, false, false)
					urSpec.Context = buildURContext(admissionRequest, userInfo)
					if err := h.urGenerator.Apply(ctx, urSpec); err != nil {
						logger.Error(err, "failed to create update request for namespaced generate policy", "policy", policy)
					} else {
						logger.V(4).Info("update request created for namespaced generate policy", "policy", policy)
					}
				}
			} else {
				synchronize := false
				if request.Operation == admissionv1.Update {
					gpol, err := h.gpolLister.Get(policy)
					if err != nil {
						logger.Error(err, "failed to get generating policy", "policy", policy)
					} else {
						synchronize = gpol.Spec.SynchronizationEnabled()
					}
				}
				logger.V(4).Info("creating the UR to generate downstream on trigger's operation", "operation", request.Operation, "policy", policy, "namespace", namespace)
				policyKey := namespace + "/" + policy
				urSpec := buildURSpecNew(kyvernov2.CELGenerate, policyKey, triggerSpec, false, synchronize)
				urSpec.Context = buildURContext(admissionRequest, userInfo)
				if err := h.urGenerator.Apply(ctx, urSpec); err != nil {
					logger.Error(err, "failed to create update request for namespaced generate policy", "policy", policy)
				} else {
					logger.V(4).Info("update request created for namespaced generate policy", "policy", policy)
				}
			}
		}
	}(policies, request, logger, request.Namespace)

	return admissionutils.Response(request.UID, nil)
}
