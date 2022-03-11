package autogen

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/utils"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// the kyvernoRule holds the temporary kyverno rule struct
// each field is a pointer to the actual object
// when serializing data, we would expect to drop the omitempty key
// otherwise (without the pointer), it will be set to empty value
// - an empty struct in this case, some may fail the schema validation
// may related to:
// https://github.com/kyverno/kyverno/pull/549#discussion_r360088556
// https://github.com/kyverno/kyverno/issues/568

type kyvernoRule struct {
	Name             string                       `json:"name"`
	MatchResources   *kyverno.MatchResources      `json:"match"`
	ExcludeResources *kyverno.ExcludeResources    `json:"exclude,omitempty"`
	Context          *[]kyverno.ContextEntry      `json:"context,omitempty"`
	AnyAllConditions *apiextensions.JSON          `json:"preconditions,omitempty"`
	Mutation         *kyverno.Mutation            `json:"mutate,omitempty"`
	Validation       *kyverno.Validation          `json:"validate,omitempty"`
	VerifyImages     []*kyverno.ImageVerification `json:"verifyImages,omitempty" yaml:"verifyImages,omitempty"`
}

func createRuleMap(rules []kyverno.Rule) map[string]kyvernoRule {
	var ruleMap = make(map[string]kyvernoRule)
	for _, rule := range rules {
		var jsonFriendlyStruct kyvernoRule

		jsonFriendlyStruct.Name = rule.Name

		if !reflect.DeepEqual(rule.MatchResources, kyverno.MatchResources{}) {
			jsonFriendlyStruct.MatchResources = rule.MatchResources.DeepCopy()
		}

		if !reflect.DeepEqual(rule.ExcludeResources, kyverno.ExcludeResources{}) {
			jsonFriendlyStruct.ExcludeResources = rule.ExcludeResources.DeepCopy()
		}

		if !reflect.DeepEqual(rule.Mutation, kyverno.Mutation{}) {
			jsonFriendlyStruct.Mutation = rule.Mutation.DeepCopy()
		}

		if !reflect.DeepEqual(rule.Validation, kyverno.Validation{}) {
			jsonFriendlyStruct.Validation = rule.Validation.DeepCopy()
		}

		ruleMap[rule.Name] = jsonFriendlyStruct
	}
	return ruleMap
}

