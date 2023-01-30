package common

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

// Info stores the policy application results for all matched resources
// Namespace is set to empty "" if resource is cluster wide resource
type Info struct {
	PolicyName string
	Namespace  string
	Results    []EngineResponseResult
}

type EngineResponseResult struct {
	Resource engineapi.ResourceSpec
	Rules    []kyvernov1.ViolatedRule
}
