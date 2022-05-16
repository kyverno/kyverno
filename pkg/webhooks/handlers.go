package webhooks

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/engine"
	enginectx "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/policycache"
	"github.com/kyverno/kyverno/pkg/userinfo"
	"github.com/kyverno/kyverno/pkg/utils"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	admissionv1 "k8s.io/api/admission/v1"
)

func errorResponse(logger logr.Logger, err error, message string) *admissionv1.AdmissionResponse {
	logger.Error(err, message)
	return admissionutils.ResponseFailure(false, message+": "+err.Error())
}

func (ws *WebhookServer) admissionHandler(logger logr.Logger, filter bool, inner handlers.AdmissionHandler) http.HandlerFunc {
	if filter {
		inner = handlers.Filter(ws.configuration, inner)
	}
	return handlers.Monitor(ws.webhookMonitor, handlers.Admission(logger, inner))
}

// resourceMutation mutates resource
func (ws *WebhookServer) resourceMutation(logger logr.Logger, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
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

	patches := append(mutatePatches, imagePatches...)

	return admissionutils.ResponseSuccessWithPatch(true, "", patches)
}

func (ws *WebhookServer) resourceValidation(logger logr.Logger, request *admissionv1.AdmissionRequest) *admissionv1.AdmissionResponse {
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
		roles, clusterRoles, err = userinfo.GetRoleRef(ws.rbLister, ws.crbLister, request, ws.configuration)
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
		ExcludeGroupRole:    ws.configuration.GetExcludeGroupRole(),
		ExcludeResourceFunc: ws.configuration.ToFilter,
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
