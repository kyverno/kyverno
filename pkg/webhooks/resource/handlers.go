package resource

import (
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	enginectx "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	engineutils2 "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/openapi"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"github.com/kyverno/kyverno/pkg/userinfo"
	"github.com/kyverno/kyverno/pkg/utils"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	engineutils "github.com/kyverno/kyverno/pkg/utils/engine"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	"github.com/kyverno/kyverno/pkg/webhooks"
	webhookgenerate "github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
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
	kyvernoClient kyvernoclient.Interface

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
	auditHandler      AuditHandler
	openAPIController openapi.ValidateInterface
}

func NewHandlers(
	client dclient.Interface,
	kyvernoClient kyvernoclient.Interface,
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
	auditHandler AuditHandler,
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
	}
}

func (h *handlers) Validate(logger logr.Logger, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	if request.Operation == admissionv1.Delete {
		h.handleDelete(logger, request)
	}

	if excludeKyvernoResources(request.Kind.Kind) {
		return admissionutils.ResponseSuccess()
	}

	kind := request.Kind.Kind
	logger.V(4).Info("received an admission request in validating webhook", "kind", kind)

	// timestamp at which this admission request got triggered
	requestTime := time.Now().Unix()
	policies := h.pCache.GetPolicies(policycache.ValidateEnforce, kind, request.Namespace)
	mutatePolicies := h.pCache.GetPolicies(policycache.Mutate, kind, request.Namespace)
	generatePolicies := h.pCache.GetPolicies(policycache.Generate, kind, request.Namespace)
	imageVerifyValidatePolicies := h.pCache.GetPolicies(policycache.VerifyImagesValidate, kind, request.Namespace)
	policies = append(policies, imageVerifyValidatePolicies...)

	if len(policies) == 0 && len(mutatePolicies) == 0 && len(generatePolicies) == 0 {
		logger.V(4).Info("no policies matched admission request", "kind", kind)
	}

	if len(generatePolicies) == 0 && request.Operation == admissionv1.Update {
		// handle generate source resource updates
		go h.handleUpdatesForGenerateRules(logger, request, []kyvernov1.PolicyInterface{})
	}

	logger.V(4).Info("processing policies for validate admission request",
		"kind", kind, "validate", len(policies), "mutate", len(mutatePolicies), "generate", len(generatePolicies))

	var roles, clusterRoles []string
	if containsRBACInfo(policies, generatePolicies) {
		var err error
		roles, clusterRoles, err = userinfo.GetRoleRef(h.rbLister, h.crbLister, request, h.configuration)
		if err != nil {
			return errorResponse(logger, err, "failed to fetch RBAC data")
		}
	}

	userRequestInfo := kyvernov1beta1.RequestInfo{
		Roles:             roles,
		ClusterRoles:      clusterRoles,
		AdmissionUserInfo: *request.UserInfo.DeepCopy(),
	}

	ctx, err := newVariablesContext(request, &userRequestInfo)
	if err != nil {
		return errorResponse(logger, err, "failed create policy rule context")
	}

	namespaceLabels := make(map[string]string)
	if request.Kind.Kind != "Namespace" && request.Namespace != "" {
		namespaceLabels = common.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, h.nsLister, logger)
	}

	newResource, oldResource, err := utils.ExtractResources(nil, request)
	if err != nil {
		return errorResponse(logger, err, "failed create parse resource")
	}

	if err := ctx.AddImageInfos(&newResource); err != nil {
		return errorResponse(logger, err, "failed add image information to policy rule context")
	}

	policyContext := &engine.PolicyContext{
		NewResource:         newResource,
		OldResource:         oldResource,
		AdmissionInfo:       userRequestInfo,
		ExcludeGroupRole:    h.configuration.GetExcludeGroupRole(),
		ExcludeResourceFunc: h.configuration.ToFilter,
		JSONContext:         ctx,
		Client:              h.client,
		AdmissionOperation:  true,
	}

	vh := &validationHandler{
		log:         logger,
		eventGen:    h.eventGen,
		prGenerator: h.prGenerator,
	}

	ok, msg, warnings := vh.handleValidation(h.metricsConfig, request, policies, policyContext, namespaceLabels, requestTime)
	if !ok {
		logger.Info("admission request denied")
		return admissionutils.ResponseFailure(msg)
	}

	h.auditHandler.Add(request.DeepCopy())
	go h.createUpdateRequests(logger, request, policyContext, generatePolicies, mutatePolicies, requestTime)

	if warnings != nil {
		return admissionutils.ResponseSuccessWithWarnings(warnings)
	}

	logger.V(4).Info("completed validating webhook")
	return admissionutils.ResponseSuccess()
}

