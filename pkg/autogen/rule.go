package autogen

import (
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

type generateResourceFilters func(kyverno.ResourceFilters, []string) kyverno.ResourceFilters

func generateRule(logger logr.Logger, name string, rule *kyverno.Rule, tplKey, shift string, kinds []string, grf generateResourceFilters) *kyverno.Rule {
	if rule == nil {
		return nil
	}
	rule = rule.DeepCopy()
	rule.Name = name
	// overwrite Kinds by pod controllers defined in the annotation
	if len(rule.MatchResources.Any) > 0 {
		rule.MatchResources.Any = grf(rule.MatchResources.Any, kinds)
	} else if len(rule.MatchResources.All) > 0 {
		rule.MatchResources.All = grf(rule.MatchResources.All, kinds)
	} else {
		rule.MatchResources.Kinds = kinds
	}
	if len(rule.ExcludeResources.Any) > 0 {
		rule.ExcludeResources.Any = grf(rule.ExcludeResources.Any, kinds)
	} else if len(rule.ExcludeResources.All) > 0 {
		rule.ExcludeResources.All = grf(rule.ExcludeResources.All, kinds)
	} else {
		if len(rule.ExcludeResources.Kinds) != 0 {
			rule.ExcludeResources.Kinds = kinds
		}
	}
	if target := rule.Mutation.GetPatchStrategicMerge(); target != nil {
		newMutation := kyverno.Mutation{}
		newMutation.SetPatchStrategicMerge(
			map[string]interface{}{
				"spec": map[string]interface{}{
					tplKey: target,
				},
			},
		)
		rule.Mutation = newMutation
		return rule
	}
	if len(rule.Mutation.ForEachMutation) > 0 && rule.Mutation.ForEachMutation != nil {
		var newForeachMutation []*kyverno.ForEachMutation
		for _, foreach := range rule.Mutation.ForEachMutation {
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
		rule.Mutation = kyverno.Mutation{
			ForEachMutation: newForeachMutation,
		}
		return rule
	}
	if target := rule.Validation.GetPattern(); target != nil {
		newValidate := kyverno.Validation{
			Message: variables.FindAndShiftReferences(logger, rule.Validation.Message, shift, "pattern"),
		}
		newValidate.SetPattern(
			map[string]interface{}{
				"spec": map[string]interface{}{
					tplKey: target,
				},
			},
		)
		rule.Validation = newValidate
		return rule
	}
	if rule.Validation.Deny != nil {
		deny := kyverno.Validation{
			Message: variables.FindAndShiftReferences(logger, rule.Validation.Message, shift, "deny"),
			Deny:    rule.Validation.Deny,
		}
		rule.Validation = deny
		return rule
	}
	if rule.Validation.GetAnyPattern() != nil {
		anyPatterns, err := rule.Validation.DeserializeAnyPattern()
		if err != nil {
			logger.Error(err, "failed to deserialize anyPattern, expect type array")
		}
		var patterns []interface{}
		for _, pattern := range anyPatterns {
			newPattern := map[string]interface{}{
				"spec": map[string]interface{}{
					tplKey: pattern,
				},
			}
			patterns = append(patterns, newPattern)
		}
		rule.Validation = kyverno.Validation{
			Message: variables.FindAndShiftReferences(logger, rule.Validation.Message, shift, "anyPattern"),
		}
		rule.Validation.SetAnyPattern(patterns)
		return rule
	}
	if len(rule.Validation.ForEachValidation) > 0 && rule.Validation.ForEachValidation != nil {
		newForeachValidate := make([]*kyverno.ForEachValidation, len(rule.Validation.ForEachValidation))
		for i, foreach := range rule.Validation.ForEachValidation {
			newForeachValidate[i] = foreach
		}
		rule.Validation = kyverno.Validation{
			Message:           variables.FindAndShiftReferences(logger, rule.Validation.Message, shift, "pattern"),
			ForEachValidation: newForeachValidate,
		}
		return rule
	}
	if rule.VerifyImages != nil {
		newVerifyImages := make([]*kyverno.ImageVerification, len(rule.VerifyImages))
		for i, vi := range rule.VerifyImages {
			newVerifyImages[i] = vi.DeepCopy()
		}
		rule.VerifyImages = newVerifyImages
		return rule
	}
	return nil
}

func getAutogenRuleName(prefix string, name string) string {
	name = prefix + "-" + name
	if len(name) > 63 {
		name = name[:63]
	}
	return name
}

func isAutogenRuleName(name string) bool {
	return strings.HasPrefix(name, "autogen-")
}

func getAnyAllAutogenRule(v kyverno.ResourceFilters, match string, kinds []string) kyverno.ResourceFilters {
	anyKind := v.DeepCopy()
	for i, value := range v {
		if kubeutils.ContainsKind(value.Kinds, match) {
			anyKind[i].Kinds = kinds
		}
	}
	return anyKind
}

func generateRuleForControllers(rule *kyverno.Rule, controllers string, log logr.Logger) *kyverno.Rule {
	logger := log.WithName("generateRuleForControllers")
	if isAutogenRuleName(rule.Name) || controllers == "" {
		logger.V(5).Info("skip generateRuleForControllers")
		return nil
	}
	logger.V(3).Info("processing rule", "rulename", rule.Name)
	match, exclude := rule.MatchResources, rule.ExcludeResources
	matchKinds, excludeKinds := match.GetKinds(), exclude.GetKinds()
	if !kubeutils.ContainsKind(matchKinds, "Pod") || (len(excludeKinds) != 0 && !kubeutils.ContainsKind(excludeKinds, "Pod")) {
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
		func(r kyverno.ResourceFilters, kinds []string) kyverno.ResourceFilters {
			return getAnyAllAutogenRule(r, "Pod", kinds)
		},
	)
}

func generateCronJobRule(rule *kyverno.Rule, controllers string, log logr.Logger) *kyverno.Rule {
	logger := log.WithName("generateCronJobRule")
	hasCronJob := strings.Contains(controllers, PodControllerCronJob) || strings.Contains(controllers, "all")
	if !hasCronJob {
		return nil
	}
	logger.V(3).Info("generating rule for cronJob")
	return generateRule(
		logger,
		getAutogenRuleName("autogen-cronjob", rule.Name),
		generateRuleForControllers(rule, controllers, log),
		"jobTemplate",
		"spec/jobTemplate/spec/template",
		[]string{PodControllerCronJob},
		func(r kyverno.ResourceFilters, kinds []string) kyverno.ResourceFilters {
			return getAnyAllAutogenRule(r, "Job", kinds)
		},
	)
}

func updateGenRuleByte(pbyte []byte, kind string) (obj []byte) {
	if kind == "Pod" {
		obj = []byte(strings.ReplaceAll(string(pbyte), "request.object.spec", "request.object.spec.template.spec"))
	}
	if kind == "Cronjob" {
		obj = []byte(strings.ReplaceAll(string(pbyte), "request.object.spec", "request.object.spec.jobTemplate.spec.template.spec"))
	}
	obj = []byte(strings.ReplaceAll(string(obj), "request.object.metadata", "request.object.spec.template.metadata"))
	return obj
}
