package policychanges

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/metrics"
)

func registerPolicyChangesMetric(
	ctx context.Context,
	m metrics.MetricsConfigManager,
	policyValidationMode metrics.PolicyValidationMode,
	policyType metrics.PolicyType,
	policyBackgroundMode metrics.PolicyBackgroundMode,
	policyNamespace, policyName string,
	policyChangeType PolicyChangeType,
) {
	if policyType == metrics.Cluster {
		policyNamespace = "-"
	}
	if m.Config().CheckNamespace(policyNamespace) {
		m.RecordPolicyChanges(ctx, policyValidationMode, policyType, policyBackgroundMode, policyNamespace, policyName, string(policyChangeType))
	}
}

func RegisterPolicy(ctx context.Context, m metrics.MetricsConfigManager, policy kyvernov1.PolicyInterface, policyChangeType PolicyChangeType) error {
	name, namespace, policyType, backgroundMode, validationMode, err := metrics.GetPolicyInfos(policy)
	if err != nil {
		return err
	}
	registerPolicyChangesMetric(ctx, m, validationMode, policyType, backgroundMode, namespace, name, policyChangeType)
	return nil
}
