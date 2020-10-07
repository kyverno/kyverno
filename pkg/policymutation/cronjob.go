package policymutation

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine"
)

func generateCronJobRule(rule kyverno.Rule, controllers string, log logr.Logger) kyvernoRule {
	logger := log.WithName("handleCronJob")

	hasCronJob := strings.Contains(controllers, engine.PodControllerCronJob) || strings.Contains(controllers, "all")
	if !hasCronJob {
		return kyvernoRule{}
	}

	logger.V(3).Info("")
	jobRule := generateRuleForControllers(rule, "Job", logger)

	if reflect.DeepEqual(jobRule, kyvernoRule{}) {
		return kyvernoRule{}
	}

	cronJobRule := &jobRule
	cronJobRule.Name = fmt.Sprintf("autogen-cronjob-%s", rule.Name)
	cronJobRule.MatchResources.Kinds = []string{engine.PodControllerCronJob}
	if (jobRule.ExcludeResources) != nil && (len(jobRule.ExcludeResources.Kinds) > 0) {
		cronJobRule.ExcludeResources.Kinds = []string{engine.PodControllerCronJob}
	}

	if (jobRule.Mutation != nil) && (jobRule.Mutation.Overlay != nil) {
		newMutation := &kyverno.Mutation{
			Overlay: map[string]interface{}{
				"spec": map[string]interface{}{
					"jobTemplate": jobRule.Mutation.Overlay,
				},
			},
		}

		cronJobRule.Mutation = newMutation.DeepCopy()
		return *cronJobRule
	}

	if (jobRule.Mutation != nil) && (jobRule.Mutation.PatchStrategicMerge != nil) {
		newMutation := &kyverno.Mutation{
			PatchStrategicMerge: map[string]interface{}{
				"spec": map[string]interface{}{
					"jobTemplate": jobRule.Mutation.PatchStrategicMerge,
				},
			},
		}
		cronJobRule.Mutation = newMutation.DeepCopy()
		return *cronJobRule
	}

	if (jobRule.Validation != nil) && (jobRule.Validation.Pattern != nil) {
		newValidate := &kyverno.Validation{
			Message: rule.Validation.Message,
			Pattern: map[string]interface{}{
				"spec": map[string]interface{}{
					"jobTemplate": jobRule.Validation.Pattern,
				},
			},
		}
		cronJobRule.Validation = newValidate.DeepCopy()
		return *cronJobRule
	}

	if (jobRule.Validation != nil) && (len(jobRule.Validation.AnyPattern) != 0) {
		var patterns []interface{}
		for _, pattern := range jobRule.Validation.AnyPattern {
			newPattern := map[string]interface{}{
				"spec": map[string]interface{}{
					"jobTemplate": pattern,
				},
			}

			patterns = append(patterns, newPattern)
		}

		cronJobRule.Validation = &kyverno.Validation{
			Message:    rule.Validation.Message,
			AnyPattern: patterns,
		}
		return *cronJobRule
	}

	return kyvernoRule{}
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
