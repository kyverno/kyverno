package policymutation

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func generateCronJobRule(rule kyverno.Rule, controllers string, log logr.Logger) kyvernoRule {
	logger := log.WithName("handleCronJob")

	hasCronJob := strings.Contains(controllers, engine.PodControllerCronJob) || strings.Contains(controllers, "all")
	if !hasCronJob {
		return kyvernoRule{}
	}

	logger.V(3).Info("generating rule for cronJob")
	jobRule := generateRuleForControllers(rule, "Job", logger)

	if reflect.DeepEqual(jobRule, kyvernoRule{}) {
		return kyvernoRule{}
	}

	cronJobRule := &jobRule

	name := fmt.Sprintf("autogen-cronjob-%s", rule.Name)
	if len(name) > 63 {
		name = name[:63]
	}
	cronJobRule.Name = name

	cronJobRule.MatchResources.Kinds = []string{engine.PodControllerCronJob}
	if (jobRule.ExcludeResources) != nil && (len(jobRule.ExcludeResources.Kinds) > 0) {
		cronJobRule.ExcludeResources.Kinds = []string{engine.PodControllerCronJob}
	}

	var JSONValue apiextensions.JSON
	if (jobRule.Mutation != nil) && (jobRule.Mutation.Overlay.Raw != nil) {
		JSONValue, _ = kyverno.ConvertInterfaceToV1JSON(map[string]interface{}{
			"spec": map[string]interface{}{
				"jobTemplate": jobRule.Mutation.Overlay,
			},
		})
		newMutation := &kyverno.Mutation{
			Overlay: JSONValue,
		}

		cronJobRule.Mutation = newMutation.DeepCopy()
		return *cronJobRule
	}

	if (jobRule.Mutation != nil) && (jobRule.Mutation.PatchStrategicMerge.Raw != nil) {
		JSONValue, _ = kyverno.ConvertInterfaceToV1JSON(map[string]interface{}{
			"spec": map[string]interface{}{
				"jobTemplate": jobRule.Mutation.PatchStrategicMerge,
			},
		})
		newMutation := &kyverno.Mutation{
			PatchStrategicMerge: JSONValue,
		}
		cronJobRule.Mutation = newMutation.DeepCopy()
		return *cronJobRule
	}

	if (jobRule.Validation != nil) && (jobRule.Validation.Pattern.Raw != nil) {
		JSONValue, _ = kyverno.ConvertInterfaceToV1JSON(map[string]interface{}{
			"spec": map[string]interface{}{
				"jobTemplate": jobRule.Validation.Pattern,
			},
		})
		newValidate := &kyverno.Validation{
			Message: variables.FindAndShiftReferences(log, rule.Validation.Message, "spec/jobTemplate/spec/template", "pattern"),
			Pattern: JSONValue,
		}
		cronJobRule.Validation = newValidate.DeepCopy()
		return *cronJobRule
	}

	if (jobRule.Validation != nil) && (jobRule.Validation.AnyPattern.Raw != nil) {
		cronJobRule.Validation = &kyverno.Validation{
			Message:    variables.FindAndShiftReferences(log, rule.Validation.Message, "spec/jobTemplate/spec/template", "anyPattern"),
			AnyPattern: jobRule.Validation.AnyPattern,
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
