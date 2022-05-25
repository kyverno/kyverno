package store

import (
	"github.com/kyverno/kyverno/pkg/registryclient"
	rbacv1 "k8s.io/api/rbac/v1"
)

var (
	Mock, RegistryAccess bool
	ContextVar           Context
	ForeachElement       int
	Subjects             Subject
)

func SetMock(mock bool) {
	Mock = mock
}

func GetMock() bool {
	return Mock
}

func SetForeachElement(foreachElement int) {
	ForeachElement = foreachElement
}

func GetForeachElement() int {
	return ForeachElement
}

func SetRegistryAccess(access bool) {
	if access {
		registryclient.DefaultClient.UseLocalKeychain()
	}
	RegistryAccess = access
}

func GetRegistryAccess() bool {
	return RegistryAccess
}

func SetContext(context Context) {
	ContextVar = context
}

func GetContext() Context {
	return ContextVar
}

func GetPolicyFromContext(policyName string) *Policy {
	for _, policy := range ContextVar.Policies {
		if policy.Name == policyName {
			return &policy
		}
	}
	return nil
}

func GetPolicyRuleFromContext(policyName string, ruleName string) *Rule {
	for _, policy := range ContextVar.Policies {
		if policy.Name == policyName {
			for _, rule := range policy.Rules {
				if rule.Name == ruleName {
					return &rule
				}
			}
		}
	}
	return nil
}

type Context struct {
	Policies []Policy `json:"policies"`
}

type Policy struct {
	Name  string `json:"name"`
	Rules []Rule `json:"rules"`
}

type Rule struct {
	Name          string                   `json:"name"`
	Values        map[string]interface{}   `json:"values"`
	ForeachValues map[string][]interface{} `json:"foreachValues"`
}

func SetSubjects(subjects Subject) {
	Subjects = subjects
}

func GetSubjects() Subject {
	return Subjects
}

type Subject struct {
	Subject rbacv1.Subject `json:"subject,omitempty" yaml:"subject,omitempty"`
}
