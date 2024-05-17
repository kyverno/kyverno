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
	typeConditions, err := utils.TransformConditions(anyAllConditions)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse preconditions: %w", err)
	}

	return variables.EvaluateConditions(logger, jsonContext, typeConditions)
}

func CheckDenyPreconditions(logger logr.Logger, jsonContext enginecontext.Interface, anyAllConditions apiextensions.JSON) (bool, string, error) {
	typeConditions, err := utils.TransformConditions(anyAllConditions)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse deny conditions: %w", err)
	}

	return variables.EvaluateConditions(logger, jsonContext, typeConditions)
}
