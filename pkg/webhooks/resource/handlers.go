package resource

import (
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	enginectx "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	engineutils2 "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"github.com/kyverno/kyverno/pkg/utils"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	"github.com/kyverno/kyverno/pkg/webhooks"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/audit"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/mutation"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/validation"
	webhookgenerate "github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	"github.com/pkg/errors"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
		go h.handleUpdatesForGenerateRules(logger, request, []kyvernov1.PolicyInterface{})
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
		resource, err := utils.ConvertResource(request.OldObject.Raw, request.Kind.Group, request.Kind.Version, request.Kind.Kind, request.Namespace)
		if err == nil {
			h.prGenerator.Add(webhookutils.BuildDeletionPrInfo(resource))
		} else {
			logger.Info(fmt.Sprintf("Converting oldObject failed: %v", err))
		}
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
	imagePatches, imageVerifyWarnings, err := h.applyImageVerifyPolicies(logger, newRequest, policyContext, verifyImagesPolicies)
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

func (h *handlers) applyImageVerifyPolicies(logger logr.Logger, request *admissionv1.AdmissionRequest, policyContext *engine.PolicyContext, policies []kyvernov1.PolicyInterface) ([]byte, []string, error) {
	ok, message, imagePatches, warnings := h.handleVerifyImages(logger, request, policyContext, policies)
	if !ok {
		return nil, nil, errors.New(message)
	}

	logger.V(6).Info("images verified", "patches", string(imagePatches), "warnings", warnings)
	return imagePatches, warnings, nil
}

func (h *handlers) handleVerifyImages(logger logr.Logger, request *admissionv1.AdmissionRequest, policyContext *engine.PolicyContext, policies []kyvernov1.PolicyInterface) (bool, string, []byte, []string) {
	if len(policies) == 0 {
		return true, "", nil, nil
	}

	var engineResponses []*response.EngineResponse
	var patches [][]byte
	verifiedImageData := &engine.ImageVerificationMetadata{}
	for _, p := range policies {
		policyContext.Policy = p
		resp, ivm := engine.VerifyAndPatchImages(policyContext)

		engineResponses = append(engineResponses, resp)
		patches = append(patches, resp.GetPatches()...)
		verifiedImageData.Merge(ivm)
	}

	failurePolicy := policyContext.Policy.GetSpec().GetFailurePolicy()
	blocked := webhookutils.BlockRequest(engineResponses, failurePolicy, logger)
	if !isResourceDeleted(policyContext) {
		events := webhookutils.GenerateEvents(engineResponses, blocked)
		h.eventGen.Add(events...)
	}

	if blocked {
		logger.V(4).Info("admission request blocked")
		return false, webhookutils.GetBlockedMessages(engineResponses), nil, nil
	}

	prInfos := policyreport.GeneratePRsFromEngineResponse(engineResponses, logger)
	h.prGenerator.Add(prInfos...)

	if !verifiedImageData.IsEmpty() {
		hasAnnotations := hasAnnotations(policyContext)
		annotationPatches, err := verifiedImageData.Patches(hasAnnotations, logger)
		if err != nil {
			logger.Error(err, "failed to create image verification annotation patches")
		} else {
			// add annotation patches first
			patches = append(annotationPatches, patches...)
		}
	}

	warnings := webhookutils.GetWarningMessages(engineResponses)
	return true, "", jsonutils.JoinPatches(patches...), warnings
}

func isResourceDeleted(policyContext *engine.PolicyContext) bool {
	var deletionTimeStamp *metav1.Time
	if reflect.DeepEqual(policyContext.NewResource, unstructured.Unstructured{}) {
		deletionTimeStamp = policyContext.NewResource.GetDeletionTimestamp()
	} else {
		deletionTimeStamp = policyContext.OldResource.GetDeletionTimestamp()
	}
	return deletionTimeStamp != nil
}

func (h *handlers) handleDelete(logger logr.Logger, request *admissionv1.AdmissionRequest) {
	if request.Operation == admissionv1.Delete {
		resource, err := engineutils2.ConvertToUnstructured(request.OldObject.Raw)
		if err != nil {
			logger.Error(err, "failed to convert object resource to unstructured format")
		}

		resLabels := resource.GetLabels()
		if resLabels["app.kubernetes.io/managed-by"] == "kyverno" {
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