func (h *handlers) Mutate(logger logr.Logger, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	if excludeKyvernoResources(request.Kind.Kind) {
		return admissionutils.ResponseSuccess()
	}
	if request.Operation == admissionv1.Delete {
		resource, err := utils.ConvertResource(request.OldObject.Raw, request.Kind.Group, request.Kind.Version, request.Kind.Kind, request.Namespace)
		if err == nil {
			h.prGenerator.Add(buildDeletionPrInfo(resource))
		} else {
			logger.Info(fmt.Sprintf("Converting oldObject failed: %v", err))
		}

		return admissionutils.ResponseSuccess()
	}
	kind := request.Kind.Kind
	logger.V(4).Info("received an admission request in mutating webhook", "kind", kind)
	requestTime := time.Now().Unix()
	mutatePolicies := h.pCache.GetPolicies(policycache.Mutate, kind, request.Namespace)
	verifyImagesPolicies := h.pCache.GetPolicies(policycache.VerifyImagesMutate, kind, request.Namespace)
	if len(mutatePolicies) == 0 && len(verifyImagesPolicies) == 0 {
		logger.V(4).Info("no policies matched mutate admission request", "kind", kind)
		return admissionutils.ResponseSuccess()
	}
	logger.V(4).Info("processing policies for mutate admission request", "kind", kind, "mutatePolicies", len(mutatePolicies), "verifyImagesPolicies", len(verifyImagesPolicies))
	addRoles := containsRBACInfo(mutatePolicies)
	policyContext, err := h.buildPolicyContext(request, addRoles)
	if err != nil {
		logger.Error(err, "failed to build policy context")
		return admissionutils.ResponseFailure(err.Error())
	}
	// update container images to a canonical form
	if err := enginectx.MutateResourceWithImageInfo(request.Object.Raw, policyContext.JSONContext); err != nil {
		logger.Error(err, "failed to patch images info to resource, policies that mutate images may be impacted")
	}
	mutatePatches, mutateWarnings, err := h.applyMutatePolicies(logger, request, policyContext, mutatePolicies, requestTime)
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

func (h *handlers) buildPolicyContext(request *admissionv1.AdmissionRequest, addRoles bool) (*engine.PolicyContext, error) {
	userRequestInfo := kyvernov1beta1.RequestInfo{
		AdmissionUserInfo: *request.UserInfo.DeepCopy(),
	}
	if addRoles {
		var err error
		userRequestInfo.Roles, userRequestInfo.ClusterRoles, err = userinfo.GetRoleRef(h.rbLister, h.crbLister, request, h.configuration)
		if err != nil {
			return nil, errors.Wrap(err, "failed to fetch RBAC information for request")
		}
	}
	ctx, err := newVariablesContext(request, &userRequestInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create policy rule context")
	}
	resource, err := convertResource(request, request.Object.Raw)
	if err != nil {
		return nil, err
	}
	if err := ctx.AddImageInfos(&resource); err != nil {
		return nil, errors.Wrap(err, "failed to add image information to the policy rule context")
	}
	policyContext := &engine.PolicyContext{
		NewResource:         resource,
		AdmissionInfo:       userRequestInfo,
		ExcludeGroupRole:    h.configuration.GetExcludeGroupRole(),
		ExcludeResourceFunc: h.configuration.ToFilter,
		JSONContext:         ctx,
		Client:              h.client,
		AdmissionOperation:  true,
	}
	if request.Operation == admissionv1.Update {
		policyContext.OldResource, err = convertResource(request, request.OldObject.Raw)
		if err != nil {
			return nil, err
		}
	}
	return policyContext, nil
}

func (h *handlers) applyMutatePolicies(logger logr.Logger, request *admissionv1.AdmissionRequest, policyContext *engine.PolicyContext, policies []kyvernov1.PolicyInterface, ts int64) ([]byte, []string, error) {
	mutatePatches, mutateEngineResponses, err := h.handleMutation(logger, request, policyContext, policies)
	if err != nil {
		return nil, nil, err
	}

	logger.V(6).Info("", "generated patches", string(mutatePatches))

	admissionReviewLatencyDuration := int64(time.Since(time.Unix(ts, 0)))
	go h.registerAdmissionReviewDurationMetricMutate(logger, string(request.Operation), mutateEngineResponses, admissionReviewLatencyDuration)
	go h.registerAdmissionRequestsMetricMutate(logger, string(request.Operation), mutateEngineResponses)

	warnings := getWarningMessages(mutateEngineResponses)
	return mutatePatches, warnings, nil
}

// handleMutation handles mutating webhook admission request
// return value: generated patches, triggered policies, engine responses correspdonding to the triggered policies
func (h *handlers) handleMutation(logger logr.Logger, request *admissionv1.AdmissionRequest, policyContext *engine.PolicyContext, policies []kyvernov1.PolicyInterface) ([]byte, []*response.EngineResponse, error) {
	if len(policies) == 0 {
		return nil, nil, nil
	}

	if isResourceDeleted(policyContext) && request.Operation == admissionv1.Update {
		return nil, nil, nil
	}

	var patches [][]byte
	var engineResponses []*response.EngineResponse

	for _, policy := range policies {
		spec := policy.GetSpec()
		if !spec.HasMutate() {
			continue
		}
		logger.V(3).Info("applying policy mutate rules", "policy", policy.GetName())
		policyContext.Policy = policy
		engineResponse, policyPatches, err := h.applyMutation(request, policyContext, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("mutation policy %s error: %v", policy.GetName(), err)
		}

		if len(policyPatches) > 0 {
			patches = append(patches, policyPatches...)
			rules := engineResponse.GetSuccessRules()
			if len(rules) != 0 {
				logger.Info("mutation rules from policy applied successfully", "policy", policy.GetName(), "rules", rules)
			}
		}

		policyContext.NewResource = engineResponse.PatchedResource
		engineResponses = append(engineResponses, engineResponse)

		// registering the kyverno_policy_results_total metric concurrently
		go h.registerPolicyResultsMetricMutation(logger, string(request.Operation), policy, *engineResponse)
		// registering the kyverno_policy_execution_duration_seconds metric concurrently
		go h.registerPolicyExecutionDurationMetricMutate(logger, string(request.Operation), policy, *engineResponse)
	}

	// generate annotations
	if annPatches := utils.GenerateAnnotationPatches(engineResponses, logger); annPatches != nil {
		patches = append(patches, annPatches...)
	}

	if !isResourceDeleted(policyContext) {
		events := generateEvents(engineResponses, false, logger)
		h.eventGen.Add(events...)
	}

	logMutationResponse(patches, engineResponses, logger)

	// patches holds all the successful patches, if no patch is created, it returns nil
	return jsonutils.JoinPatches(patches...), engineResponses, nil
}

func logMutationResponse(patches [][]byte, engineResponses []*response.EngineResponse, logger logr.Logger) {
	if len(patches) != 0 {
		logger.V(4).Info("created patches", "count", len(patches))
	}

	// if any of the policies fails, print out the error
	if !engineutils.IsResponseSuccessful(engineResponses) {
		logger.Error(errors.New(getErrorMsg(engineResponses)), "failed to apply mutation rules on the resource, reporting policy violation")
	}
}

func (h *handlers) applyMutation(request *admissionv1.AdmissionRequest, policyContext *engine.PolicyContext, logger logr.Logger) (*response.EngineResponse, [][]byte, error) {
	if request.Kind.Kind != "Namespace" && request.Namespace != "" {
		policyContext.NamespaceLabels = common.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, h.nsLister, logger)
	}

	engineResponse := engine.Mutate(policyContext)
	policyPatches := engineResponse.GetPatches()

	if !engineResponse.IsSuccessful() && len(engineResponse.GetFailedRules()) > 0 {
		return nil, nil, fmt.Errorf("failed to apply policy %s rules %v", policyContext.Policy.GetName(), engineResponse.GetFailedRules())
	}

	if engineResponse.PatchedResource.GetKind() != "*" {
		err := h.openAPIController.ValidateResource(*engineResponse.PatchedResource.DeepCopy(), engineResponse.PatchedResource.GetAPIVersion(), engineResponse.PatchedResource.GetKind())
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to validate resource mutated by policy %s", policyContext.Policy.GetName())
		}
	}

	return engineResponse, policyPatches, nil
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
	blocked := blockRequest(engineResponses, failurePolicy, logger)
	if !isResourceDeleted(policyContext) {
		events := generateEvents(engineResponses, blocked, logger)
		h.eventGen.Add(events...)
	}

	if blocked {
		logger.V(4).Info("admission request blocked")
		return false, getBlockedMessages(engineResponses), nil, nil
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

	warnings := getWarningMessages(engineResponses)
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
	resource, err := engineutils2.ConvertToUnstructured(request.OldObject.Raw)
	if err != nil {
		logger.Error(err, "failed to convert object resource to unstructured format")
	}

	resLabels := resource.GetLabels()
	if resLabels["app.kubernetes.io/managed-by"] == "kyverno" && request.Operation == admissionv1.Delete {
		urName := resLabels["policy.kyverno.io/gr-name"]
		ur, err := h.urLister.Get(urName)
		if err != nil {
			logger.Error(err, "failed to get update request", "name", urName)
			return
		}

		if ur.Spec.Type == kyvernov1beta1.Mutate {
			return
		}
		h.updateAnnotationInUR(ur, logger)
	}
}
