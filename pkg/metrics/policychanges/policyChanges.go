package policychanges

import (
	"fmt"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
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
<<<<<<< HEAD
		metrics.Logger().Info(fmt.Sprintf("Skipping the registration of kyverno_policy_changes_total metric as the operation belongs to the namespace '%s' which is one of 'namespaces.exclude' %+v in values.yaml", policyNamespace, excludeNamespaces))
		return nil
	}
	if (policyNamespace != "" && policyNamespace != "-") && len(includeNamespaces) > 0 && !utils.ContainsString(includeNamespaces, policyNamespace) {
		metrics.Logger().Info(fmt.Sprintf("Skipping the registration of kyverno_policy_changes_total metric as the operation belongs to the namespace '%s' which is not one of 'namespaces.include' %+v in values.yaml", policyNamespace, includeNamespaces))
=======
		m.Log.Info(fmt.Sprintf("Skipping the registration of kyverno_policy_changes_total metric as the operation belongs to the namespace '%s' which is one of 'namespaces.exclude' %+v in values.yaml", policyNamespace, excludeNamespaces))
		return nil
	}
	if (policyNamespace != "" && policyNamespace != "-") && len(includeNamespaces) > 0 && !utils.ContainsString(includeNamespaces, policyNamespace) {
		m.Log.Info(fmt.Sprintf("Skipping the registration of kyverno_policy_changes_total metric as the operation belongs to the namespace '%s' which is not one of 'namespaces.include' %+v in values.yaml", policyNamespace, includeNamespaces))
>>>>>>> 4d3fab5be (metrics in otel format, created struct for binding data)
		return nil
	}

	m.RecordPolicyChanges(policyValidationMode, policyType, policyBackgroundMode, policyNamespace, policyName, string(policyChangeType), m.Log)

	return nil
}

func RegisterPolicy(m *metrics.MetricsConfig, policy kyverno.PolicyInterface, policyChangeType PolicyChangeType) error {
	name, namespace, policyType, backgroundMode, validationMode, err := metrics.GetPolicyInfos(policy)
	if err != nil {
		return err
	}
	if err = registerPolicyChangesMetric(m, validationMode, policyType, backgroundMode, namespace, name, policyChangeType); err != nil {
		return err
	}
	return nil
}
