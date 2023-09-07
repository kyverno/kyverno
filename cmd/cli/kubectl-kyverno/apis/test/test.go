package test

import (
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/values"
)

type Test struct {
	Name      string         `json:"name"`
	Policies  []string       `json:"policies"`
	Resources []string       `json:"resources"`
	Variables string         `json:"variables,omitempty"`
	UserInfo  string         `json:"userinfo,omitempty"`
	Results   []TestResults  `json:"results"`
	Values    *values.Values `json:"values,omitempty"`
}
