package autogen

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/utils"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
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
	ExcludeResources *kyverno.MatchResources      `json:"exclude,omitempty"`
	Context          *[]kyverno.ContextEntry      `json:"context,omitempty"`
	AnyAllConditions *apiextensions.JSON          `json:"preconditions,omitempty"`
	Mutation         *kyverno.Mutation            `json:"mutate,omitempty"`
	Validation       *kyverno.Validation          `json:"validate,omitempty"`
	VerifyImages     []*kyverno.ImageVerification `json:"verifyImages,omitempty" yaml:"verifyImages,omitempty"`
}

func createRule(rule *kyverno.Rule) *kyvernoRule {
	if rule == nil {
		return nil
	}
	jsonFriendlyStruct := kyvernoRule{
		Name: rule.Name,
	}
	if !reflect.DeepEqual(rule.MatchResources, kyverno.MatchResources{}) {
		jsonFriendlyStruct.MatchResources = rule.MatchResources.DeepCopy()
	}
	if !reflect.DeepEqual(rule.ExcludeResources, kyverno.MatchResources{}) {
		jsonFriendlyStruct.ExcludeResources = rule.ExcludeResources.DeepCopy()
	}
	if !reflect.DeepEqual(rule.Mutation, kyverno.Mutation{}) {
		jsonFriendlyStruct.Mutation = rule.Mutation.DeepCopy()
	}
	if !reflect.DeepEqual(rule.Validation, kyverno.Validation{}) {
		jsonFriendlyStruct.Validation = rule.Validation.DeepCopy()
	}
	kyvernoAnyAllConditions, _ := utils.ApiextensionsJsonToKyvernoConditions(rule.GetAnyAllConditions())
	switch typedAnyAllConditions := kyvernoAnyAllConditions.(type) {
	case kyverno.AnyAllConditions:
		if !reflect.DeepEqual(typedAnyAllConditions, kyverno.AnyAllConditions{}) {
			jsonFriendlyStruct.AnyAllConditions = rule.DeepCopy().RawAnyAllConditions
		}
	case []kyverno.Condition:
		if len(typedAnyAllConditions) > 0 {
			jsonFriendlyStruct.AnyAllConditions = rule.DeepCopy().RawAnyAllConditions
		}
	}
	if len(rule.Context) > 0 {
		jsonFriendlyStruct.Context = &rule.DeepCopy().Context
	}
	return &jsonFriendlyStruct
}

func createRuleMap(rules []kyverno.Rule) map[string]kyvernoRule {
	var ruleMap = make(map[string]kyvernoRule)
	for _, rule := range rules {
		ruleMap[rule.Name] = *createRule(&rule)
	}
	return ruleMap
}

type generateResourceFilters func(kyverno.ResourceFilters, []string) kyverno.ResourceFilters

