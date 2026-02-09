package policychanges

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/metrics"
)

func registerPolicyChangesMetric(
	ctx context.Context,
	policyValidationMode metrics.PolicyValidationMode,
	policyType metrics.PolicyType,
	policyBackgroundMode metrics.PolicyBackgroundMode,
	policyNamespace, policyName string,
	policyChangeType PolicyChangeType,
) {
	if policyType == metrics.Cluster {
		policyNamespace = "-"
	}
	if metrics.GetManager().Config().CheckNamespace(policyNamespace) {
		metrics.GetManager().RecordPolicyChanges(ctx, policyValidationMode, policyType, policyBackgroundMode, policyNamespace, policyName, string(policyChangeType))
	}
}

func RegisterPolicy(ctx context.Context, policy kyvernov1.PolicyInterface, policyChangeType PolicyChangeType) error {
	name, namespace, policyType, backgroundMode, validationMode, err := metrics.GetPolicyInfos(policy)
	if err != nil {
		return err
	}
	registerPolicyChangesMetric(ctx, validationMode, policyType, backgroundMode, namespace, name, policyChangeType)
	return nil
}
