package matching

import (
	"testing"

	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func TestExpandPodSubresources(t *testing.T) {
	match := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"pods"},
				},
			},
		}},
		ExcludeResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{"*"},
					APIVersions: []string{"*"},
					Resources:   []string{"pods", "pods/ephemeralcontainers"},
				},
			},
		}},
	}

	expanded := ExpandPodSubresources(match)

	assert.Equal(t, []string{"pods"}, match.ResourceRules[0].Resources)
	assert.Equal(t, []string{"pods", "pods/ephemeralcontainers"}, expanded.ResourceRules[0].Resources)
	assert.Equal(t, []string{"pods", "pods/ephemeralcontainers"}, expanded.ExcludeResourceRules[0].Resources)
}

func TestExpandPodSubresourcesIgnoresNonPodRules(t *testing.T) {
	match := &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{"apps"},
					APIVersions: []string{"v1"},
					Resources:   []string{"deployments"},
				},
			},
		}},
	}

	assert.Equal(t, match, ExpandPodSubresources(match))
}
