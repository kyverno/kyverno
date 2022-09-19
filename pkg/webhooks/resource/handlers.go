package resource

import (
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	enginectx "github.com/kyverno/kyverno/pkg/engine/context"
	engineutils2 "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/policyreport"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	"github.com/kyverno/kyverno/pkg/webhooks"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/audit"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/generation"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/imageverification"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/mutation"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/validation"
	webhookgenerate "github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	admissionv1 "k8s.io/api/admission/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
)

type handlers struct {
	// clients
	client        dclient.Interface
	kyvernoClient versioned.Interface

	// config
	configuration config.Configuration
	metricsConfig *metrics.MetricsConfig

	// cache
	pCache policycache.Cache

	// listers
	nsLister  corev1listers.NamespaceLister
	rbLister  rbacv1listers.RoleBindingLister
	crbLister rbacv1listers.ClusterRoleBindingLister
	urLister  kyvernov1beta1listers.UpdateRequestNamespaceLister

	prGenerator       policyreport.GeneratorInterface
	urGenerator       webhookgenerate.Generator
	eventGen          event.Interface
	auditHandler      audit.AuditHandler
	openAPIController openapi.ValidateInterface
	pcBuilder         webhookutils.PolicyContextBuilder
	urUpdater         webhookutils.UpdateRequestUpdater
}

func NewHandlers(
	client dclient.Interface,
	kyvernoClient versioned.Interface,
	configuration config.Configuration,
	metricsConfig *metrics.MetricsConfig,
	pCache policycache.Cache,
	nsLister corev1listers.NamespaceLister,
	rbLister rbacv1listers.RoleBindingLister,
	crbLister rbacv1listers.ClusterRoleBindingLister,
	urLister kyvernov1beta1listers.UpdateRequestNamespaceLister,
	prGenerator policyreport.GeneratorInterface,
	urGenerator webhookgenerate.Generator,
	eventGen event.Interface,
	auditHandler audit.AuditHandler,
	openAPIController openapi.ValidateInterface,
) webhooks.Handlers {
	return &handlers{
		client:            client,
		kyvernoClient:     kyvernoClient,
		configuration:     configuration,
		metricsConfig:     metricsConfig,
		pCache:            pCache,
		nsLister:          nsLister,
		rbLister:          rbLister,
		crbLister:         crbLister,
		urLister:          urLister,
		prGenerator:       prGenerator,
		urGenerator:       urGenerator,
		eventGen:          eventGen,
		auditHandler:      auditHandler,
		openAPIController: openAPIController,
		pcBuilder:         webhookutils.NewPolicyContextBuilder(configuration, client, rbLister, crbLister),
		urUpdater:         webhookutils.NewUpdateRequestUpdater(kyvernoClient, urLister),
	}
}

func (h *handlers) Validate(logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
	if webhookutils.ExcludeKyvernoResources(request.Kind.Kind) {
		return admissionutils.ResponseSuccess()
	}
	kind := request.Kind.Kind
	logger = logger.WithValues("kind", kind)
	logger.V(4).Info("received an admission request in validating webhook")

	// timestamp at which this admission request got triggered
	policies := h.pCache.GetPolicies(policycache.ValidateEnforce, kind, request.Namespace)
	mutatePolicies := h.pCache.GetPolicies(policycache.Mutate, kind, request.Namespace)
	generatePolicies := h.pCache.GetPolicies(policycache.Generate, kind, request.Namespace)
	imageVerifyValidatePolicies := h.pCache.GetPolicies(policycache.VerifyImagesValidate, kind, request.Namespace)
	policies = append(policies, imageVerifyValidatePolicies...)

	if len(policies) == 0 && len(mutatePolicies) == 0 && len(generatePolicies) == 0 {
		logger.V(4).Info("no policies matched admission request")
	}
	if len(generatePolicies) == 0 && request.Operation == admissionv1.Update {
		// handle generate source resource updates
		gh := generation.NewGenerationHandler(logger, h.client, h.kyvernoClient, h.nsLister, h.urLister, h.urGenerator, h.urUpdater, h.eventGen)
		go gh.HandleUpdatesForGenerateRules(request, []kyvernov1.PolicyInterface{})
	}

	logger.V(4).Info("processing policies for validate admission request", "validate", len(policies), "mutate", len(mutatePolicies), "generate", len(generatePolicies))

	policyContext, err := h.pcBuilder.Build(request, generatePolicies...)
	if err != nil {
		return errorResponse(logger, err, "failed create policy context")
	}

	namespaceLabels := make(map[string]string)
	if request.Kind.Kind != "Namespace" && request.Namespace != "" {
		namespaceLabels = common.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, h.nsLister, logger)
	}

	vh := validation.NewValidationHandler(logger, h.eventGen, h.prGenerator)

	ok, msg, warnings := vh.HandleValidation(h.metricsConfig, request, policies, policyContext, namespaceLabels, startTime)
	if !ok {
		logger.Info("admission request denied")
		return admissionutils.ResponseFailure(msg)
	}
	defer func() { h.handleDelete(logger, request) }()

	h.auditHandler.Add(request.DeepCopy())
	go h.createUpdateRequests(logger, request, policyContext, generatePolicies, mutatePolicies, startTime)

	if warnings != nil {
		return admissionutils.ResponseSuccessWithWarnings(warnings)
	}

	logger.V(4).Info("completed validating webhook")
	return admissionutils.ResponseSuccess()
}

