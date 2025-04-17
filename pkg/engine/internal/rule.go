package internal

import (
	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/variables"
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
