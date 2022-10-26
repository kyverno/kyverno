package policy

import (
	"reflect"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	policyChangesMetric "github.com/kyverno/kyverno/pkg/metrics/policychanges"
	policyRuleInfoMetric "github.com/kyverno/kyverno/pkg/metrics/policyruleinfo"
)

func (pc *controller) registerPolicyRuleInfoMetricAddPolicy(logger logr.Logger, p kyvernov1.PolicyInterface) {
	err := policyRuleInfoMetric.AddPolicy(pc.metricsConfig, p)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_rule_info_total metrics for the above policy's creation", "name", p.GetName())
	}
}

func (pc *controller) registerPolicyRuleInfoMetricUpdatePolicy(logger logr.Logger, oldP, curP kyvernov1.PolicyInterface) {
	// removing the old rules associated metrics
	err := policyRuleInfoMetric.RemovePolicy(pc.metricsConfig, oldP)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_rule_info_total metrics for the above policy's updation", "name", oldP.GetName())
	}
	// adding the new rules associated metrics
	err = policyRuleInfoMetric.AddPolicy(pc.metricsConfig, curP)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_rule_info_total metrics for the above policy's updation", "name", oldP.GetName())
	}
}

func (pc *controller) registerPolicyRuleInfoMetricDeletePolicy(logger logr.Logger, p kyvernov1.PolicyInterface) {
	err := policyRuleInfoMetric.RemovePolicy(pc.metricsConfig, p)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_rule_info_total metrics for the above policy's deletion", "name", p.GetName())
	}
}

func (pc *controller) registerPolicyChangesMetricAddPolicy(logger logr.Logger, p kyvernov1.PolicyInterface) {
	err := policyChangesMetric.RegisterPolicy(pc.metricsConfig, p, policyChangesMetric.PolicyCreated)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_changes_total metrics for the above policy's creation", "name", p.GetName())
	}
}

func (pc *controller) registerPolicyChangesMetricUpdatePolicy(logger logr.Logger, oldP, curP kyvernov1.PolicyInterface) {
	oldSpec := oldP.GetSpec()
	curSpec := curP.GetSpec()
	if reflect.DeepEqual(oldSpec, curSpec) {
		return
	}
	err := policyChangesMetric.RegisterPolicy(pc.metricsConfig, oldP, policyChangesMetric.PolicyUpdated)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_changes_total metrics for the above policy's updation", "name", oldP.GetName())
	}
	// curP will require a new kyverno_policy_changes_total metric if the above update involved change in the following fields:
	if curSpec.BackgroundProcessingEnabled() != oldSpec.BackgroundProcessingEnabled() || curSpec.GetValidationFailureAction() != oldSpec.GetValidationFailureAction() {
		err = policyChangesMetric.RegisterPolicy(pc.metricsConfig, curP, policyChangesMetric.PolicyUpdated)
		if err != nil {
			logger.Error(err, "error occurred while registering kyverno_policy_changes_total metrics for the above policy's updation", "name", curP.GetName())
		}
	}
}

func (pc *controller) registerPolicyChangesMetricDeletePolicy(logger logr.Logger, p kyvernov1.PolicyInterface) {
	err := policyChangesMetric.RegisterPolicy(pc.metricsConfig, p, policyChangesMetric.PolicyDeleted)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_changes_total metrics for the above policy's deletion", "name", p.GetName())
	}
}
