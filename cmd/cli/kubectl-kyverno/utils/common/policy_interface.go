package common

import (
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

type PolicyInterface interface {
	ApplyPolicyOnResource(c ApplyPolicyConfig) ([]engineapi.EngineResponse, error)
}
