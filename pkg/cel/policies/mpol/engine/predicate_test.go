package engine

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMatchNames(t *testing.T) {
	tests := []struct {
		name     string
		names    []string
		policies []policiesv1beta1.MutatingPolicyLike
		expected []bool
	}{
		{
			name:     "no names provided - always match",
			names:    []string{},
			policies: []policiesv1beta1.MutatingPolicyLike{&policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "p1"}}, &policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "any"}}},
			expected: []bool{true, true},
		},
		{
			name:     "single name match",
			names:    []string{"p1"},
			policies: []policiesv1beta1.MutatingPolicyLike{&policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "p1"}}, &policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "p2"}}},
			expected: []bool{true, false},
		},
		{
			name:     "multiple name match",
			names:    []string{"p1", "p3"},
			policies: []policiesv1beta1.MutatingPolicyLike{&policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "p1"}}, &policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "p2"}}, &policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "p3"}}},
			expected: []bool{true, false, true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			predicate := MatchNames(tt.names...)
			for i, policy := range tt.policies {
				assert.Equal(t, tt.expected[i], predicate(policy), "policy name: %s", policy.GetName())
			}
		})
	}
}

func TestPredicates(t *testing.T) {
	clustered := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "add-label"},
	}
	teamA := &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "add-label", Namespace: "team-a"},
	}
	teamB := &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "add-label", Namespace: "team-b"},
	}

	t.Run("clustered only selects cluster scoped policies", func(t *testing.T) {
		predicate := ClusteredPolicy()
		assert.True(t, predicate(clustered))
		assert.False(t, predicate(teamA))
		assert.False(t, predicate(teamB))
	})

	t.Run("namespaced only selects policies from that namespace", func(t *testing.T) {
		predicate := NamespacedPolicy("team-a")
		assert.True(t, predicate(teamA))
		assert.False(t, predicate(teamB), "a policy from another namespace must not be evaluated")
		assert.False(t, predicate(clustered), "a cluster scoped policy must not be evaluated on the namespaced route")
	})

	t.Run("empty namespace never selects any policy", func(t *testing.T) {
		predicate := NamespacedPolicy("")
		assert.False(t, predicate(clustered), "a cluster scoped policy must not be matched when namespace is empty")
		assert.False(t, predicate(teamA))
		assert.False(t, predicate(teamB))
	})
}
