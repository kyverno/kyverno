package internal

import (
	"fmt"

	"github.com/go-logr/logr"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
)

func CheckPreconditions(logger logr.Logger, jsonContext enginecontext.Interface, anyAllConditions apiextensions.JSON) (bool, string, error) {
	preconditions, err := variables.SubstituteAllInPreconditions(logger, jsonContext, anyAllConditions)
	if err != nil {
		return false, "", fmt.Errorf("failed to substitute variables in preconditions: %w", err)
	}
	typeConditions, err := utils.TransformConditions(preconditions)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse preconditions: %w", err)
	}

	val, msg := variables.EvaluateConditions(logger, jsonContext, typeConditions)
	return val, msg, nil
}

func CheckDenyPreconditions(logger logr.Logger, jsonContext enginecontext.Interface, anyAllConditions apiextensions.JSON) (bool, string, error) {
	preconditions, err := variables.SubstituteAll(logger, jsonContext, anyAllConditions)
	if err != nil {
		return false, "", fmt.Errorf("failed to substitute variables in deny conditions: %w", err)
	}
	typeConditions, err := utils.TransformConditions(preconditions)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse deny conditions: %w", err)
	}

	val, msg := variables.EvaluateConditions(logger, jsonContext, typeConditions)
	return val, msg, nil
}
