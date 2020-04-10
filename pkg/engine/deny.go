package engine

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/nirmata/kyverno/pkg/engine/context"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/variables"
)

func Deny(logger logr.Logger, policy v1.ClusterPolicy, ctx *context.Context) error {
	for _, rule := range policy.Spec.Rules {
		if rule.Deny != nil {
			sliceCopy := make([]v1.Condition, len(rule.Deny.Conditions))
			copy(sliceCopy, rule.Deny.Conditions)

			if !variables.EvaluateConditions(logger, ctx, sliceCopy) {
				return fmt.Errorf("request has been denied by policy %s due to - %s", policy.Name, rule.Deny.Message)
			}
		}
	}

	return nil
}
