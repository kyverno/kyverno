package internal

import (
	"fmt"

	"github.com/go-logr/logr"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
)

func CheckPreconditions(logger logr.Logger, ctx engineapi.PolicyContext, anyAllConditions apiextensions.JSON) (bool, error) {
	preconditions, err := variables.SubstituteAllInPreconditions(logger, ctx.JSONContext(), anyAllConditions)
	if err != nil {
		return false, fmt.Errorf("failed to substitute variables in preconditions: %w", err)
	}
	typeConditions, err := utils.TransformConditions(preconditions)
	if err != nil {
		return false, fmt.Errorf("failed to parse preconditions: %w", err)
	}
	return variables.EvaluateConditions(logger, ctx.JSONContext(), typeConditions), nil
}

func CheckDenyPreconditions(logger logr.Logger, ctx engineapi.PolicyContext, anyAllConditions apiextensions.JSON) (bool, error) {
	preconditions, err := variables.SubstituteAll(logger, ctx.JSONContext(), anyAllConditions)
	if err != nil {
		return false, fmt.Errorf("failed to substitute variables in deny preconditions: %w", err)
	}
	typeConditions, err := utils.TransformConditions(preconditions)
	if err != nil {
		return false, fmt.Errorf("failed to parse deny preconditions: %w", err)
	}
	return variables.EvaluateConditions(logger, ctx.JSONContext(), typeConditions), nil
}
