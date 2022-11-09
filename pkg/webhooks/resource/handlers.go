package resource

import (
	"errors"
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
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	"github.com/kyverno/kyverno/pkg/webhooks"
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

	urGenerator    webhookgenerate.Generator
	eventGen       event.Interface
	openApiManager openapi.ValidateInterface
	pcBuilder      webhookutils.PolicyContextBuilder
	urUpdater      webhookutils.UpdateRequestUpdater

	admissionReports bool
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
	urGenerator webhookgenerate.Generator,
	eventGen event.Interface,
	openApiManager openapi.ValidateInterface,
	admissionReports bool,
) webhooks.ResourceHandlers {
	return &handlers{
		client:           client,
		kyvernoClient:    kyvernoClient,
		configuration:    configuration,
		metricsConfig:    metricsConfig,
		pCache:           pCache,
		nsLister:         nsLister,
		rbLister:         rbLister,
		crbLister:        crbLister,
		urLister:         urLister,
		urGenerator:      urGenerator,
		eventGen:         eventGen,
		openApiManager:   openApiManager,
		pcBuilder:        webhookutils.NewPolicyContextBuilder(configuration, client, rbLister, crbLister),
		urUpdater:        webhookutils.NewUpdateRequestUpdater(kyvernoClient, urLister),
		admissionReports: admissionReports,
	}
}

func (h *handlers) Validate(logger logr.Logger, request *admissionv1.AdmissionRequest, failurePolicy string, startTime time.Time) *admissionv1.AdmissionResponse {
	if webhookutils.ExcludeKyvernoResources(request.Kind.Kind) {
		return admissionutils.ResponseSuccess()
	}
	kind := request.Kind.Kind
	logger = logger.WithValues("kind", kind)
	logger.V(4).Info("received an admission request in validating webhook")

	// timestamp at which this admission request got triggered
	policies := filterPolicies(failurePolicy, h.pCache.GetPolicies(policycache.ValidateEnforce, kind, request.Namespace)...)
	mutatePolicies := filterPolicies(failurePolicy, h.pCache.GetPolicies(policycache.Mutate, kind, request.Namespace)...)
	generatePolicies := filterPolicies(failurePolicy, h.pCache.GetPolicies(policycache.Generate, kind, request.Namespace)...)
	imageVerifyValidatePolicies := filterPolicies(failurePolicy, h.pCache.GetPolicies(policycache.VerifyImagesValidate, kind, request.Namespace)...)
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

	vh := validation.NewValidationHandler(logger, h.kyvernoClient, h.pCache, h.pcBuilder, h.eventGen, h.admissionReports)

	ok, msg, warnings := vh.HandleValidation(h.metricsConfig, request, policies, policyContext, namespaceLabels, startTime)
	if !ok {
		logger.Info("admission request denied")
		return admissionutils.Response(errors.New(msg), warnings...)
	}

	defer h.handleDelete(logger, request)
	go h.createUpdateRequests(logger, request, policyContext, generatePolicies, mutatePolicies, startTime)

	return admissionutils.ResponseSuccess(warnings...)
}

func (h *handlers) Mutate(logger logr.Logger, request *admissionv1.AdmissionRequest, failurePolicy string, startTime time.Time) *admissionv1.AdmissionResponse {
	if webhookutils.ExcludeKyvernoResources(request.Kind.Kind) {
		return admissionutils.ResponseSuccess()
	}
	if request.Operation == admissionv1.Delete {
		return admissionutils.ResponseSuccess()
	}
	kind := request.Kind.Kind
	logger = logger.WithValues("kind", kind)
	logger.V(4).Info("received an admission request in mutating webhook")
	mutatePolicies := filterPolicies(failurePolicy, h.pCache.GetPolicies(policycache.Mutate, kind, request.Namespace)...)
	verifyImagesPolicies := filterPolicies(failurePolicy, h.pCache.GetPolicies(policycache.VerifyImagesMutate, kind, request.Namespace)...)
	if len(mutatePolicies) == 0 && len(verifyImagesPolicies) == 0 {
		logger.V(4).Info("no policies matched mutate admission request")
		return admissionutils.ResponseSuccess()
	}
	logger.V(4).Info("processing policies for mutate admission request", "mutatePolicies", len(mutatePolicies), "verifyImagesPolicies", len(verifyImagesPolicies))
	policyContext, err := h.pcBuilder.Build(request, mutatePolicies...)
	if err != nil {
		logger.Error(err, "failed to build policy context")
		return admissionutils.Response(err)
	}
	// update container images to a canonical form
	if err := enginectx.MutateResourceWithImageInfo(request.Object.Raw, policyContext.JSONContext); err != nil {
		logger.Error(err, "failed to patch images info to resource, policies that mutate images may be impacted")
	}
	mh := mutation.NewMutationHandler(logger, h.eventGen, h.openApiManager, h.nsLister)
	mutatePatches, mutateWarnings, err := mh.HandleMutation(h.metricsConfig, request, mutatePolicies, policyContext, startTime)
	if err != nil {
		logger.Error(err, "mutation failed")
		return admissionutils.Response(err)
	}
	newRequest := patchRequest(mutatePatches, request, logger)
	// rebuild context to process images updated via mutate policies
	policyContext, err = h.pcBuilder.Build(newRequest, mutatePolicies...)
	if err != nil {
		logger.Error(err, "failed to build policy context")
		return admissionutils.Response(err)
	}
	ivh := imageverification.NewImageVerificationHandler(logger, h.kyvernoClient, h.eventGen, h.admissionReports)
	imagePatches, imageVerifyWarnings, err := ivh.Handle(h.metricsConfig, newRequest, verifyImagesPolicies, policyContext)
	if err != nil {
		logger.Error(err, "image verification failed")
		return admissionutils.Response(err)
	}
	patch := jsonutils.JoinPatches(mutatePatches, imagePatches)
	var warnings []string
	warnings = append(warnings, mutateWarnings...)
	warnings = append(warnings, imageVerifyWarnings...)
	return admissionutils.MutationResponse(patch, warnings...)
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

func filterPolicies(failurePolicy string, policies ...kyvernov1.PolicyInterface) []kyvernov1.PolicyInterface {
	var results []kyvernov1.PolicyInterface
	for _, policy := range policies {
		if failurePolicy == "fail" {
			if policy.GetSpec().GetFailurePolicy() == kyvernov1.Fail {
				results = append(results, policy)
			}
		} else if failurePolicy == "ignore" {
			if policy.GetSpec().GetFailurePolicy() == kyvernov1.Ignore {
				results = append(results, policy)
			}
		} else {
			results = append(results, policy)
		}
	}
	return results
}
