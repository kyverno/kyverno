package mpol

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/breaker"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/mutate/patch"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/toggle"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	webhookgenerate "github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type handler struct {
	context                      libs.Context
	engine                       mpolengine.Engine
	kyvernoClient                versioned.Interface
	reportsConfig                reportutils.ReportingConfiguration
	urGenerator                  webhookgenerate.Generator
	backgroundServiceAccountName string
	eventGen                     event.Interface
}

func New(
	context libs.Context,
	engine mpolengine.Engine,
	kyvernoClient versioned.Interface,
	reportsConfig reportutils.ReportingConfiguration,
	urGenerator webhookgenerate.Generator,
	backgroundServiceAccountName string,
	eventGen event.Interface,
) *handler {
	return &handler{
		context:                      context,
		engine:                       engine,
		kyvernoClient:                kyvernoClient,
		reportsConfig:                reportsConfig,
		urGenerator:                  urGenerator,
		backgroundServiceAccountName: backgroundServiceAccountName,
		eventGen:                     eventGen,
	}
}

func (h *handler) MutateClustered(ctx context.Context, logger logr.Logger, admissionRequest handlers.AdmissionRequest, _ string, _ time.Time) handlers.AdmissionResponse {
	policies := policyNamesFromContext(ctx)
	return h.mutate(ctx, logger, admissionRequest, policies, mpolengine.And(mpolengine.MatchNames(policies...), mpolengine.ClusteredPolicy()))
}

func (h *handler) MutateNamespaced(ctx context.Context, logger logr.Logger, admissionRequest handlers.AdmissionRequest, _ string, _ time.Time) handlers.AdmissionResponse {
	if admissionRequest.Namespace == "" {
		return admissionutils.ResponseSuccess(admissionRequest.UID)
	}
	policies := policyNamesFromContext(ctx)
	return h.mutate(ctx, logger, admissionRequest, policies, mpolengine.And(mpolengine.MatchNames(policies...), mpolengine.NamespacedPolicy(admissionRequest.Namespace)))
}

func (h *handler) mutate(ctx context.Context, logger logr.Logger, admissionRequest handlers.AdmissionRequest, policies []string, predicate mpolengine.Predicate) handlers.AdmissionResponse {
	if h.backgroundServiceAccountName == admissionRequest.UserInfo.Username {
		return admissionutils.ResponseSuccess(admissionRequest.UID)
	}
	if len(policies) == 0 {
		return admissionutils.ResponseSuccess(admissionRequest.UID)
	}

	request := celengine.RequestFromAdmission(h.context, admissionRequest.AdmissionRequest)
	response, err := h.engine.Handle(ctx, request, predicate)
	if err != nil {
		logger.Error(err, "failed to handle mutating policy request")
		return admissionutils.Response(admissionRequest.UID, err)
	}

	go func() {
		if err := h.audit(context.TODO(), response, request); err != nil {
			logger.Error(err, "failed to create reports")
		}
	}()

	// Skip mutate-existing UpdateRequest creation for dry-run requests
	// to honor the SideEffects: NoneOnDryRun contract.
	if !admissionutils.IsDryRun(admissionRequest.AdmissionRequest) {
		go func() {
			mpols := h.engine.MatchedMutateExistingPolicies(ctx, request)
			for _, p := range mpols {
				logger.V(4).Info("creating a UR for mpol", "name", p)
				if err := h.urGenerator.Apply(ctx, kyvernov2.UpdateRequestSpec{
					Type:   kyvernov2.CELMutate,
					Policy: p,
					Context: kyvernov2.UpdateRequestSpecContext{
						UserRequestInfo: kyvernov2.RequestInfo{
							Roles:             admissionRequest.Roles,
							ClusterRoles:      admissionRequest.ClusterRoles,
							AdmissionUserInfo: *admissionRequest.UserInfo.DeepCopy(),
						},
						AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{
							AdmissionRequest: &admissionRequest.AdmissionRequest,
							Operation:        admissionRequest.Operation,
						},
					},
				}); err != nil {
					logger.Error(err, "failed to create update request for mutate existing policy", "policy", p)
				}
			}
		}()
	}

	resp, err := h.admissionResponse(request, response)
	if err != nil {
		logger.Error(err, "mutation failures")
		return admissionutils.Response(admissionRequest.UID, err)
	}
	return resp
}

