package internal

import (
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/engine/variables/regex"
)

func SubstitutePropertiesInRule(log logr.Logger, rule *kyvernov1.Rule, jsonContext enginecontext.Interface) error {
	if len(rule.ReportProperties) == 0 {
		return nil
	}
	properties := rule.ReportProperties
	updatedProperties, err := variables.SubstituteAllInType(log, jsonContext, &properties)
	if err != nil {
		return err
	}
	rule.ReportProperties = *updatedProperties
	return nil
}

func SubstituteImageVerifyVariables(rule kyvernov1.Rule, ctx enginecontext.EvalInterface, logger logr.Logger) (*kyvernov1.Rule, error) {
	hasValidateImageVerification := rule.HasValidateImageVerification()
	ruleCopy := *rule.DeepCopy()
	for i := range ruleCopy.VerifyImages {
		for j := range ruleCopy.VerifyImages[i].Attestations {
			ruleCopy.VerifyImages[i].Attestations[j].Conditions = nil
		}
		if hasValidateImageVerification {
			ruleCopy.VerifyImages[i].Validation.Message = ""
			ruleCopy.VerifyImages[i].Validation.Deny.RawAnyAllConditions = nil
		}
	}

	var err error
	ruleCopy, err = variables.SubstituteAllInRule(logger, ctx, ruleCopy)
	if err != nil {
		return nil, err
	}

	for i := range ruleCopy.VerifyImages {
		for j := range ruleCopy.VerifyImages[i].Attestations {
			ruleCopy.VerifyImages[i].Attestations[j].Conditions = rule.VerifyImages[i].Attestations[j].Conditions
		}
		if hasValidateImageVerification {
			ruleCopy.VerifyImages[i].Validation.Message = rule.VerifyImages[i].Validation.Message
			ruleCopy.VerifyImages[i].Validation.Deny.RawAnyAllConditions = rule.VerifyImages[i].Validation.Deny.RawAnyAllConditions
		}
	}
	return &ruleCopy, nil
}

func ImageReferencesHasVariables(rule kyvernov1.Rule) bool {
	for _, v := range rule.VerifyImages {
		v = *v.Convert()
		for _, ref := range v.ImageReferences {
			if regex.IsVariable(ref) || regex.IsReference(ref) {
				return true
			}
		}
	}
	return false
}