func generateRuleForControllers(rule kyverno.Rule, controllers string, log logr.Logger) *kyvernoRule {
	logger := log.WithName("generateRuleForControllers")

	if strings.HasPrefix(rule.Name, "autogen-") || controllers == "" {
		logger.V(5).Info("skip generateRuleForControllers")
		return nil
	}

	logger.V(3).Info("processing rule", "rulename", rule.Name)

	match := rule.MatchResources
	exclude := rule.ExcludeResources

	matchResourceDescriptionsKinds := rule.MatchKinds()
	excludeResourceDescriptionsKinds := rule.ExcludeKinds()

	if !utils.ContainsKind(matchResourceDescriptionsKinds, "Pod") ||
		(len(excludeResourceDescriptionsKinds) != 0 && !utils.ContainsKind(excludeResourceDescriptionsKinds, "Pod")) {
		return nil
	}

	// Support backwards compatibility
	skipAutoGeneration := false
	var controllersValidated []string
	if controllers == "all" {
		skipAutoGeneration = true
	} else if controllers != "none" && controllers != "all" {
		controllersList := map[string]int{"DaemonSet": 1, "Deployment": 1, "Job": 1, "StatefulSet": 1}
		for _, value := range strings.Split(controllers, ",") {
			if _, ok := controllersList[value]; ok {
				controllersValidated = append(controllersValidated, value)
			}
		}
		if len(controllersValidated) > 0 {
			skipAutoGeneration = true
		}
	}

	if skipAutoGeneration {
		if controllers == "all" {
			controllers = "DaemonSet,Deployment,Job,StatefulSet"
		} else {
			controllers = strings.Join(controllersValidated, ",")
		}
	}

	name := fmt.Sprintf("autogen-%s", rule.Name)
	if len(name) > 63 {
		name = name[:63]
	}

	controllerRule := &kyvernoRule{
		Name:           name,
		MatchResources: match.DeepCopy(),
	}

	if len(rule.Context) > 0 {
		controllerRule.Context = &rule.DeepCopy().Context
	}

	kyvernoAnyAllConditions, _ := utils.ApiextensionsJsonToKyvernoConditions(rule.GetAnyAllConditions())
	switch typedAnyAllConditions := kyvernoAnyAllConditions.(type) {
	case kyverno.AnyAllConditions:
		if !reflect.DeepEqual(typedAnyAllConditions, kyverno.AnyAllConditions{}) {
			controllerRule.AnyAllConditions = rule.DeepCopy().RawAnyAllConditions
		}
	case []kyverno.Condition:
		if len(typedAnyAllConditions) > 0 {
			controllerRule.AnyAllConditions = rule.DeepCopy().RawAnyAllConditions
		}
	}

	if !reflect.DeepEqual(exclude, kyverno.ExcludeResources{}) {
		controllerRule.ExcludeResources = exclude.DeepCopy()
	}

	// overwrite Kinds by pod controllers defined in the annotation
	if len(rule.MatchResources.Any) > 0 {
		rule := getAnyAllAutogenRule(controllerRule.MatchResources.Any, controllers)
		controllerRule.MatchResources.Any = rule
	} else if len(rule.MatchResources.All) > 0 {
		rule := getAnyAllAutogenRule(controllerRule.MatchResources.All, controllers)
		controllerRule.MatchResources.All = rule
	} else {
		controllerRule.MatchResources.Kinds = strings.Split(controllers, ",")
	}

	if len(rule.ExcludeResources.Any) > 0 {
		rule := getAnyAllAutogenRule(controllerRule.ExcludeResources.Any, controllers)
		controllerRule.ExcludeResources.Any = rule
	} else if len(rule.ExcludeResources.All) > 0 {
		rule := getAnyAllAutogenRule(controllerRule.ExcludeResources.All, controllers)
		controllerRule.ExcludeResources.All = rule
	} else {
		if len(exclude.Kinds) != 0 {
			controllerRule.ExcludeResources.Kinds = strings.Split(controllers, ",")
		}
	}

	if target := rule.Mutation.GetPatchStrategicMerge(); target != nil {
		newMutation := &kyverno.Mutation{}
		newMutation.SetPatchStrategicMerge(
			map[string]interface{}{
				"spec": map[string]interface{}{
					"template": target,
				},
			},
		)
		controllerRule.Mutation = newMutation.DeepCopy()
		return controllerRule
	}

	if len(rule.Mutation.ForEachMutation) > 0 && rule.Mutation.ForEachMutation != nil {
		var newForeachMutation []*kyverno.ForEachMutation
		for _, foreach := range rule.Mutation.ForEachMutation {
			temp := &kyverno.ForEachMutation{
				List:             foreach.List,
				Context:          foreach.Context,
				AnyAllConditions: foreach.AnyAllConditions,
			}
			temp.SetPatchStrategicMerge(
				map[string]interface{}{
					"spec": map[string]interface{}{
						"template": foreach.GetPatchStrategicMerge(),
					},
				},
			)
			newForeachMutation = append(newForeachMutation, temp)
		}
		controllerRule.Mutation = &kyverno.Mutation{
			ForEachMutation: newForeachMutation,
		}
		return controllerRule
	}

	if target := rule.Validation.GetPattern(); target != nil {
		newValidate := &kyverno.Validation{
			Message: variables.FindAndShiftReferences(log, rule.Validation.Message, "spec/template", "pattern"),
		}
		newValidate.SetPattern(
			map[string]interface{}{
				"spec": map[string]interface{}{
					"template": target,
				},
			},
		)
		controllerRule.Validation = newValidate.DeepCopy()
		return controllerRule
	}

	if rule.Validation.Deny != nil {
		deny := &kyverno.Validation{
			Message: variables.FindAndShiftReferences(log, rule.Validation.Message, "spec/template", "deny"),
			Deny:    rule.Validation.Deny,
		}
		controllerRule.Validation = deny.DeepCopy()
		return controllerRule
	}

	if rule.Validation.GetAnyPattern() != nil {
		anyPatterns, err := rule.Validation.DeserializeAnyPattern()
		if err != nil {
			logger.Error(err, "failed to deserialize anyPattern, expect type array")
		}
		patterns := validateAnyPattern(anyPatterns)
		controllerRule.Validation = &kyverno.Validation{
			Message: variables.FindAndShiftReferences(log, rule.Validation.Message, "spec/template", "anyPattern"),
		}
		controllerRule.Validation.SetAnyPattern(patterns)
		return controllerRule
	}

	if len(rule.Validation.ForEachValidation) > 0 && rule.Validation.ForEachValidation != nil {
		newForeachValidate := make([]*kyverno.ForEachValidation, len(rule.Validation.ForEachValidation))
		for i, foreach := range rule.Validation.ForEachValidation {
			newForeachValidate[i] = foreach
		}
		controllerRule.Validation = &kyverno.Validation{
			Message:           variables.FindAndShiftReferences(log, rule.Validation.Message, "spec/template", "pattern"),
			ForEachValidation: newForeachValidate,
		}
		return controllerRule
	}

	if rule.VerifyImages != nil {
		newVerifyImages := make([]*kyverno.ImageVerification, len(rule.VerifyImages))
		for i, vi := range rule.VerifyImages {
			newVerifyImages[i] = vi.DeepCopy()
		}

		controllerRule.VerifyImages = newVerifyImages
		return controllerRule
	}

	return nil
}

