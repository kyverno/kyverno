package gpol

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type handler struct {
	urGenerator                  updaterequest.Generator
	gpolLister                   policiesv1beta1listers.GeneratingPolicyLister
	ngpolLister                  policiesv1beta1listers.NamespacedGeneratingPolicyLister
	backgroundServiceAccountName string
}

func New(
	urGenerator updaterequest.Generator,
	gpolLister policiesv1beta1listers.GeneratingPolicyLister,
	ngpolLister policiesv1beta1listers.NamespacedGeneratingPolicyLister,
	backgroundServiceAccountName string,
) *handler {
	return &handler{
		urGenerator:                  urGenerator,
		gpolLister:                   gpolLister,
		ngpolLister:                  ngpolLister,
		backgroundServiceAccountName: backgroundServiceAccountName,
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
		h.queuePolicies(ctx, logger, policies, request, func(name string) (policiesv1beta1.GeneratingPolicyLike, error) {
			if h.gpolLister == nil {
				return nil, errNoPolicyLister
			}
			return h.gpolLister.Get(name)
		}, func(name string) string {
			return name
		})
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

	namespace := request.Namespace
	go func(policies []string, request handlers.AdmissionRequest, logger logr.Logger, namespace string) {
		h.queuePolicies(ctx, logger, policies, request, func(name string) (policiesv1beta1.GeneratingPolicyLike, error) {
			if h.ngpolLister == nil {
				return nil, errNoPolicyLister
			}
			return h.ngpolLister.NamespacedGeneratingPolicies(namespace).Get(name)
		}, func(name string) string {
			return namespace + "/" + name
		})
	}(policies, request, logger, namespace)

	return admissionutils.Response(request.UID, nil)
}

var errNoPolicyLister = errors.New("generating policy lister is not configured")

func (h *handler) queuePolicies(
	ctx context.Context,
	logger logr.Logger,
	policies []string,
	request handlers.AdmissionRequest,
	load func(string) (policiesv1beta1.GeneratingPolicyLike, error),
	policyKey func(string) string,
) {
	admissionRequest := request.AdmissionRequest
	isBackgroundRequest := h.backgroundServiceAccountName != "" && request.UserInfo.Username == h.backgroundServiceAccountName
	userInfo := kyvernov2.RequestInfo{
		AdmissionUserInfo: *request.UserInfo.DeepCopy(),
		Roles:             request.Roles,
		ClusterRoles:      request.ClusterRoles,
	}
	object, oldObject, err := admissionutils.ExtractResources(nil, admissionRequest)
	if err != nil {
		logger.Error(err, "failed to extract resources from admission request")
		return
	}
	for _, name := range policies {
		policy, err := load(name)
		if err != nil {
			logger.Error(err, "failed to get generating policy", "policy", name)
			// A non-background CREATE used to queue without reading the policy.
			// Keep that when the lister has not seen it yet. UPDATE and DELETE
			// already required the policy, and a background CREATE is skipped
			// when the policy cannot be loaded.
			if !isBackgroundRequest && request.Operation == admissionv1.Create {
				h.queueUpdateRequest(ctx, logger, policyKey(name), admissionRequest, userInfo, object, oldObject, admissionPlan{queue: true})
			}
			continue
		}
		if isBackgroundRequest && skipBackgroundRequests(policy) {
			logger.V(4).Info("skipping background request for generating policy", "policy", name)
			continue
		}
		plan := planGeneratingAdmission(ctx, policy, admissionRequest, object, oldObject)
		if !plan.queue {
			logger.V(4).Info("skipping update request; resource is not a trigger for this generating policy", "policy", name, "operation", request.Operation)
			continue
		}
		h.queueUpdateRequest(ctx, logger, policyKey(name), admissionRequest, userInfo, object, oldObject, plan)
	}
}

func (h *handler) queueUpdateRequest(
	ctx context.Context,
	logger logr.Logger,
	policyKey string,
	admissionRequest admissionv1.AdmissionRequest,
	userInfo kyvernov2.RequestInfo,
	object unstructured.Unstructured,
	oldObject unstructured.Unstructured,
	plan admissionPlan,
) {
	trigger := object
	if trigger.Object == nil {
		trigger = oldObject
	}
	triggerSpec := kyverno.ResourceSpec{
		APIVersion: trigger.GetAPIVersion(),
		Kind:       trigger.GetKind(),
		Namespace:  trigger.GetNamespace(),
		Name:       trigger.GetName(),
		UID:        trigger.GetUID(),
	}
	if plan.deleteDownstream {
		logger.V(4).Info("creating the UR to delete downstream on trigger's deletion", "operation", admissionRequest.Operation, "policy", policyKey, "trigger", triggerSpec.String())
	} else {
		logger.V(4).Info("creating the UR to generate downstream on trigger's operation", "operation", admissionRequest.Operation, "policy", policyKey, "synchronize", plan.synchronize, "trigger", triggerSpec.String())
	}
	urSpec := buildURSpecNew(kyvernov2.CELGenerate, policyKey, triggerSpec, plan.deleteDownstream, plan.synchronize)
	urSpec.Context = buildURContext(admissionRequest, userInfo)
	if err := h.urGenerator.Apply(ctx, urSpec); err != nil {
		logger.Error(err, "failed to create update request for generate policy", "policy", policyKey)
		return
	}
	logger.V(4).Info("update request created for generate policy", "policy", policyKey)
}

func skipBackgroundRequests(policy policiesv1beta1.GeneratingPolicyLike) bool {
	return policy.GetSpec().SkipBackgroundRequestsEnabled()
}
