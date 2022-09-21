package policyreport

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
)

// Info stores the policy application results for all matched resources
// Namespace is set to empty "" if resource is cluster wide resource
type Info struct {
	Namespace string
	Resource  response.ResourceSpec
	Results   map[string]EngineResponseResult
}

type EngineResponseResult struct {
	Rules []kyvernov1.ViolatedRule
}
