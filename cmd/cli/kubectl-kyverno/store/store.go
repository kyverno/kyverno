package store

import (
	"github.com/kyverno/kyverno/pkg/registryclient"
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

var (
	local          bool
	registryClient registryclient.Client
	allowApiCalls  bool
	policies       []Policy
	foreachElement int
)

// SetLocal sets local (clusterless) execution for the CLI
func SetLocal(m bool) {
	local = m
}

// IsLocal returns 'true' if the CLI is in local (clusterless) execution
func IsLocal() bool {
	return local
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
				switch ruleName {
				case rule.Name, "autogen-" + rule.Name, "autogen-cronjob-" + rule.Name:
					return &rule
				}
			}
		}
	}
	return nil
}

func AllowApiCall(allow bool) {
	allowApiCalls = allow
}

func IsApiCallAllowed() bool {
	return allowApiCalls
}
