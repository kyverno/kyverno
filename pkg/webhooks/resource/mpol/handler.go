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
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	webhookgenerate "github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type handler struct {
	context                      libs.Context
	engine                       mpolengine.Engine
	kyvernoClient                versioned.Interface
	reportsConfig                reportutils.ReportingConfiguration
	urGenerator                  webhookgenerate.Generator
	backgroundServiceAccountName string
}

func New(
	context libs.Context,
	engine mpolengine.Engine,
	kyvernoClient versioned.Interface,
	reportsConfig reportutils.ReportingConfiguration,
	urGenerator webhookgenerate.Generator,
	backgroundServiceAccountName string,
) *handler {
	return &handler{
		context:                      context,
		engine:                       engine,
		kyvernoClient:                kyvernoClient,
		reportsConfig:                reportsConfig,
		urGenerator:                  urGenerator,
		backgroundServiceAccountName: backgroundServiceAccountName,
	}
}

func (h *handler) Mutate(ctx context.Context, logger logr.Logger, admissionRequest handlers.AdmissionRequest, _ string, _ time.Time) handlers.AdmissionResponse {
	var policies []string

	if h.backgroundServiceAccountName == admissionRequest.UserInfo.Username {
		return admissionutils.ResponseSuccess(admissionRequest.UID)
	}

	if params := httprouter.ParamsFromContext(ctx); params != nil {
		if params := strings.Split(strings.TrimLeft(params.ByName("policies"), "/"), "/"); len(params) != 0 {
			policies = params
		}
	}

	if len(policies) == 0 {
		return admissionutils.ResponseSuccess(admissionRequest.UID)
	}

	request := celengine.RequestFromAdmission(h.context, admissionRequest.AdmissionRequest)
	response, err := h.engine.Handle(ctx, request, mpolengine.MatchNames(policies...))
	if err != nil {
		logger.Error(err, "failed to handle mutating policy request")
		return admissionutils.ResponseSuccess(admissionRequest.UID)
	}

	go func() {
		if err := h.createReports(context.TODO(), response, request); err != nil {
			logger.Error(err, "failed to create reports")
		}
	}()

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

	resp, err := h.admissionResponse(request, response)
	if err != nil {
		logger.Error(err, "mutation failures")
		return admissionutils.ResponseSuccess(admissionRequest.UID)
	}
	return resp
}

func (h *handler) createReports(ctx context.Context, response mpolengine.EngineResponse, request celengine.EngineRequest) error {
	if !h.needsReports(request) {
		return nil
	}

	engineResponses := make([]engineapi.EngineResponse, 0, len(response.Policies))
	for _, res := range response.Policies {
		engineResponses = append(engineResponses, engineapi.EngineResponse{
			Resource: *response.Resource,
			PolicyResponse: engineapi.PolicyResponse{
				Rules: res.Rules,
			},
		}.WithPolicy(engineapi.NewMutatingPolicy(res.Policy)))
	}

	report := reportutils.BuildMutationReport(*response.Resource, request.Request, engineResponses...)
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
		for _, rule := range policy.Rules {
			switch rule.Status() {
			case engineapi.RuleStatusError:
				mutationErrors = append(mutationErrors, fmt.Sprintf("Policy %s: %s", policy.Policy.Name, rule.Message()))
			case engineapi.RuleStatusWarn:
				warnings = append(warnings, rule.Message())
			}
		}
	}

	if len(mutationErrors) > 0 {
		return admissionutils.ResponseSuccess(request.Request.UID),
			fmt.Errorf("Resource: %s/%s, Kind: %s, Errors: %v\n",
				request.Request.Namespace, request.Request.Name, request.Request.Kind.Kind, mutationErrors)
	}

	if response.PatchedResource != nil {
		patches := jsonutils.JoinPatches(patch.ConvertPatches(response.GetPatches()...)...)
		return admissionutils.MutationResponse(request.Request.UID, patches, warnings...), nil
	}

	return admissionutils.MutationResponse(request.Request.UID, nil, warnings...), nil
}
