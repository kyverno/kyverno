package store

import (
	"github.com/kyverno/kyverno/pkg/engine/context/loaders"
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
	Name          string                     `json:"name"`
	Values        map[string]interface{}     `json:"values"`
	ForEachValues []map[string][]interface{} `json:"foreachValues"`
}

type Store struct {
	local          bool
	registryClient registryclient.Client
	allowApiCalls  bool
	policies       []Policy
	gctxStore      loaders.Store
}

// SetLocal sets local (clusterless) execution for the CLI
func (s *Store) SetLocal(m bool) {
	s.local = m
}

// IsLocal returns 'true' if the CLI is in local (clusterless) execution
func (s *Store) IsLocal() bool {
	return s.local
}

func (s *Store) SetRegistryAccess(access bool) {
	if access {
		s.registryClient = registryclient.NewOrDie(registryclient.WithLocalKeychain())
	}
}

func (s *Store) GetRegistryAccess() bool {
	return s.registryClient != nil
}

func (s *Store) GetRegistryClient() registryclient.Client {
	return s.registryClient
}

func (s *Store) SetPolicies(p ...Policy) {
	s.policies = p
}

func (s *Store) HasPolicies() bool {
	return len(s.policies) != 0
}

func (s *Store) GetPolicy(policyName string) *Policy {
	for _, policy := range s.policies {
		if policy.Name == policyName {
			return &policy
		}
	}
	return nil
}

func (s *Store) GetPolicyRule(policyName string, ruleName string) *Rule {
	for _, policy := range s.policies {
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

func (s *Store) AllowApiCall(allow bool) {
	s.allowApiCalls = allow
}

func (s *Store) IsApiCallAllowed() bool {
	return s.allowApiCalls
}

func (s *Store) SetGlobalContextStore(store loaders.Store) {
	s.gctxStore = store
}

func (s *Store) GetGlobalContextStore() loaders.Store {
	return s.gctxStore
}
