package v1

import (
	"bytes"
	"sort"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
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
	Name                   string                                    `json:"name"`
	MatchResources         *kyvernov1.MatchResources                 `json:"match"`
	ExcludeResources       *kyvernov1.MatchResources                 `json:"exclude,omitempty"`
	CELPreconditions       *[]admissionregistrationv1.MatchCondition `json:"celPreconditions,omitempty"`
	Context                *[]kyvernov1.ContextEntry                 `json:"context,omitempty"`
	AnyAllConditions       *kyvernov1.ConditionsWrapper              `json:"preconditions,omitempty"`
	Mutation               *kyvernov1.Mutation                       `json:"mutate,omitempty"`
	Validation             *kyvernov1.Validation                     `json:"validate,omitempty"`
	VerifyImages           []kyvernov1.ImageVerification             `json:"verifyImages,omitempty"`
	SkipBackgroundRequests *bool                                     `json:"skipBackgroundRequests,omitempty"`
}

func createRule(rule *kyvernov1.Rule) *kyvernoRule {
	if rule == nil {
		return nil
	}
	jsonFriendlyStruct := kyvernoRule{
		Name:                   rule.Name,
		VerifyImages:           rule.VerifyImages,
		SkipBackgroundRequests: rule.SkipBackgroundRequests,
	}
	if !datautils.DeepEqual(rule.MatchResources, kyvernov1.MatchResources{}) {
		jsonFriendlyStruct.MatchResources = rule.MatchResources.DeepCopy()
	}
	if rule.ExcludeResources != nil && !datautils.DeepEqual(*rule.ExcludeResources, kyvernov1.MatchResources{}) {
		jsonFriendlyStruct.ExcludeResources = rule.ExcludeResources.DeepCopy()
	}
	if rule.Mutation != nil && !datautils.DeepEqual(*rule.Mutation, kyvernov1.Mutation{}) {
		jsonFriendlyStruct.Mutation = rule.Mutation.DeepCopy()
	}
	if rule.Validation != nil && !datautils.DeepEqual(*rule.Validation, kyvernov1.Validation{}) {
		jsonFriendlyStruct.Validation = rule.Validation.DeepCopy()
	}
	kyvernoAnyAllConditions := rule.GetAnyAllConditions()
	switch typedAnyAllConditions := kyvernoAnyAllConditions.(type) {
	case kyvernov1.AnyAllConditions:
		if !datautils.DeepEqual(typedAnyAllConditions, kyvernov1.AnyAllConditions{}) {
			jsonFriendlyStruct.AnyAllConditions = rule.DeepCopy().RawAnyAllConditions
		}
	case []kyvernov1.Condition:
		if len(typedAnyAllConditions) > 0 {
			jsonFriendlyStruct.AnyAllConditions = rule.DeepCopy().RawAnyAllConditions
		}
	}
	if len(rule.Context) > 0 {
		jsonFriendlyStruct.Context = &rule.DeepCopy().Context
	}
	if len(rule.CELPreconditions) > 0 {
		jsonFriendlyStruct.CELPreconditions = &rule.DeepCopy().CELPreconditions
	}
	return &jsonFriendlyStruct
}

type generateResourceFilters func(kyvernov1.ResourceFilters, []string) kyvernov1.ResourceFilters

