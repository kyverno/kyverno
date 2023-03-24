package policy

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	policyChangesMetric "github.com/kyverno/kyverno/pkg/metrics/policychanges"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
)

func (pc *controller) registerPolicyChangesMetricAddPolicy(ctx context.Context, logger logr.Logger, p kyvernov1.PolicyInterface) {
	err := policyChangesMetric.RegisterPolicy(ctx, pc.metricsConfig, p, policyChangesMetric.PolicyCreated)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_changes_total metrics for the above policy's creation", "name", p.GetName())
	}
}

func (pc *controller) registerPolicyChangesMetricUpdatePolicy(ctx context.Context, logger logr.Logger, oldP, curP kyvernov1.PolicyInterface) {
	oldSpec := oldP.GetSpec()
	curSpec := curP.GetSpec()
	if datautils.DeepEqual(oldSpec, curSpec) {
		return
	}
	err := policyChangesMetric.RegisterPolicy(ctx, pc.metricsConfig, oldP, policyChangesMetric.PolicyUpdated)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_changes_total metrics for the above policy's updation", "name", oldP.GetName())
	}
	// curP will require a new kyverno_policy_changes_total metric if the above update involved change in the following fields:
	if curSpec.BackgroundProcessingEnabled() != oldSpec.BackgroundProcessingEnabled() || curSpec.ValidationFailureAction.Enforce() != oldSpec.ValidationFailureAction.Enforce() {
		err = policyChangesMetric.RegisterPolicy(ctx, pc.metricsConfig, curP, policyChangesMetric.PolicyUpdated)
		if err != nil {
			logger.Error(err, "error occurred while registering kyverno_policy_changes_total metrics for the above policy's updation", "name", curP.GetName())
		}
	}
}

func (pc *controller) registerPolicyChangesMetricDeletePolicy(ctx context.Context, logger logr.Logger, p kyvernov1.PolicyInterface) {
	err := policyChangesMetric.RegisterPolicy(ctx, pc.metricsConfig, p, policyChangesMetric.PolicyDeleted)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_changes_total metrics for the above policy's deletion", "name", p.GetName())
	}
}