func generateCronJobRule(rule kyverno.Rule, controllers string, log logr.Logger) *kyvernoRule {
	logger := log.WithName("handleCronJob")

	hasCronJob := strings.Contains(controllers, PodControllerCronJob) || strings.Contains(controllers, "all")
	if !hasCronJob {
		return nil
	}

	logger.V(3).Info("generating rule for cronJob")
	jobRule := generateRuleForControllers(rule, "Job", logger)

	if jobRule == nil {
		return nil
	}

	cronJobRule := jobRule

	name := fmt.Sprintf("autogen-cronjob-%s", rule.Name)
	if len(name) > 63 {
		name = name[:63]
	}
	cronJobRule.Name = name

	if len(jobRule.MatchResources.Any) > 0 {
		rule := cronJobAnyAllAutogenRule(cronJobRule.MatchResources.Any)
		cronJobRule.MatchResources.Any = rule
	} else if len(jobRule.MatchResources.All) > 0 {
		rule := cronJobAnyAllAutogenRule(cronJobRule.MatchResources.All)
		cronJobRule.MatchResources.All = rule
	} else {
		cronJobRule.MatchResources.Kinds = []string{PodControllerCronJob}
	}

	if (jobRule.ExcludeResources) != nil && len(jobRule.ExcludeResources.Any) > 0 {
		rule := cronJobAnyAllAutogenRule(cronJobRule.ExcludeResources.Any)
		cronJobRule.ExcludeResources.Any = rule
	} else if (jobRule.ExcludeResources) != nil && len(jobRule.ExcludeResources.All) > 0 {
		rule := cronJobAnyAllAutogenRule(cronJobRule.ExcludeResources.All)
		cronJobRule.ExcludeResources.All = rule
	} else {
		if (jobRule.ExcludeResources) != nil && (len(jobRule.ExcludeResources.Kinds) > 0) {
			cronJobRule.ExcludeResources.Kinds = []string{PodControllerCronJob}
		}
	}

	if jobRule.Mutation != nil {
		if target := jobRule.Mutation.GetPatchStrategicMerge(); target != nil {
			newMutation := &kyverno.Mutation{}
			newMutation.SetPatchStrategicMerge(
				map[string]interface{}{
					"spec": map[string]interface{}{
						"jobTemplate": target,
					},
				},
			)
			cronJobRule.Mutation = newMutation.DeepCopy()
			return cronJobRule
		}
	}

	if jobRule.Validation != nil {
		if target := jobRule.Validation.GetPattern(); target != nil {
			newValidate := &kyverno.Validation{
				Message: variables.FindAndShiftReferences(log, rule.Validation.Message, "spec/jobTemplate/spec/template", "pattern"),
			}
			newValidate.SetPattern(
				map[string]interface{}{
					"spec": map[string]interface{}{
						"jobTemplate": target,
					},
				},
			)
			cronJobRule.Validation = newValidate.DeepCopy()
			return cronJobRule
		}

		if jobRule.Validation.Deny != nil {
			newValidate := &kyverno.Validation{
				Message: variables.FindAndShiftReferences(log, rule.Validation.Message, "spec/jobTemplate/spec/template", "pattern"),
				Deny:    jobRule.Validation.Deny,
			}
			cronJobRule.Validation = newValidate.DeepCopy()
			return cronJobRule
		}

		if target := jobRule.Validation.GetAnyPattern(); target != nil {
			var patterns []interface{}
			anyPatterns, err := jobRule.Validation.DeserializeAnyPattern()
			if err != nil {
				logger.Error(err, "failed to deserialize anyPattern, expect type array")
			}
			for _, pattern := range anyPatterns {
				newPattern := map[string]interface{}{
					"spec": map[string]interface{}{
						"jobTemplate": pattern,
					},
				}
				patterns = append(patterns, newPattern)
			}
			cronJobRule.Validation = &kyverno.Validation{
				Message: variables.FindAndShiftReferences(log, rule.Validation.Message, "spec/jobTemplate/spec/template", "anyPattern"),
			}
			cronJobRule.Validation.SetAnyPattern(patterns)
			return cronJobRule
		}

		if len(jobRule.Validation.ForEachValidation) > 0 && jobRule.Validation.ForEachValidation != nil {
			newForeachValidate := make([]*kyverno.ForEachValidation, len(jobRule.Validation.ForEachValidation))
			for i, foreach := range rule.Validation.ForEachValidation {
				newForeachValidate[i] = foreach
			}
			cronJobRule.Validation = &kyverno.Validation{
				Message:           variables.FindAndShiftReferences(log, rule.Validation.Message, "spec/template", "pattern"),
				ForEachValidation: newForeachValidate,
			}
			return cronJobRule
		}
	}

	if jobRule.Mutation != nil && len(jobRule.Mutation.ForEachMutation) > 0 && jobRule.Mutation.ForEachMutation != nil {
		var newForeachMutation []*kyverno.ForEachMutation
		for _, foreach := range jobRule.Mutation.ForEachMutation {
			temp := &kyverno.ForEachMutation{
				List:             foreach.List,
				Context:          foreach.Context,
				AnyAllConditions: foreach.AnyAllConditions,
			}
			temp.SetPatchStrategicMerge(
				map[string]interface{}{
					"spec": map[string]interface{}{
						"jobTemplate": foreach.GetPatchStrategicMerge(),
					},
				},
			)
			newForeachMutation = append(newForeachMutation, temp)
		}
		cronJobRule.Mutation = &kyverno.Mutation{
			ForEachMutation: newForeachMutation,
		}
		return cronJobRule
	}

	if jobRule.VerifyImages != nil {
		newVerifyImages := make([]*kyverno.ImageVerification, len(jobRule.VerifyImages))
		for i, vi := range rule.VerifyImages {
			newVerifyImages[i] = vi.DeepCopy()
		}
		cronJobRule.VerifyImages = newVerifyImages
		return cronJobRule
	}

	return nil
}

func updateGenRuleByte(pbyte []byte, kind string, genRule kyvernoRule) (obj []byte) {
	// TODO: do we need to unmarshall here ?
	if err := json.Unmarshal(pbyte, &genRule); err != nil {
		return obj
	}
	if kind == "Pod" {
		obj = []byte(strings.Replace(string(pbyte), "request.object.spec", "request.object.spec.template.spec", -1))
	}
	if kind == "Cronjob" {
		obj = []byte(strings.Replace(string(pbyte), "request.object.spec", "request.object.spec.jobTemplate.spec.template.spec", -1))
	}
	obj = []byte(strings.Replace(string(obj), "request.object.metadata", "request.object.spec.template.metadata", -1))
	return obj
}
