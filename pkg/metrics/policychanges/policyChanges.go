package policychanges

import (
	"fmt"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/metrics"
	prom "github.com/prometheus/client_golang/prometheus"
)

func (pc PromConfig) registerPolicyChangesMetric(
	policyValidationMode metrics.PolicyValidationMode,
	policyType metrics.PolicyType,
	policyBackgroundMode metrics.PolicyBackgroundMode,
	policyNamespace, policyName string,
	policyChangeType PolicyChangeType,
) error {
	if policyType == metrics.Cluster {
		policyNamespace = "-"
	}
	includeNamespaces, excludeNamespaces := pc.Config.GetIncludeNamespaces(), pc.Config.GetExcludeNamespaces()
	if (policyNamespace != "" && policyNamespace != "-") && metrics.ElementInSlice(policyNamespace, excludeNamespaces) {
		pc.Log.Info(fmt.Sprintf("Skipping the registration of kyverno_policy_changes_total metric as the operation belongs to the namespace '%s' which is one of 'namespaces.exclude' %+v in values.yaml", policyNamespace, excludeNamespaces))
		return nil
	}
	if (policyNamespace != "" && policyNamespace != "-") && len(includeNamespaces) > 0 && !metrics.ElementInSlice(policyNamespace, includeNamespaces) {
		pc.Log.Info(fmt.Sprintf("Skipping the registration of kyverno_policy_changes_total metric as the operation belongs to the namespace '%s' which is not one of 'namespaces.include' %+v in values.yaml", policyNamespace, includeNamespaces))
		return nil
	}
	pc.Metrics.PolicyChanges.With(prom.Labels{
		"policy_validation_mode": string(policyValidationMode),
		"policy_type":            string(policyType),
		"policy_background_mode": string(policyBackgroundMode),
		"policy_namespace":       policyNamespace,
		"policy_name":            policyName,
		"policy_change_type":     string(policyChangeType),
	}).Inc()
	return nil
}

func (pc PromConfig) RegisterPolicy(policy interface{}, policyChangeType PolicyChangeType) error {
	switch inputPolicy := policy.(type) {
	case *kyverno.ClusterPolicy:
		policyValidationMode, err := metrics.ParsePolicyValidationMode(inputPolicy.Spec.ValidationFailureAction)
		if err != nil {
			return err
		}
		policyBackgroundMode := metrics.ParsePolicyBackgroundMode(inputPolicy.Spec.Background)
		policyType := metrics.Cluster
		policyNamespace := "" // doesn't matter for cluster policy
		policyName := inputPolicy.ObjectMeta.Name
		if err = pc.registerPolicyChangesMetric(policyValidationMode, policyType, policyBackgroundMode, policyNamespace, policyName, policyChangeType); err != nil {
			return err
		}
		return nil
	case *kyverno.Policy:
		policyValidationMode, err := metrics.ParsePolicyValidationMode(inputPolicy.Spec.ValidationFailureAction)
		if err != nil {
			return err
		}
		policyBackgroundMode := metrics.ParsePolicyBackgroundMode(inputPolicy.Spec.Background)
		policyType := metrics.Namespaced
		policyNamespace := inputPolicy.ObjectMeta.Namespace
		policyName := inputPolicy.ObjectMeta.Name
		if err = pc.registerPolicyChangesMetric(policyValidationMode, policyType, policyBackgroundMode, policyNamespace, policyName, policyChangeType); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("wrong input type provided %T. Only kyverno.Policy and kyverno.ClusterPolicy allowed", inputPolicy)
	}
}
