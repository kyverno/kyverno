package policyreport

import (
	"reflect"
	"strconv"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
)

// Info stores the policy application results for all matched resources
// Namespace is set to empty "" if resource is cluster wide resource
type Info struct {
	PolicyName string
	Namespace  string
	Results    []EngineResponseResult
}

type EngineResponseResult struct {
	Resource response.ResourceSpec
	Rules    []kyvernov1.ViolatedRule
}

func (i Info) ToKey() string {
	keys := []string{
		i.PolicyName,
		i.Namespace,
		strconv.Itoa(len(i.Results)),
	}

	for _, result := range i.Results {
		keys = append(keys, result.Resource.GetKey())
	}
	return strings.Join(keys, "/")
}

func (i Info) GetRuleLength() int {
	l := 0
	for _, res := range i.Results {
		l += len(res.Rules)
	}
	return l
}

func (info Info) isResourceDeletion() bool {
	return info.PolicyName == "" && len(info.Results) == 1 && info.GetRuleLength() == 0
}

func (info Info) isPolicyDeletion() bool {
	return info.PolicyName != "" && len(info.Results) == 0
}

func (info Info) isRuleDeletion() bool {
	if info.PolicyName != "" && len(info.Results) == 1 {
		result := info.Results[0]
		if len(result.Rules) == 1 && reflect.DeepEqual(result.Resource, response.ResourceSpec{}) {
			return true
		}
	}
	return false
}
