package ivpol

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	"github.com/kyverno/kyverno/pkg/breaker"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	ivpolengine "github.com/kyverno/kyverno/pkg/cel/policies/ivpol/engine"
	"github.com/kyverno/kyverno/pkg/cel/trace"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/mutate/patch"
	"github.com/kyverno/kyverno/pkg/event"
	eval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
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
	engine           ivpolengine.Engine
	kyvernoClient    versioned.Interface
	admissionReports bool
	eventGen         event.Interface
}

func New(
	engine ivpolengine.Engine,
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

// policyNames returns the policy names carried in the webhook URL path.
func policyNames(ctx context.Context) []string {
	if params := httprouter.ParamsFromContext(ctx); params != nil {
		if params := strings.Split(strings.TrimLeft(params.ByName("policies"), "/"), "/"); len(params) != 0 {
			return params
		}
	}
	return nil
}

// MutateClustered serves the /ivpol/mutate route, so it only evaluates cluster scoped policies.
func (h *handler) MutateClustered(ctx context.Context, logger logr.Logger, admissionRequest handlers.AdmissionRequest, _ string, _ time.Time) handlers.AdmissionResponse {
	predicate := ivpolengine.And(ivpolengine.MatchNames(policyNames(ctx)...), ivpolengine.ClusteredPolicy())
	return h.mutate(ctx, admissionRequest, predicate)
}

// MutateNamespaced serves the /nivpol/mutate route. The provider watches cluster scoped and
// namespaced image validating policies as one set, and policy names are not unique across
// namespaces, so the request's namespace is matched as well to avoid evaluating a same named
// policy from another namespace (or a cluster scoped one).
func (h *handler) MutateNamespaced(ctx context.Context, logger logr.Logger, admissionRequest handlers.AdmissionRequest, _ string, _ time.Time) handlers.AdmissionResponse {
	predicate := ivpolengine.And(ivpolengine.MatchNames(policyNames(ctx)...), ivpolengine.NamespacedPolicy(admissionRequest.Namespace))
	return h.mutate(ctx, admissionRequest, predicate)
}

func (h *handler) mutate(ctx context.Context, admissionRequest handlers.AdmissionRequest, predicate ivpolengine.Predicate) handlers.AdmissionResponse {
	request := celengine.RequestFromAdmission(h.context, admissionRequest.AdmissionRequest)
	response, patches, err := h.engine.HandleMutating(ctx, request, predicate)
	if err != nil {
		return admissionutils.Response(admissionRequest.UID, err)
	}
	rawPatches := jsonutils.JoinPatches(patch.ConvertPatches(patches...)...)
	admissionResponse := h.mutationResponse(request, response, rawPatches)
	h.mutationEvents(ctx, response, !admissionResponse.Allowed)
	return admissionResponse
}

// ValidateClustered serves the /ivpol/validate route, so it only evaluates cluster scoped policies.
func (h *handler) ValidateClustered(ctx context.Context, logger logr.Logger, admissionRequest handlers.AdmissionRequest, _ string, _ time.Time) handlers.AdmissionResponse {
	predicate := ivpolengine.And(ivpolengine.MatchNames(policyNames(ctx)...), ivpolengine.ClusteredPolicy())
	return h.validate(ctx, logger, admissionRequest, predicate)
}

// ValidateNamespaced serves the /nivpol/validate route, scoping evaluation to policies in the
// request's namespace (see MutateNamespaced).
func (h *handler) ValidateNamespaced(ctx context.Context, logger logr.Logger, admissionRequest handlers.AdmissionRequest, _ string, _ time.Time) handlers.AdmissionResponse {
	predicate := ivpolengine.And(ivpolengine.MatchNames(policyNames(ctx)...), ivpolengine.NamespacedPolicy(admissionRequest.Namespace))
	return h.validate(ctx, logger, admissionRequest, predicate)
}

func (h *handler) validate(ctx context.Context, logger logr.Logger, admissionRequest handlers.AdmissionRequest, predicate ivpolengine.Predicate) handlers.AdmissionResponse {
	request := celengine.RequestFromAdmission(h.context, admissionRequest.AdmissionRequest)
	response, err := h.engine.HandleValidating(ctx, request, predicate)
	if err != nil {
		return admissionutils.Response(admissionRequest.UID, err)
	}
	logDecisionTraces(logger, response)
	response = withoutTraceOnlyPolicies(response)
	var group wait.Group
	defer group.Wait()
	group.Start(func() {
		h.audit(ctx, logger, admissionRequest, request, response)
	})
	return h.validationResponse(request, response)
}

// logDecisionTraces writes the decision trace of every policy that has one to the log. A policy
// only has one when the admission controller compiles policies with tracing on, so this does
// nothing otherwise. The traces are written in their own goroutine, outside the audit group the
// handler waits for, so that logging them never holds up the admission response; they are not
// part of that response either.
func logDecisionTraces(logger logr.Logger, response eval.ImageVerifyEngineResponse) {
	var traces []*trace.Decision
	for _, p := range response.Policies {
		if p.Trace != nil {
			traces = append(traces, p.Trace)
		}
	}
	if len(traces) == 0 {
		return
	}
	go webhookutils.LogDecisionTraces(logger, traces...)
}

// withoutTraceOnlyPolicies drops the policies that are in the response only for their trace. With
// tracing on, the engine also returns a policy that did not apply to the resource, or that a match
// condition skipped, with an empty Result so that its trace is not lost (see
// eval.ImageVerifyPolicyResponse.Trace); without tracing it returns no entry for them at all. The
// admission response, the report and the events are built from the policies left, so they are
// exactly what they are without tracing: a Deny policy that did not apply must not mark the
// request as blocked, nor an empty result reach a report or an event. With tracing off nothing is
// dropped and the response is returned as it is.
func withoutTraceOnlyPolicies(response eval.ImageVerifyEngineResponse) eval.ImageVerifyEngineResponse {
	if !slices.ContainsFunc(response.Policies, eval.ImageVerifyPolicyResponse.TraceOnly) {
		return response
	}
	// a new slice: the traces logged above still belong to the engine's response
	policies := make([]eval.ImageVerifyPolicyResponse, 0, len(response.Policies))
	for _, p := range response.Policies {
		if !p.TraceOnly() {
			policies = append(policies, p)
		}
	}
	response.Policies = policies
	return response
}

func (h *handler) mutationResponse(request celengine.EngineRequest, response eval.ImageVerifyEngineResponse, rawPatches []byte) handlers.AdmissionResponse {
	var errs []error
	var warnings []string
	for _, policy := range response.Policies {
		if policy.Actions.Has(admissionregistrationv1.Deny) {
			switch policy.Result.Status() {
			case engineapi.RuleStatusFail:
				errs = append(errs, fmt.Errorf("Policy %s failed: %s", policy.Policy.GetName(), policy.Result.Message()))
			case engineapi.RuleStatusError:
				errs = append(errs, fmt.Errorf("Policy %s error: %s", policy.Policy.GetName(), policy.Result.Message()))
			}
		}

		if policy.Actions.Has(admissionregistrationv1.Warn) {
			switch policy.Result.Status() {
			case engineapi.RuleStatusFail:
				warnings = append(warnings, fmt.Sprintf("Policy %s failed: %s", policy.Policy.GetName(), policy.Result.Message()))
			case engineapi.RuleStatusError:
				warnings = append(warnings, fmt.Sprintf("Policy %s error: %s", policy.Policy.GetName(), policy.Result.Message()))
			}
		}
	}
	if len(errs) > 0 {
		return admissionutils.Response(request.AdmissionRequest().UID, multierr.Combine(errs...), warnings...)
	}

	return admissionutils.MutationResponse(request.AdmissionRequest().UID, rawPatches, warnings...)
}

// mutationEvents reports digest pinning failures as events against the policy and the resource.
// Only failures reach it: handleMutation records a policy result solely when a policy errors, so a
// mutation that pins every image (or has nothing to pin) is silent.
//
// Unlike the validating phase this deliberately creates no admission report. When the failure is not
// blocking, the request carries on to the validating webhook, which evaluates the same policies and
// reports against them, so reporting here as well would duplicate that. When it is blocking the
// request is rejected outright and no report is produced, matching ClusterPolicy, which likewise
// skips handleAudit for a blocked request (pkg/webhooks/resource/imageverification/handler.go).
func (h *handler) mutationEvents(ctx context.Context, response eval.ImageVerifyEngineResponse, blocked bool) {
	if len(response.Policies) == 0 || response.Resource == nil {
		return
	}
	responses := make([]engineapi.EngineResponse, 0, len(response.Policies))
	for _, r := range response.Policies {
		engineResponse := engineapi.EngineResponse{
			Resource: *response.Resource,
			PolicyResponse: engineapi.PolicyResponse{
				Rules: []engineapi.RuleResponse{r.Result},
			},
		}
		responses = append(responses, engineResponse.WithPolicy(engineapi.NewImageValidatingPolicyFromLike(r.Policy)))
	}
	h.admissionEvent(ctx, responses, blocked)
}

func (h *handler) validationResponse(request celengine.EngineRequest, response eval.ImageVerifyEngineResponse) handlers.AdmissionResponse {
	var errs []error
	var warnings []string
	for _, policy := range response.Policies {
		if policy.Actions.Has(admissionregistrationv1.Deny) {
			switch policy.Result.Status() {
			case engineapi.RuleStatusFail:
				errs = append(errs, fmt.Errorf("Policy %s failed: %s", policy.Policy.GetName(), policy.Result.Message()))
			case engineapi.RuleStatusError:
				errs = append(errs, fmt.Errorf("Policy %s error: %s", policy.Policy.GetName(), policy.Result.Message()))
			}
		}
		if policy.Actions.Has(admissionregistrationv1.Warn) {
			switch policy.Result.Status() {
			case engineapi.RuleStatusFail:
				warnings = append(warnings, fmt.Sprintf("Policy %s failed: %s", policy.Policy.GetName(), policy.Result.Message()))
			case engineapi.RuleStatusError:
				warnings = append(warnings, fmt.Sprintf("Policy %s error: %s", policy.Policy.GetName(), policy.Result.Message()))
			}
		}
	}
	return admissionutils.Response(request.AdmissionRequest().UID, multierr.Combine(errs...), warnings...)
}

func (h *handler) audit(ctx context.Context, logger logr.Logger, admissionRequest handlers.AdmissionRequest, request celengine.EngineRequest, response eval.ImageVerifyEngineResponse) {
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
				Rules: []engineapi.RuleResponse{r.Result},
			},
		}
		engineResponse = engineResponse.WithPolicy(engineapi.NewImageValidatingPolicyFromLike(r.Policy))
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

func (h *handler) admissionReport(ctx context.Context, request celengine.EngineRequest, response eval.ImageVerifyEngineResponse, responses []engineapi.EngineResponse) error {
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

func (h *handler) admissionEvent(ctx context.Context, responses []engineapi.EngineResponse, blocked bool) {
	for _, response := range responses {
		events := webhookutils.GenerateEvents([]engineapi.EngineResponse{response}, blocked)
		h.eventGen.Add(events...)
	}
}
