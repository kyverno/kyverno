package webhooks

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/engine"
	enginectx "github.com/kyverno/kyverno/pkg/engine/context"
	policyvalidate "github.com/kyverno/kyverno/pkg/policy"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/policymutation"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/kyverno/pkg/userinfo"
	"github.com/kyverno/kyverno/pkg/utils"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	admissionv1 "k8s.io/api/admission/v1"
)

// TODO: use admission review sub resource ?
func isStatusUpdate(old, new kyverno.PolicyInterface) bool {
	if !reflect.DeepEqual(old.GetAnnotations(), new.GetAnnotations()) {
		return false
	}
	if !reflect.DeepEqual(old.GetLabels(), new.GetLabels()) {
		return false
	}
	if !reflect.DeepEqual(old.GetSpec(), new.GetSpec()) {
		return false
	}
	return true
}

func errorResponse(logger logr.Logger, err error, message string) *admissionv1.AdmissionResponse {
	logger.Error(err, message)
	return admissionutils.ResponseFailure(false, message+": "+err.Error())
}

func setupLogger(logger logr.Logger, name string, request *admissionv1.AdmissionRequest) logr.Logger {
	return logger.WithName(name).WithValues(
		"uid", request.UID,
		"kind", request.Kind,
		"namespace", request.Namespace,
		"name", request.Name,
		"operation", request.Operation,
		"gvk", request.Kind.String(),
	)
}

func (ws *WebhookServer) admissionHandler(filter bool, inner handlers.AdmissionHandler) http.HandlerFunc {
	if filter {
		inner = handlers.Filter(ws.configHandler, inner)
	}
	return handlers.Monitor(ws.webhookMonitor, handlers.Admission(ws.log.WithName("handlerFunc"), inner))
}

func (ws *WebhookServer) policyMutation(request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	if toggle.AutogenInternals() {
		return admissionutils.Response(true)
	}
	logger := setupLogger(ws.log, "PolicyMutationWebhook", request)
	policy, oldPolicy, err := admissionutils.GetPolicies(request)
	if err != nil {
		logger.Error(err, "failed to unmarshal policies from admission request")
		return admissionutils.ResponseWithMessage(true, fmt.Sprintf("failed to default value, check kyverno controller logs for details: %v", err))
	}

	if oldPolicy != nil && isStatusUpdate(oldPolicy, policy) {
		logger.V(4).Info("skip policy mutation on status update")
		return admissionutils.Response(true)
	}

	startTime := time.Now()
	logger.V(3).Info("start policy change mutation")
	defer logger.V(3).Info("finished policy change mutation", "time", time.Since(startTime).String())

	// Generate JSON Patches for defaults
	if patches, updateMsgs := policymutation.GenerateJSONPatchesForDefaults(policy, logger); len(patches) != 0 {
		return admissionutils.ResponseWithMessageAndPatch(true, strings.Join(updateMsgs, "'"), patches)
	}

	return admissionutils.Response(true)
}

//policyValidation performs the validation check on policy resource
func (ws *WebhookServer) policyValidation(request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	logger := setupLogger(ws.log, "PolicyValidationWebhook", request)
	policy, oldPolicy, err := admissionutils.GetPolicies(request)
	if err != nil {
		logger.Error(err, "failed to unmarshal policies from admission request")
		return admissionutils.ResponseWithMessage(true, fmt.Sprintf("failed to validate policy, check kyverno controller logs for details: %v", err))
	}

	if oldPolicy != nil && isStatusUpdate(oldPolicy, policy) {
		logger.V(4).Info("skip policy validation on status update")
		return admissionutils.Response(true)
	}

	startTime := time.Now()
	logger.V(3).Info("start policy change validation")
	defer logger.V(3).Info("finished policy change validation", "time", time.Since(startTime).String())

	response, err := policyvalidate.Validate(policy, ws.client, false, ws.openAPIController)
	if err != nil {
		logger.Error(err, "policy validation errors")
		return admissionutils.ResponseWithMessage(false, err.Error())
	}

	if response != nil && len(response.Warnings) != 0 {
		return response
	}

	return admissionutils.Response(true)
}

