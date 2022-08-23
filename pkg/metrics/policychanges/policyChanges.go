package policychanges

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/utils"
)

func registerPolicyChangesMetric(
	m *metrics.MetricsConfig,
	policyValidationMode metrics.PolicyValidationMode,
	policyType metrics.PolicyType,
	policyBackgroundMode metrics.PolicyBackgroundMode,
	policyNamespace, policyName string,
	policyChangeType PolicyChangeType,
) error {
	if policyType == metrics.Cluster {
		policyNamespace = "-"
	}
	includeNamespaces, excludeNamespaces := m.Config.GetIncludeNamespaces(), m.Config.GetExcludeNamespaces()
	if (policyNamespace != "" && policyNamespace != "-") && utils.ContainsString(excludeNamespaces, policyNamespace) {
		m.Log.V(2).Info(fmt.Sprintf("Skipping the registration of kyverno_policy_changes_total metric as the operation belongs to the namespace '%s' which is one of 'namespaces.exclude' %+v in values.yaml", policyNamespace, excludeNamespaces))
		return nil
	}
	if (policyNamespace != "" && policyNamespace != "-") && len(includeNamespaces) > 0 && !utils.ContainsString(includeNamespaces, policyNamespace) {
		m.Log.V(2).Info(fmt.Sprintf("Skipping the registration of kyverno_policy_changes_total metric as the operation belongs to the namespace '%s' which is not one of 'namespaces.include' %+v in values.yaml", policyNamespace, includeNamespaces))
		return nil
	}

	m.RecordPolicyChanges(policyValidationMode, policyType, policyBackgroundMode, policyNamespace, policyName, string(policyChangeType))

	return nil
}

func RegisterPolicy(m *metrics.MetricsConfig, policy kyvernov1.PolicyInterface, policyChangeType PolicyChangeType) error {
	name, namespace, policyType, backgroundMode, validationMode, err := metrics.GetPolicyInfos(policy)
	if err != nil {
		return err
	}
	if err = registerPolicyChangesMetric(m, validationMode, policyType, backgroundMode, namespace, name, policyChangeType); err != nil {
		return err
	}
	return nil
}
