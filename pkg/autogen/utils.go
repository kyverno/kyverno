package autogen

import (
	"strings"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/utils"
)

func isKindOtherthanPod(kinds []string) bool {
	if len(kinds) > 1 && utils.ContainsPod(kinds, "Pod") {
		return true
	}
	return false
}

func hasAutogenKinds(kind []string) bool {
	for _, v := range kind {
		if v == "Pod" || strings.Contains(engine.PodControllers, v) {
			return true
		}
	}
	return false
}

func validateAnyPattern(anyPatterns []interface{}) []interface{} {
	var patterns []interface{}
	for _, pattern := range anyPatterns {
		newPattern := map[string]interface{}{
			"spec": map[string]interface{}{
				"template": pattern,
			},
		}
		patterns = append(patterns, newPattern)
	}
	return patterns
}

func getAnyAllAutogenRule(v kyverno.ResourceFilters, controllers string) kyverno.ResourceFilters {
	anyKind := v.DeepCopy()
	for i, value := range v {
		if utils.ContainsPod(value.Kinds, "Pod") {
			anyKind[i].Kinds = strings.Split(controllers, ",")
		}
	}
	return anyKind
}

// stripCronJob removes CronJob from controllers
func stripCronJob(controllers string) string {
	var newControllers []string
	controllerArr := strings.Split(controllers, ",")
	for _, c := range controllerArr {
		if c == engine.PodControllerCronJob {
			continue
		}
		newControllers = append(newControllers, c)
	}
	if len(newControllers) == 0 {
		return ""
	}
	return strings.Join(newControllers, ",")
}

func cronJobAnyAllAutogenRule(v kyverno.ResourceFilters) kyverno.ResourceFilters {
	anyKind := v.DeepCopy()
	for i, value := range v {
		if utils.ContainsPod(value.Kinds, "Job") {
			anyKind[i].Kinds = []string{engine.PodControllerCronJob}
		}
	}
	return anyKind
}

func arrayContains(array []string, item string) bool {
	for _, candidate := range array {
		if candidate == item {
			return true
		}
	}
	return false
}
