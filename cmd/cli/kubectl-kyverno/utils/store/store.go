package store

import (
	"github.com/kyverno/kyverno/pkg/registryclient"
	rbacv1 "k8s.io/api/rbac/v1"
)

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
	ForEachValues map[string][]interface{} `json:"foreachValues"`
}

type Subject struct {
	Subject rbacv1.Subject `json:"subject,omitempty" yaml:"subject,omitempty"`
}

var (
	mock           bool
	registryClient registryclient.Client
	allowApiCalls  bool
	policies       []Policy
	// contextVar     Context
	foreachElement int
	subject        rbacv1.Subject
)

func SetMock(m bool) {
	mock = m
}

func IsMock() bool {
	return mock
}

func SetForEachElement(element int) {
	foreachElement = element
}

func GetForeachElement() int {
	return foreachElement
}

func SetRegistryAccess(access bool) {
	if access {
		registryClient = registryclient.NewOrDie(registryclient.WithLocalKeychain())
	}
}

func GetRegistryAccess() bool {
	return registryClient != nil
}

func GetRegistryClient() registryclient.Client {
	return registryClient
}

func SetPolicies(p ...Policy) {
	policies = p
}

func HasPolicies() bool {
	return len(policies) != 0
}

func GetPolicy(policyName string) *Policy {
	for _, policy := range policies {
		if policy.Name == policyName {
			return &policy
		}
	}
	return nil
}

func GetPolicyRule(policyName string, ruleName string) *Rule {
	for _, policy := range policies {
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

func SetSubject(s rbacv1.Subject) {
	subject = s
}

func GetSubject() rbacv1.Subject {
	return subject
}

func AllowApiCall(allow bool) {
	allowApiCalls = allow
}

func IsApiCallAllowed() bool {
	return allowApiCalls
}
