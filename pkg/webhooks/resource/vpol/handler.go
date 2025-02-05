package vpol

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/breaker"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	celpolicy "github.com/kyverno/kyverno/pkg/cel/policy"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"go.uber.org/multierr"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

type handler struct {
	context        celpolicy.Context
	engine         celengine.Engine
	kyvernoClient  versioned.Interface
	reportsBreaker breaker.Breaker
}

func New(
	engine celengine.Engine,
	context celpolicy.Context,
	kyvernoClient versioned.Interface,
	reportsBreaker breaker.Breaker,
) *handler {
	return &handler{
		context:        context,
		engine:         engine,
		kyvernoClient:  kyvernoClient,
		reportsBreaker: reportsBreaker,
	}
}

func (h *handler) Validate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, failurePolicy string, startTime time.Time) handlers.AdmissionResponse {
	response, err := h.engine.Handle(ctx, celengine.EngineRequest{
		Request: &request.AdmissionRequest,
		Context: h.context,
	})
	if err != nil {
		return admissionutils.Response(request.UID, err)
	}
	var group wait.Group
	defer group.Wait()
	group.Start(func() {
		err := h.admissionReport(ctx, response, request)
		if err != nil {
			logger.Error(err, "failed to create report")
		}
	})
	return h.admissionResponse(response, request)
}

func (h *handler) admissionResponse(response celengine.EngineResponse, request handlers.AdmissionRequest) handlers.AdmissionResponse {
	var errs []error
	var warnings []string
	for _, policy := range response.Policies {
		if policy.Actions.Has(admissionregistrationv1.Deny) {
			for _, rule := range policy.Rules {
				switch rule.Status() {
				case engineapi.RuleStatusFail:
					errs = append(errs, fmt.Errorf("Policy %s rule %s failed: %s", policy.Policy.GetName(), rule.Name(), rule.Message()))
				case engineapi.RuleStatusError:
					errs = append(errs, fmt.Errorf("Policy %s rule %s error: %s", policy.Policy.GetName(), rule.Name(), rule.Message()))
				}
			}
		}
		if policy.Actions.Has(admissionregistrationv1.Warn) {
			for _, rule := range policy.Rules {
				switch rule.Status() {
				case engineapi.RuleStatusFail:
					warnings = append(warnings, fmt.Sprintf("Policy %s rule %s failed: %s", policy.Policy.GetName(), rule.Name(), rule.Message()))
				case engineapi.RuleStatusError:
					warnings = append(warnings, fmt.Sprintf("Policy %s rule %s error: %s", policy.Policy.GetName(), rule.Name(), rule.Message()))
				}
			}
		}
	}
	return admissionutils.Response(request.UID, multierr.Combine(errs...), warnings...)
}

func (h *handler) admissionReport(ctx context.Context, response celengine.EngineResponse, request handlers.AdmissionRequest) error {
	object, oldObject, err := admissionutils.ExtractResources(nil, request.AdmissionRequest)
	if err != nil {
		return err
	}
	if object.Object == nil {
		object = oldObject
	}
	responses := make([]engineapi.EngineResponse, 0, len(response.Policies))
	for _, r := range response.Policies {
		engineResponse := engineapi.EngineResponse{
			Resource: object,
			PolicyResponse: engineapi.PolicyResponse{
				Rules: r.Rules,
			},
		}
		engineResponse = engineResponse.WithPolicy(engineapi.NewValidatingPolicy(&r.Policy))
		responses = append(responses, engineResponse)
	}
	report := reportutils.BuildAdmissionReport(object, request.AdmissionRequest, responses...)
	if len(report.GetResults()) > 0 {
		err := h.reportsBreaker.Do(ctx, func(ctx context.Context) error {
			_, err := reportutils.CreateReport(ctx, report, h.kyvernoClient)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}
