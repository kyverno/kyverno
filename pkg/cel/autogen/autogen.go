package autogen

import (
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
)

var podControllers = sets.New("daemonsets", "deployments", "jobs", "statefulsets", "replicasets", "cronjobs")

func ComputeRules(policy *policiesv1alpha1.ValidatingPolicy) []policiesv1alpha1.AutogenRule {
	applyAutoGen := CanAutoGen(policy.GetSpec().MatchConstraints)
	if !applyAutoGen {
		return []policiesv1alpha1.AutogenRule{}
	}
	actualControllers := podControllers
	if policy.Spec.AutogenConfiguration != nil &&
		policy.Spec.AutogenConfiguration.PodControllers != nil &&
		policy.Spec.AutogenConfiguration.PodControllers.Controllers != nil {
		actualControllers = sets.New(policy.Spec.AutogenConfiguration.PodControllers.Controllers...)
	}
	return generateRules(policy.GetSpec().DeepCopy(), actualControllers)
}

func generateRules(spec *policiesv1alpha1.ValidatingPolicySpec, configs sets.Set[string]) []policiesv1alpha1.AutogenRule {
	var genRules []policiesv1alpha1.AutogenRule
	cronjobs := sets.New("cronjobs")
	// generate rule for cronjobs if exist
	if configs.Has("cronjobs") {
		if genRule, err := generateCronJobRule(spec.DeepCopy(), cronjobs); err == nil {
			genRules = append(genRules, *genRule.DeepCopy())
		}
	}
	// generate rule for pod controllers
	if configs := configs.Difference(cronjobs); configs.Len() != 0 {
		if genRule, err := generatePodControllerRule(spec.DeepCopy(), configs); err == nil {
			genRules = append(genRules, *genRule.DeepCopy())
		}
	}
	return genRules
}