func (h *handler) audit(ctx context.Context, response mpolengine.EngineResponse, request celengine.EngineRequest) error {
	allEngineResponses := make([]engineapi.EngineResponse, 0, len(response.Policies))
	reportableEngineResponses := make([]engineapi.EngineResponse, 0, len(response.Policies))
	for _, r := range response.Policies {
		engineResponse := engineapi.EngineResponse{
			Resource: *response.Resource,
			PolicyResponse: engineapi.PolicyResponse{
				Rules: r.Rules,
			},
		}
		engineResponse = engineResponse.WithPolicy(engineapi.NewMutatingPolicyFromLike(r.Policy))
		allEngineResponses = append(allEngineResponses, engineResponse)
		if reportutils.IsPolicyReportable(r.Policy) {
			reportableEngineResponses = append(reportableEngineResponses, engineResponse)
		}
	}

	for _, response := range allEngineResponses {
		events := webhookutils.GenerateEvents([]engineapi.EngineResponse{response}, false)
		h.eventGen.Add(events...)
	}

	if !h.needsReports(request) {
		return nil
	}

	report := reportutils.BuildMutationReport(*response.Resource, request.Request, reportableEngineResponses...)
	if len(report.GetResults()) > 0 {
		err := breaker.GetReportsBreaker().Do(ctx, func(ctx context.Context) error {
			_, err := reportutils.CreateEphemeralReport(ctx, report, h.kyvernoClient)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *handler) needsReports(request celengine.EngineRequest) bool {
	if admissionutils.IsDryRun(request.Request) {
		return false
	}
	if !h.reportsConfig.MutateReportsEnabled() {
		return false
	}
	if request.Request.Operation == admissionv1.Delete {
		return false
	}
	if !reportutils.IsGvkSupported(schema.GroupVersionKind(request.Request.Kind)) {
		return false
	}

	return true
}

func (h *handler) admissionResponse(request celengine.EngineRequest, response mpolengine.EngineResponse) (handlers.AdmissionResponse, error) {
	if len(response.Policies) == 0 {
		return admissionutils.ResponseSuccess(request.Request.UID), nil
	}

	var warnings []string
	var mutationErrors []string

	for _, policy := range response.Policies {
		failurePolicy := policy.Policy.GetFailurePolicy(toggle.FromContext(context.TODO()).ForceFailurePolicyIgnore())
		for _, rule := range policy.Rules {
			switch rule.Status() {
			case engineapi.RuleStatusError:
				// Only block the request if failurePolicy is Fail
				// If failurePolicy is Ignore, the error is logged but the request proceeds
				if failurePolicy == admissionregistrationv1.Fail {
					mutationErrors = append(mutationErrors, fmt.Sprintf("Policy %s: %s", policy.Policy.GetName(), rule.Message()))
				}
			case engineapi.RuleStatusWarn:
				warnings = append(warnings, rule.Message())
			}
		}
	}

	if len(mutationErrors) > 0 {
		err := fmt.Errorf("Resource: %s/%s, Kind: %s, Errors: %v",
			request.Request.Namespace, request.Request.Name, request.Request.Kind.Kind, mutationErrors)
		return admissionutils.Response(request.Request.UID, err), err
	}

	if response.PatchedResource != nil {
		patches := jsonutils.JoinPatches(patch.ConvertPatches(response.GetPatches()...)...)
		return admissionutils.MutationResponse(request.Request.UID, patches, warnings...), nil
	}

	return admissionutils.MutationResponse(request.Request.UID, nil, warnings...), nil
}

func policyNamesFromContext(ctx context.Context) []string {
	params := httprouter.ParamsFromContext(ctx)
	if params == nil {
		return nil
	}
	raw := strings.Trim(params.ByName("policies"), "/")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "/")
}
