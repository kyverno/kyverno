package vpol

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	celpolicy "github.com/kyverno/kyverno/pkg/cel/policy"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"go.uber.org/multierr"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

type handler struct {
	context celpolicy.Context
	engine  celengine.Engine
}

func New(engine celengine.Engine, context celpolicy.Context) *handler {
	return &handler{
		context: context,
		engine:  engine,
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
	var errs []error
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
	}
	// TODO: reporting
	return admissionutils.Response(request.UID, multierr.Combine(errs...))
}