func generateRule(name string, rule *kyvernov1.Rule, tplKey, shift string, kinds []string, grf generateResourceFilters) *kyvernov1.Rule {
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
	if rule.ExcludeResources != nil {
		if len(rule.ExcludeResources.Any) > 0 {
			rule.ExcludeResources.Any = grf(rule.ExcludeResources.Any, kinds)
		} else if len(rule.ExcludeResources.All) > 0 {
			rule.ExcludeResources.All = grf(rule.ExcludeResources.All, kinds)
		} else {
			if len(rule.ExcludeResources.Kinds) != 0 {
				rule.ExcludeResources.Kinds = kinds
			}
		}
	}
	if rule.Mutation != nil {
		if target := rule.Mutation.GetPatchStrategicMerge(); target != nil {
			newMutation := &kyvernov1.Mutation{}
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
			var newForEachMutation []kyvernov1.ForEachMutation
			for _, foreach := range rule.Mutation.ForEachMutation {
				temp := kyvernov1.ForEachMutation{
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
				newForEachMutation = append(newForEachMutation, temp)
			}
			rule.Mutation = &kyvernov1.Mutation{
				ForEachMutation: newForEachMutation,
			}
			return rule
		}
	}
	if rule.Validation != nil {
		if target := rule.Validation.GetPattern(); target != nil {
			newValidate := &kyvernov1.Validation{
				Message:                 variables.FindAndShiftReferences(logger, rule.Validation.Message, shift, "pattern"),
				FailureAction:           rule.Validation.FailureAction,
				FailureActionOverrides:  rule.Validation.FailureActionOverrides,
				AllowExistingViolations: rule.Validation.AllowExistingViolations,
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
			deny := &kyvernov1.Validation{
				Message:                 variables.FindAndShiftReferences(logger, rule.Validation.Message, shift, "deny"),
				Deny:                    rule.Validation.Deny,
				FailureAction:           rule.Validation.FailureAction,
				FailureActionOverrides:  rule.Validation.FailureActionOverrides,
				AllowExistingViolations: rule.Validation.AllowExistingViolations,
			}
			rule.Validation = deny
			return rule
		}
		if rule.Validation.PodSecurity != nil {
			newExclude := make([]kyvernov1.PodSecurityStandard, len(rule.Validation.PodSecurity.Exclude))
			copy(newExclude, rule.Validation.PodSecurity.Exclude)
			podSecurity := &kyvernov1.Validation{
				Message: variables.FindAndShiftReferences(logger, rule.Validation.Message, shift, "podSecurity"),
				PodSecurity: &kyvernov1.PodSecurity{
					Level:   rule.Validation.PodSecurity.Level,
					Version: rule.Validation.PodSecurity.Version,
					Exclude: newExclude,
				},
				FailureAction:           rule.Validation.FailureAction,
				FailureActionOverrides:  rule.Validation.FailureActionOverrides,
				AllowExistingViolations: rule.Validation.AllowExistingViolations,
			}
			rule.Validation = podSecurity
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
			failureAction := rule.Validation.FailureAction
			failureActionOverrides := rule.Validation.FailureActionOverrides
			rule.Validation = &kyvernov1.Validation{
				Message:                 variables.FindAndShiftReferences(logger, rule.Validation.Message, shift, "anyPattern"),
				FailureAction:           failureAction,
				FailureActionOverrides:  failureActionOverrides,
				AllowExistingViolations: rule.Validation.AllowExistingViolations,
			}
			rule.Validation.SetAnyPattern(patterns)
			return rule
		}
		if len(rule.Validation.ForEachValidation) > 0 && rule.Validation.ForEachValidation != nil {
			newForeachValidate := make([]kyvernov1.ForEachValidation, len(rule.Validation.ForEachValidation))
			copy(newForeachValidate, rule.Validation.ForEachValidation)
			failureAction := rule.Validation.FailureAction
			failureActionOverrides := rule.Validation.FailureActionOverrides
			rule.Validation = &kyvernov1.Validation{
				Message:                 variables.FindAndShiftReferences(logger, rule.Validation.Message, shift, "pattern"),
				ForEachValidation:       newForeachValidate,
				FailureAction:           failureAction,
				FailureActionOverrides:  failureActionOverrides,
				AllowExistingViolations: rule.Validation.AllowExistingViolations,
			}
			return rule
		}
		if rule.HasValidateCEL() {
			cel := rule.Validation.CEL.DeepCopy()
			rule.Validation.CEL = cel
			return rule
		}
		if rule.HasValidateAssert() {
			rule.Validation.Assert = createAutogenAssertion(*rule.Validation.Assert.DeepCopy(), tplKey)
			return rule
		}
	}
	if rule.VerifyImages != nil {
		newVerifyImages := make([]kyvernov1.ImageVerification, len(rule.VerifyImages))
		for i, vi := range rule.VerifyImages {
			newVerifyImages[i] = *vi.DeepCopy()
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

func getAnyAllAutogenRule(v kyvernov1.ResourceFilters, match string, kinds []string) kyvernov1.ResourceFilters {
	anyKind := v.DeepCopy()
	for i, value := range v {
		if kubeutils.ContainsKind(value.Kinds, match) {
			anyKind[i].Kinds = kinds
		}
	}
	return anyKind
}

func generateRuleForControllers(rule *kyvernov1.Rule, controllers string) *kyvernov1.Rule {
	if isAutogenRuleName(rule.Name) || controllers == "" {
		debug.Info("skip generateRuleForControllers")
		return nil
	}
	debug.Info("processing rule", "rulename", rule.Name)
	match := rule.MatchResources
	matchKinds := match.GetKinds()
	var excludeKinds []string
	if exclude := rule.ExcludeResources; exclude != nil {
		excludeKinds = exclude.GetKinds()
	}
	if !kubeutils.ContainsKind(matchKinds, "Pod") || (len(excludeKinds) != 0 && !kubeutils.ContainsKind(excludeKinds, "Pod")) {
		return nil
	}
	// Support backwards compatibility
	skipAutoGeneration := false
	var controllersValidated []string
	if controllers == "all" {
		skipAutoGeneration = true
	} else if controllers != "none" && controllers != "all" {
		controllersList := map[string]int{
			"DaemonSet":             1,
			"Deployment":            1,
			"Job":                   1,
			"StatefulSet":           1,
			"ReplicaSet":            1,
			"ReplicationController": 1,
		}
		for _, value := range splitKinds(controllers, ",") {
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
			controllers = "DaemonSet,Deployment,Job,StatefulSet,ReplicaSet,ReplicationController"
		} else {
			controllers = strings.Join(controllersValidated, ",")
		}
	}
	return generateRule(
		getAutogenRuleName("autogen", rule.Name),
		rule,
		"template",
		"spec/template",
		splitKinds(controllers, ","),
		func(r kyvernov1.ResourceFilters, kinds []string) kyvernov1.ResourceFilters {
			return getAnyAllAutogenRule(r, "Pod", kinds)
		},
	)
}

func splitKinds(controllers, separator string) []string {
	kinds := strings.Split(controllers, separator)
	sort.Strings(kinds)
	return kinds
}

func generateCronJobRule(rule *kyvernov1.Rule, controllers string) *kyvernov1.Rule {
	hasCronJob := strings.Contains(controllers, PodControllerCronJob) || strings.Contains(controllers, "all")
	if !hasCronJob {
		return nil
	}
	debug.Info("generating rule for cronJob")
	return generateRule(
		getAutogenRuleName("autogen-cronjob", rule.Name),
		generateRuleForControllers(rule, controllers),
		"jobTemplate",
		"spec/jobTemplate/spec/template",
		[]string{PodControllerCronJob},
		func(r kyvernov1.ResourceFilters, kinds []string) kyvernov1.ResourceFilters {
			anyKind := r.DeepCopy()
			for i := range anyKind {
				anyKind[i].Kinds = kinds
			}
			return anyKind
		},
	)
}

var (
	podReplacementRules [][2][]byte = [][2][]byte{
		{[]byte("request.object.spec"), []byte("request.object.spec.template.spec")},
		{[]byte("request.oldObject.spec"), []byte("request.oldObject.spec.template.spec")},
		{[]byte("request.object.metadata"), []byte("request.object.spec.template.metadata")},
		{[]byte("request.oldObject.metadata"), []byte("request.oldObject.spec.template.metadata")},
	}
	podCELReplacementRules [][2][]byte = [][2][]byte{
		{[]byte("object.spec"), []byte("object.spec.template.spec")},
		{[]byte("oldObject.spec"), []byte("oldObject.spec.template.spec")},
		{[]byte("object.metadata"), []byte("object.spec.template.metadata")},
		{[]byte("oldObject.metadata"), []byte("oldObject.spec.template.metadata")},
	}
	cronJobReplacementRules [][2][]byte = [][2][]byte{
		{[]byte("request.object.spec"), []byte("request.object.spec.jobTemplate.spec.template.spec")},
		{[]byte("request.oldObject.spec"), []byte("request.oldObject.spec.jobTemplate.spec.template.spec")},
		{[]byte("request.object.metadata"), []byte("request.object.spec.jobTemplate.spec.template.metadata")},
		{[]byte("request.oldObject.metadata"), []byte("request.oldObject.spec.jobTemplate.spec.template.metadata")},
	}
	cronJobCELReplacementRules [][2][]byte = [][2][]byte{
		{[]byte("object.spec"), []byte("object.spec.jobTemplate.spec.template.spec")},
		{[]byte("oldObject.spec"), []byte("oldObject.spec.jobTemplate.spec.template.spec")},
		{[]byte("object.metadata"), []byte("object.spec.jobTemplate.spec.template.metadata")},
		{[]byte("oldObject.metadata"), []byte("oldObject.spec.jobTemplate.spec.template.metadata")},
	}
)

func updateFields(data []byte, kind string, cel bool) []byte {
	switch kind {
	case "Pod":
		if cel {
			for _, replacement := range podCELReplacementRules {
				data = bytes.ReplaceAll(data, replacement[0], replacement[1])
			}
		} else {
			for _, replacement := range podReplacementRules {
				data = bytes.ReplaceAll(data, replacement[0], replacement[1])
			}
		}
	case "Cronjob":
		if cel {
			for _, replacement := range cronJobCELReplacementRules {
				data = bytes.ReplaceAll(data, replacement[0], replacement[1])
			}
		} else {
			for _, replacement := range cronJobReplacementRules {
				data = bytes.ReplaceAll(data, replacement[0], replacement[1])
			}
		}
	}

	return data
}
