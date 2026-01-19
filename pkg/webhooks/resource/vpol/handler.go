package vpol

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	"github.com/kyverno/kyverno/pkg/breaker"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	vpolengine "github.com/kyverno/kyverno/pkg/cel/policies/vpol/engine"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	event "github.com/kyverno/kyverno/pkg/event"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/kyverno/kyverno/pkg/webhooks/resource/validation"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	"go.uber.org/multierr"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

type handler struct {
	context          libs.Context
	engine           vpolengine.Engine
	kyvernoClient    versioned.Interface
	admissionReports bool
	eventGen         event.Interface
}

func New(
	engine vpolengine.Engine,
	context libs.Context,
	kyvernoClient versioned.Interface,
	admissionReports bool,
	eventGen event.Interface,
) *handler {
	return &handler{
		context:          context,
		engine:           engine,
		kyvernoClient:    kyvernoClient,
		admissionReports: admissionReports,
		eventGen:         eventGen,
	}
}

func (h *handler) ValidateClustered(ctx context.Context, logger logr.Logger, admissionRequest handlers.AdmissionRequest, _ string, _ time.Time) handlers.AdmissionResponse {
	var policies []string
	if params := httprouter.ParamsFromContext(ctx); params != nil {
		if params := strings.Split(strings.TrimLeft(params.ByName("policies"), "/"), "/"); len(params) != 0 {
			policies = params
		}
	}
	predicate := vpolengine.And(vpolengine.MatchNames(policies...), vpolengine.ClusteredPolicy())
	return h.validate(ctx, logger, admissionRequest, predicate)
}

func (h *handler) ValidateNamespaced(ctx context.Context, logger logr.Logger, admissionRequest handlers.AdmissionRequest, _ string, _ time.Time) handlers.AdmissionResponse {
	var policies []string
	if params := httprouter.ParamsFromContext(ctx); params != nil {
		if params := strings.Split(strings.TrimLeft(params.ByName("policies"), "/"), "/"); len(params) != 0 {
			policies = params
		}
	}
	predicate := vpolengine.And(vpolengine.MatchNames(policies...), vpolengine.NamespacedPolicy(admissionRequest.Namespace))
	return h.validate(ctx, logger, admissionRequest, predicate)
}

func (h *handler) validate(ctx context.Context, logger logr.Logger, admissionRequest handlers.AdmissionRequest, predicate vpolengine.Predicate) handlers.AdmissionResponse {
	request := celengine.RequestFromAdmission(h.context, admissionRequest.AdmissionRequest)
	response, err := h.engine.Handle(ctx, request, predicate)
	if err != nil {
		return admissionutils.Response(admissionRequest.UID, err)
	}
	var group wait.Group
	defer group.Wait()
	group.Start(func() {
		h.audit(ctx, logger, admissionRequest, request, response)
	})
	return h.admissionResponse(request, response)
}

func (h *handler) audit(ctx context.Context, logger logr.Logger, admissionRequest handlers.AdmissionRequest, request vpolengine.EngineRequest, response vpolengine.EngineResponse) {
	blocked := false
	for _, p := range response.Policies {
		if p.Actions.Has(admissionregistrationv1.Deny) {
			blocked = true
			break
		}
	}

	allEngineResponses := make([]engineapi.EngineResponse, 0, len(response.Policies))
	reportableEngineResponses := make([]engineapi.EngineResponse, 0, len(response.Policies))
	for _, r := range response.Policies {
		engineResponse := engineapi.EngineResponse{
			Resource: *response.Resource,
			PolicyResponse: engineapi.PolicyResponse{
				Rules: r.Rules,
			},
		}
		engineResponse = engineResponse.WithPolicy(engineapi.NewValidatingPolicyFromLike(r.Policy))
		allEngineResponses = append(allEngineResponses, engineResponse)
		if reportutils.IsPolicyReportable(r.Policy) {
			reportableEngineResponses = append(reportableEngineResponses, engineResponse)
		}
	}

	if !blocked && validation.NeedsReports(admissionRequest, *response.Resource, h.admissionReports) {
		err := h.admissionReport(ctx, request, response, reportableEngineResponses)
		if err != nil {
			logger.Error(err, "failed to create report")
		}
	}

	h.admissionEvent(ctx, allEngineResponses, blocked)
}

func (h *handler) admissionReport(ctx context.Context, request vpolengine.EngineRequest, response vpolengine.EngineResponse, responses []engineapi.EngineResponse) error {
	report := reportutils.BuildAdmissionReport(*response.Resource, request.AdmissionRequest(), responses...)
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

func (h *handler) admissionEvent(_ context.Context, responses []engineapi.EngineResponse, blocked bool) {
	for _, response := range responses {
		events := webhookutils.GenerateEvents([]engineapi.EngineResponse{response}, blocked)
		h.eventGen.Add(events...)
	}
}

func (h *handler) admissionResponse(request vpolengine.EngineRequest, response vpolengine.EngineResponse) handlers.AdmissionResponse {
	var errs []error
	var warnings []string
	for _, policy := range response.Policies {
		if policy.Actions.Has(admissionregistrationv1.Deny) {
			for _, rule := range policy.Rules {
				switch rule.Status() {
				case engineapi.RuleStatusFail:
					errs = append(errs, fmt.Errorf("Policy %s failed: %s", policy.Policy.GetName(), rule.Message()))
				case engineapi.RuleStatusError:
					errs = append(errs, fmt.Errorf("Policy %s error: %s", policy.Policy.GetName(), rule.Message()))
				}
			}
		}
		if policy.Actions.Has(admissionregistrationv1.Warn) {
			for _, rule := range policy.Rules {
				switch rule.Status() {
				case engineapi.RuleStatusFail:
					warnings = append(warnings, fmt.Sprintf("Policy %s failed: %s", policy.Policy.GetName(), rule.Message()))
				case engineapi.RuleStatusError:
					warnings = append(warnings, fmt.Sprintf("Policy %s error: %s", policy.Policy.GetName(), rule.Message()))
				}
			}
		}
	}
	return admissionutils.Response(request.AdmissionRequest().UID, multierr.Combine(errs...), warnings...)
}
