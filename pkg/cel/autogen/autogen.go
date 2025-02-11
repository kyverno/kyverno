package autogen

import (
	"strings"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
)

var podControllers = sets.New("daemonsets", "deployments", "jobs", "statefulsets", "replicasets", "cronjobs")

// canAutoGen checks whether the policy can be applied to Pod controllers
// It returns false if:
//   - name or selector is defined
//   - mixed kinds (Pod + pod controller) is defined
//   - Pod is not defined
//
// Otherwise it returns all pod controllers
func CanAutoGen(spec *kyvernov2alpha1.ValidatingPolicySpec) (bool, sets.Set[string]) {
	match := spec.MatchConstraints
	if match.NamespaceSelector != nil {
		if len(match.NamespaceSelector.MatchLabels) > 0 || len(match.NamespaceSelector.MatchExpressions) > 0 {
			return false, sets.New[string]()
		}
	}
	if match.ObjectSelector != nil {
		if len(match.ObjectSelector.MatchLabels) > 0 || len(match.ObjectSelector.MatchExpressions) > 0 {
			return false, sets.New[string]()
		}
	}

	rules := match.ResourceRules
	for _, rule := range rules {
		if len(rule.ResourceNames) > 0 {
			return false, sets.New[string]()
		}
		if len(rule.Resources) > 1 {
			return false, sets.New[string]()
		}
		if rule.Resources[0] != "pods" {
			return false, sets.New[string]()
		}
	}
	return true, podControllers
}

func generateRules(spec *kyvernov2alpha1.ValidatingPolicySpec, controllers string) []kyvernov2alpha1.AutogenRule {
	var genRules []kyvernov2alpha1.AutogenRule
	// strip cronjobs from controllers if exist
	isRemoved, controllers := stripCronJob(controllers)
	// generate rule for pod controllers
	if genRule, err := generateRuleForControllers(spec, controllers); err == nil {
		genRules = append(genRules, *genRule.DeepCopy())
	}

	// generate rule for cronjobs if exist
	if isRemoved {
		if genRule, err := generateCronJobRule(spec, "cronjobs"); err == nil {
			genRules = append(genRules, *genRule.DeepCopy())
		}
	}
	return genRules
}

// stripCronJob removes the cronjobs from controllers
// it returns true, if cronjobs is removed
func stripCronJob(controllers string) (bool, string) {
	controllerArr := strings.Split(controllers, ",")
	newControllers := make([]string, 0, len(controllerArr))
	isRemoved := false
	for _, c := range controllerArr {
		if c == "cronjobs" {
			isRemoved = true
			continue
		}
		newControllers = append(newControllers, c)
	}
	if len(newControllers) == 0 {
		return isRemoved, ""
	}
	return isRemoved, strings.Join(newControllers, ",")
}

func ComputeRules(policy kyvernov2alpha1.GenericPolicy) []kyvernov2alpha1.AutogenRule {
	applyAutoGen, desiredControllers := CanAutoGen(policy.GetSpec())
	if !applyAutoGen {
		return []kyvernov2alpha1.AutogenRule{}
	}

	var actualControllers sets.Set[string]
	ann := policy.GetAnnotations()
	actualControllersString, ok := ann[kyverno.AnnotationAutogenControllers]
	if !ok {
		actualControllers = desiredControllers
	} else {
		actualControllers = sets.New(strings.Split(actualControllersString, ",")...)
	}

	resources := strings.Join(actualControllers.UnsortedList(), ",")
	genRules := generateRules(policy.GetSpec().DeepCopy(), resources)
	return genRules
}