func generateRule(logger logr.Logger, name string, r kyverno.Rule, tplKey, shift string, kinds []string, grf generateResourceFilters) *kyverno.Rule {
	autoRule := r.DeepCopy()
	autoRule.Name = name
	// overwrite Kinds by pod controllers defined in the annotation
	if len(autoRule.MatchResources.Any) > 0 {
		autoRule.MatchResources.Any = grf(autoRule.MatchResources.Any, kinds)
	} else if len(autoRule.MatchResources.All) > 0 {
		autoRule.MatchResources.All = grf(autoRule.MatchResources.All, kinds)
	} else {
		autoRule.MatchResources.Kinds = kinds
	}
	if len(autoRule.ExcludeResources.Any) > 0 {
		autoRule.ExcludeResources.Any = grf(autoRule.ExcludeResources.Any, kinds)
	} else if len(autoRule.ExcludeResources.All) > 0 {
		autoRule.ExcludeResources.All = grf(autoRule.ExcludeResources.All, kinds)
	} else {
		if len(autoRule.ExcludeResources.Kinds) != 0 {
			autoRule.ExcludeResources.Kinds = kinds
		}
	}
	if target := autoRule.Mutation.GetPatchStrategicMerge(); target != nil {
		newMutation := kyverno.Mutation{}
		newMutation.SetPatchStrategicMerge(
			map[string]interface{}{
				"spec": map[string]interface{}{
					tplKey: target,
				},
			},
		)
		autoRule.Mutation = newMutation
		return autoRule
	}
	if len(autoRule.Mutation.ForEachMutation) > 0 && autoRule.Mutation.ForEachMutation != nil {
		var newForeachMutation []*kyverno.ForEachMutation
		for _, foreach := range autoRule.Mutation.ForEachMutation {
			temp := kyverno.ForEachMutation{
				List:             foreach.List,
				Context:          foreach.Context,
				AnyAllConditions: foreach.AnyAllConditions,
			}
			temp.SetPatchStrategicMerge(
				map[string]interface{}{
					"spec": map[string]interface{}{
						tplKey: foreach.GetPatchStrategicMerge(),
					},
				},
			)
			newForeachMutation = append(newForeachMutation, &temp)
		}
		autoRule.Mutation = kyverno.Mutation{
			ForEachMutation: newForeachMutation,
		}
		return autoRule
	}
	if target := autoRule.Validation.GetPattern(); target != nil {
		newValidate := kyverno.Validation{
			Message: variables.FindAndShiftReferences(logger, autoRule.Validation.Message, shift, "pattern"),
		}
		newValidate.SetPattern(
			map[string]interface{}{
				"spec": map[string]interface{}{
					tplKey: target,
				},
			},
		)
		autoRule.Validation = newValidate
		return autoRule
	}
	if autoRule.Validation.Deny != nil {
		deny := kyverno.Validation{
			Message: variables.FindAndShiftReferences(logger, autoRule.Validation.Message, shift, "deny"),
			Deny:    autoRule.Validation.Deny,
		}
		autoRule.Validation = deny
		return autoRule
	}
	if autoRule.Validation.GetAnyPattern() != nil {
		anyPatterns, err := autoRule.Validation.DeserializeAnyPattern()
		if err != nil {
			logger.Error(err, "failed to deserialize anyPattern, expect type array")
		}
		patterns := validateAnyPattern(anyPatterns)
		autoRule.Validation = kyverno.Validation{
			Message: variables.FindAndShiftReferences(logger, autoRule.Validation.Message, shift, "anyPattern"),
		}
		autoRule.Validation.SetAnyPattern(patterns)
		return autoRule
	}
	if len(autoRule.Validation.ForEachValidation) > 0 && autoRule.Validation.ForEachValidation != nil {
		newForeachValidate := make([]*kyverno.ForEachValidation, len(autoRule.Validation.ForEachValidation))
		for i, foreach := range autoRule.Validation.ForEachValidation {
			newForeachValidate[i] = foreach
		}
		autoRule.Validation = kyverno.Validation{
			Message:           variables.FindAndShiftReferences(logger, autoRule.Validation.Message, shift, "pattern"),
			ForEachValidation: newForeachValidate,
		}
		return autoRule
	}
	if autoRule.VerifyImages != nil {
		newVerifyImages := make([]*kyverno.ImageVerification, len(autoRule.VerifyImages))
		for i, vi := range autoRule.VerifyImages {
			newVerifyImages[i] = vi.DeepCopy()
		}
		autoRule.VerifyImages = newVerifyImages
		return autoRule
	}
	return nil
}

func generateRuleForControllers(rule kyverno.Rule, controllers string, log logr.Logger) *kyverno.Rule {
	logger := log.WithName("generateRuleForControllers")
	if strings.HasPrefix(rule.Name, "autogen-") || controllers == "" {
		logger.V(5).Info("skip generateRuleForControllers")
		return nil
	}
	logger.V(3).Info("processing rule", "rulename", rule.Name)
	match, exclude := rule.MatchResources, rule.ExcludeResources
	matchKinds, excludeKinds := match.GetKinds(), exclude.GetKinds()
	if !kubeutils.ContainsKind(matchKinds, "Pod") ||
		(len(excludeKinds) != 0 && !kubeutils.ContainsKind(excludeKinds, "Pod")) {
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
	return generateRule(
		logger,
		getAutogenRuleName("autogen", rule.Name),
		rule,
		"template",
		"spec/template",
		strings.Split(controllers, ","),
		getAnyAllAutogenRule,
	)
}

func generateCronJobRule(rule kyverno.Rule, controllers string, log logr.Logger) *kyverno.Rule {
	logger := log.WithName("handleCronJob")
	hasCronJob := strings.Contains(controllers, PodControllerCronJob) || strings.Contains(controllers, "all")
	if !hasCronJob {
		return nil
	}
	logger.V(3).Info("generating rule for cronJob")
	autoRule := generateRuleForControllers(rule, controllers, log)
	if autoRule == nil {
		return nil
	}
	name := getAutogenRuleName("autogen-cronjob", rule.Name)
	return generateRule(
		logger,
		name,
		*autoRule,
		"jobTemplate",
		"spec/jobTemplate/spec/template",
		[]string{PodControllerCronJob},
		cronJobAnyAllAutogenRule,
	)
}

func updateGenRuleByte(pbyte []byte, kind string, genRule kyvernoRule) (obj []byte) {
	// TODO: do we need to unmarshall here ?
	if err := json.Unmarshal(pbyte, &genRule); err != nil {
		return obj
	}
	if kind == "Pod" {
		obj = []byte(strings.ReplaceAll(string(pbyte), "request.object.spec", "request.object.spec.template.spec"))
	}
	if kind == "Cronjob" {
		obj = []byte(strings.ReplaceAll(string(pbyte), "request.object.spec", "request.object.spec.jobTemplate.spec.template.spec"))
	}
	obj = []byte(strings.ReplaceAll(string(obj), "request.object.metadata", "request.object.spec.template.metadata"))
	return obj
}
