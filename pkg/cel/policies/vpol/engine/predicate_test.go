package engine

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makePolicy(name, namespace string) *policiesv1beta1.ValidatingPolicy {
	return &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func TestMatchNames(t *testing.T) {
	tests := []struct {
		name       string
		names      []string
		policyName string
		want       bool
	}{{
		name:       "no names matches everything",
		names:      nil,
		policyName: "any-policy",
		want:       true,
	}, {
		name:       "empty names matches everything",
		names:      []string{},
		policyName: "any-policy",
		want:       true,
	}, {
		name:       "single name matches",
		names:      []string{"my-policy"},
		policyName: "my-policy",
		want:       true,
	}, {
		name:       "single name does not match",
		names:      []string{"my-policy"},
		policyName: "other-policy",
		want:       false,
	}, {
		name:       "multiple names matches first",
		names:      []string{"policy-a", "policy-b"},
		policyName: "policy-a",
		want:       true,
	}, {
		name:       "multiple names matches second",
		names:      []string{"policy-a", "policy-b"},
		policyName: "policy-b",
		want:       true,
	}, {
		name:       "multiple names no match",
		names:      []string{"policy-a", "policy-b"},
		policyName: "policy-c",
		want:       false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred := MatchNames(tt.names...)
			got := pred(makePolicy(tt.policyName, ""))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClusteredPolicy(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		want      bool
	}{{
		name:      "cluster-scoped policy",
		namespace: "",
		want:      true,
	}, {
		name:      "namespaced policy",
		namespace: "default",
		want:      false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred := ClusteredPolicy()
			got := pred(makePolicy("test", tt.namespace))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNamespacedPolicy(t *testing.T) {
	tests := []struct {
		name            string
		filterNamespace string
		policyNamespace string
		want            bool
	}{{
		name:            "matching namespace",
		filterNamespace: "default",
		policyNamespace: "default",
		want:            true,
	}, {
		name:            "different namespace",
		filterNamespace: "default",
		policyNamespace: "kube-system",
		want:            false,
	}, {
		name:            "cluster-scoped does not match",
		filterNamespace: "default",
		policyNamespace: "",
		want:            false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred := NamespacedPolicy(tt.filterNamespace)
			got := pred(makePolicy("test", tt.policyNamespace))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAnd(t *testing.T) {
	alwaysTrue := func(policiesv1beta1.ValidatingPolicyLike) bool { return true }
	alwaysFalse := func(policiesv1beta1.ValidatingPolicyLike) bool { return false }

	tests := []struct {
		name       string
		conditions []Predicate
		want       bool
	}{{
		name:       "no conditions always true",
		conditions: nil,
		want:       true,
	}, {
		name:       "single true",
		conditions: []Predicate{alwaysTrue},
		want:       true,
	}, {
		name:       "single false",
		conditions: []Predicate{alwaysFalse},
		want:       false,
	}, {
		name:       "all true",
		conditions: []Predicate{alwaysTrue, alwaysTrue},
		want:       true,
	}, {
		name:       "first false short circuits",
		conditions: []Predicate{alwaysFalse, alwaysTrue},
		want:       false,
	}, {
		name:       "last false",
		conditions: []Predicate{alwaysTrue, alwaysFalse},
		want:       false,
	}, {
		name:       "combined name and namespace match",
		conditions: []Predicate{MatchNames("my-policy"), ClusteredPolicy()},
		want:       true,
	}, {
		name:       "combined name matches but namespace does not",
		conditions: []Predicate{MatchNames("my-policy"), NamespacedPolicy("default")},
		want:       false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pred := And(tt.conditions...)
			got := pred(makePolicy("my-policy", ""))
			assert.Equal(t, tt.want, got)
		})
	}
}