func (h *handlers) Mutate(logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
	if webhookutils.ExcludeKyvernoResources(request.Kind.Kind) {
		return admissionutils.ResponseSuccess()
	}
	if request.Operation == admissionv1.Delete {
		return admissionutils.ResponseSuccess()
	}
	kind := request.Kind.Kind
	logger = logger.WithValues("kind", kind)
	logger.V(4).Info("received an admission request in mutating webhook")
	mutatePolicies := h.pCache.GetPolicies(policycache.Mutate, kind, request.Namespace)
	verifyImagesPolicies := h.pCache.GetPolicies(policycache.VerifyImagesMutate, kind, request.Namespace)
	if len(mutatePolicies) == 0 && len(verifyImagesPolicies) == 0 {
		logger.V(4).Info("no policies matched mutate admission request")
		return admissionutils.ResponseSuccess()
	}
	logger.V(4).Info("processing policies for mutate admission request", "mutatePolicies", len(mutatePolicies), "verifyImagesPolicies", len(verifyImagesPolicies))
	policyContext, err := h.pcBuilder.Build(request, mutatePolicies...)
	if err != nil {
		logger.Error(err, "failed to build policy context")
		return admissionutils.ResponseFailure(err.Error())
	}
	// update container images to a canonical form
	if err := enginectx.MutateResourceWithImageInfo(request.Object.Raw, policyContext.JSONContext); err != nil {
		logger.Error(err, "failed to patch images info to resource, policies that mutate images may be impacted")
	}

	mh := mutation.NewMutationHandler(logger, h.eventGen, h.openAPIController, h.nsLister)
	mutatePatches, mutateWarnings, err := mh.HandleMutation(h.metricsConfig, request, mutatePolicies, policyContext, startTime)
	if err != nil {
		logger.Error(err, "mutation failed")
		return admissionutils.ResponseFailure(err.Error())
	}
	newRequest := patchRequest(mutatePatches, request, logger)
	ivh := imageverification.NewImageVerificationHandler(logger, h.eventGen, h.prGenerator)
	imagePatches, imageVerifyWarnings, err := ivh.Handle(h.metricsConfig, newRequest, verifyImagesPolicies, policyContext)
	if err != nil {
		logger.Error(err, "image verification failed")
		return admissionutils.ResponseFailure(err.Error())
	}
	patch := jsonutils.JoinPatches(mutatePatches, imagePatches)
	if len(mutateWarnings) > 0 || len(imageVerifyWarnings) > 0 {
		warnings := append(mutateWarnings, imageVerifyWarnings...)
		logger.V(2).Info("mutation webhook", "warnings", warnings)
		return admissionutils.ResponseSuccessWithPatchAndWarnings(patch, warnings)
	}
	admissionResponse := admissionutils.ResponseSuccessWithPatch(patch)
	logger.V(4).Info("completed mutating webhook", "response", admissionResponse)
	return admissionResponse
}

func (h *handlers) handleDelete(logger logr.Logger, request *admissionv1.AdmissionRequest) {
	if request.Operation == admissionv1.Delete {
		resource, err := engineutils2.ConvertToUnstructured(request.OldObject.Raw)
		if err != nil {
			logger.Error(err, "failed to convert object resource to unstructured format")
		}

		resLabels := resource.GetLabels()
		if resLabels[kyvernov1.LabelAppManagedBy] == kyvernov1.ValueKyvernoApp {
			urName := resLabels["policy.kyverno.io/gr-name"]
			ur, err := h.urLister.Get(urName)
			if err != nil {
				logger.Error(err, "failed to get update request", "name", urName)
				return
			}

			if ur.Spec.Type == kyvernov1beta1.Mutate {
				return
			}
			h.urUpdater.UpdateAnnotation(logger, ur.GetName())
		}
	}
}
