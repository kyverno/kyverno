package policy

import (
	"reflect"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	policyChangesMetric "github.com/kyverno/kyverno/pkg/metrics/policychanges"
	policyRuleInfoMetric "github.com/kyverno/kyverno/pkg/metrics/policyruleinfo"
)

func (pc *PolicyController) registerPolicyRuleInfoMetricAddPolicy(logger logr.Logger, p kyverno.PolicyInterface) {
	err := policyRuleInfoMetric.ParsePromConfig(*pc.promConfig).AddPolicy(p)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_rule_info_total metrics for the above policy's creation", "name", p.GetName())
	}
}

func (pc *PolicyController) registerPolicyRuleInfoMetricUpdatePolicy(logger logr.Logger, oldP, curP kyverno.PolicyInterface) {
	// removing the old rules associated metrics
	err := policyRuleInfoMetric.ParsePromConfig(*pc.promConfig).RemovePolicy(oldP)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_rule_info_total metrics for the above policy's updation", "name", oldP.GetName())
	}
	// adding the new rules associated metrics
	err = policyRuleInfoMetric.ParsePromConfig(*pc.promConfig).AddPolicy(curP)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_rule_info_total metrics for the above policy's updation", "name", oldP.GetName())
	}
}

func (pc *PolicyController) registerPolicyRuleInfoMetricDeletePolicy(logger logr.Logger, p kyverno.PolicyInterface) {
	err := policyRuleInfoMetric.ParsePromConfig(*pc.promConfig).RemovePolicy(p)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_rule_info_total metrics for the above policy's deletion", "name", p.GetName())
	}
}

func (pc *PolicyController) registerPolicyChangesMetricAddPolicy(logger logr.Logger, p kyverno.PolicyInterface) {
	err := policyChangesMetric.ParsePromConfig(*pc.promConfig).RegisterPolicy(p, policyChangesMetric.PolicyCreated)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_changes_total metrics for the above policy's creation", "name", p.GetName())
	}
}

func (pc *PolicyController) registerPolicyChangesMetricUpdatePolicy(logger logr.Logger, oldP, curP kyverno.PolicyInterface) {
	oldSpec := oldP.GetSpec()
	curSpec := curP.GetSpec()
	if reflect.DeepEqual(oldSpec, curSpec) {
		return
	}
	err := policyChangesMetric.ParsePromConfig(*pc.promConfig).RegisterPolicy(oldP, policyChangesMetric.PolicyUpdated)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_changes_total metrics for the above policy's updation", "name", oldP.GetName())
	}
	// curP will require a new kyverno_policy_changes_total metric if the above update involved change in the following fields:
	if curSpec.BackgroundProcessingEnabled() != oldSpec.BackgroundProcessingEnabled() || curSpec.GetValidationFailureAction() != oldSpec.GetValidationFailureAction() {
		err = policyChangesMetric.ParsePromConfig(*pc.promConfig).RegisterPolicy(curP, policyChangesMetric.PolicyUpdated)
		if err != nil {
			logger.Error(err, "error occurred while registering kyverno_policy_changes_total metrics for the above policy's updation", "name", curP.GetName())
		}
	}
}

func (pc *PolicyController) registerPolicyChangesMetricDeletePolicy(logger logr.Logger, p kyverno.PolicyInterface) {
	err := policyChangesMetric.ParsePromConfig(*pc.promConfig).RegisterPolicy(p, policyChangesMetric.PolicyDeleted)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_changes_total metrics for the above policy's deletion", "name", p.GetName())
	}
}

func (pc *PolicyController) registerPolicyRuleInfoMetricDeleteNsPolicy(logger logr.Logger, p *kyverno.Policy) {
	err := policyRuleInfoMetric.ParsePromConfig(*pc.promConfig).RemovePolicy(p)
	if err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_rule_info_total metrics for the above policy's deletion", "name", p.Name)
	}
}
