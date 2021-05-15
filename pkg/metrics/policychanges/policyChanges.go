package policychanges

import (
	"fmt"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/metrics"
	prom "github.com/prometheus/client_golang/prometheus"
	"time"
)

func (pm PromMetrics) registerPolicyChangesMetric(
	policyValidationMode metrics.PolicyValidationMode,
	policyType metrics.PolicyType,
	policyBackgroundMode metrics.PolicyBackgroundMode,
	policyNamespace, policyName string,
	policyChangeType PolicyChangeType,
	policyChangeTimestamp int64,
) error {
	if policyType == metrics.Cluster {
		policyNamespace = "-"
	}
	pm.PolicyChanges.With(prom.Labels{
		"policy_validation_mode": string(policyValidationMode),
		"policy_type":            string(policyType),
		"policy_background_mode": string(policyBackgroundMode),
		"policy_namespace":       policyNamespace,
		"policy_name":            policyName,
		"policy_change_type":     string(policyChangeType),
		"timestamp":              fmt.Sprintf("%+v", time.Unix(policyChangeTimestamp, 0)),
	}).Set(1)
	return nil
}

func (pm PromMetrics) RegisterPolicy(policy interface{}, policyChangeType PolicyChangeType, policyChangeTimestamp int64) error {
	switch inputPolicy := policy.(type) {
	case *kyverno.ClusterPolicy:
		policyValidationMode, err := metrics.ParsePolicyValidationMode(inputPolicy.Spec.ValidationFailureAction)
		if err != nil {
			return err
		}
		policyBackgroundMode := metrics.ParsePolicyBackgroundMode(*inputPolicy.Spec.Background)
		policyType := metrics.Cluster
		policyNamespace := "" // doesn't matter for cluster policy
		policyName := inputPolicy.ObjectMeta.Name
		if err = pm.registerPolicyChangesMetric(policyValidationMode, policyType, policyBackgroundMode, policyNamespace, policyName, policyChangeType, policyChangeTimestamp); err != nil {
			return err
		}
		return nil
	case *kyverno.Policy:
		policyValidationMode, err := metrics.ParsePolicyValidationMode(inputPolicy.Spec.ValidationFailureAction)
		if err != nil {
			return err
		}
		policyBackgroundMode := metrics.ParsePolicyBackgroundMode(*inputPolicy.Spec.Background)
		policyType := metrics.Namespaced
		policyNamespace := inputPolicy.ObjectMeta.Namespace
		policyName := inputPolicy.ObjectMeta.Name
		if err = pm.registerPolicyChangesMetric(policyValidationMode, policyType, policyBackgroundMode, policyNamespace, policyName, policyChangeType, policyChangeTimestamp); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("wrong input type provided %T. Only kyverno.Policy and kyverno.ClusterPolicy allowed", inputPolicy)
	}
}