// resourceMutation mutates resource
func (ws *WebhookServer) resourceMutation(request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	logger := setupLogger(ws.log, "ResourceMutationWebhook", request)
	if excludeKyvernoResources(request.Kind.Kind) {
		return admissionutils.ResponseSuccess(true, "")
	}

	if request.Operation == admissionv1.Delete {
		resource, err := utils.ConvertResource(request.OldObject.Raw, request.Kind.Group, request.Kind.Version, request.Kind.Kind, request.Namespace)
		if err == nil {
			ws.prGenerator.Add(buildDeletionPrInfo(resource))
		} else {
			logger.Info(fmt.Sprintf("Converting oldObject failed: %v", err))
		}

		return admissionutils.ResponseSuccess(true, "")
	}

	kind := request.Kind.Kind
	logger.V(4).Info("received an admission request in mutating webhook", "kind", kind)

	requestTime := time.Now().Unix()
	mutatePolicies := ws.pCache.GetPolicies(policycache.Mutate, kind, request.Namespace)
	verifyImagesPolicies := ws.pCache.GetPolicies(policycache.VerifyImagesMutate, kind, request.Namespace)

	if len(mutatePolicies) == 0 && len(verifyImagesPolicies) == 0 {
		logger.V(4).Info("no policies matched mutate admission request", "kind", kind)
		return admissionutils.ResponseSuccess(true, "")
	}

	logger.V(4).Info("processing policies for mutate admission request", "kind", kind,
		"mutatePolicies", len(mutatePolicies), "verifyImagesPolicies", len(verifyImagesPolicies))

	addRoles := containsRBACInfo(mutatePolicies)
	policyContext, err := ws.buildPolicyContext(request, addRoles)
	if err != nil {
		logger.Error(err, "failed to build policy context")
		return admissionutils.ResponseFailure(false, err.Error())
	}

	// update container images to a canonical form
	if err := enginectx.MutateResourceWithImageInfo(request.Object.Raw, policyContext.JSONContext); err != nil {
		ws.log.Error(err, "failed to patch images info to resource, policies that mutate images may be impacted")
	}

	mutatePatches := ws.applyMutatePolicies(request, policyContext, mutatePolicies, requestTime, logger)
	newRequest := patchRequest(mutatePatches, request, logger)
	imagePatches, err := ws.applyImageVerifyPolicies(newRequest, policyContext, verifyImagesPolicies, logger)
	if err != nil {
		logger.Error(err, "image verification failed")
		return admissionutils.ResponseFailure(false, err.Error())
	}

	var patches = append(mutatePatches, imagePatches...)

	return admissionutils.ResponseSuccessWithPatch(true, "", patches)
}

func (ws *WebhookServer) resourceValidation(request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
	logger := setupLogger(ws.log, "ResourceValidationWebhook", request)
	if request.Operation == admissionv1.Delete {
		ws.handleDelete(request)
	}

	if excludeKyvernoResources(request.Kind.Kind) {
		return admissionutils.ResponseSuccess(true, "")
	}

	kind := request.Kind.Kind
	logger.V(4).Info("received an admission request in validating webhook", "kind", kind)

	// timestamp at which this admission request got triggered
	requestTime := time.Now().Unix()
	policies := ws.pCache.GetPolicies(policycache.ValidateEnforce, kind, request.Namespace)
	mutatePolicies := ws.pCache.GetPolicies(policycache.Mutate, kind, request.Namespace)
	generatePolicies := ws.pCache.GetPolicies(policycache.Generate, kind, request.Namespace)
	imageVerifyValidatePolicies := ws.pCache.GetPolicies(policycache.VerifyImagesValidate, kind, request.Namespace)
	policies = append(policies, imageVerifyValidatePolicies...)

	if len(policies) == 0 && len(mutatePolicies) == 0 && len(generatePolicies) == 0 {
		logger.V(4).Info("no policies matched admission request", "kind", kind)
	}

	if len(generatePolicies) == 0 && request.Operation == admissionv1.Update {
		// handle generate source resource updates
		go ws.handleUpdatesForGenerateRules(request, []kyverno.PolicyInterface{})
	}

	logger.V(4).Info("processing policies for validate admission request",
		"kind", kind, "validate", len(policies), "mutate", len(mutatePolicies), "generate", len(generatePolicies))

	var roles, clusterRoles []string
	if containsRBACInfo(policies, generatePolicies) {
		var err error
		roles, clusterRoles, err = userinfo.GetRoleRef(ws.rbLister, ws.crbLister, request, ws.configHandler)
		if err != nil {
			return errorResponse(logger, err, "failed to fetch RBAC data")
		}
	}

	userRequestInfo := urkyverno.RequestInfo{
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
		namespaceLabels = common.GetNamespaceSelectorsFromNamespaceLister(request.Kind.Kind, request.Namespace, ws.nsLister, logger)
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
		ExcludeGroupRole:    ws.configHandler.GetExcludeGroupRole(),
		ExcludeResourceFunc: ws.configHandler.ToFilter,
		JSONContext:         ctx,
		Client:              ws.client,
		AdmissionOperation:  true,
	}

	vh := &validationHandler{
		log:         ws.log,
		eventGen:    ws.eventGen,
		prGenerator: ws.prGenerator,
	}

	ok, msg := vh.handleValidation(ws.promConfig, request, policies, policyContext, namespaceLabels, requestTime)
	if !ok {
		logger.Info("admission request denied")
		return admissionutils.ResponseFailure(false, msg)
	}

	// push admission request to audit handler, this won't block the admission request
	ws.auditHandler.Add(request.DeepCopy())

	go ws.createUpdateRequests(request, policyContext, generatePolicies, mutatePolicies, requestTime, logger)

	return admissionutils.ResponseSuccess(true, "")
}
