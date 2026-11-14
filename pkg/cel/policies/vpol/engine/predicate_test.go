package engine

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPredicates(t *testing.T) {
	clustered := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "check-labels"},
	}
	teamA := &policiesv1beta1.NamespacedValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "check-labels", Namespace: "team-a"},
	}
	teamB := &policiesv1beta1.NamespacedValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "check-labels", Namespace: "team-b"},
	}

	t.Run("clustered only selects cluster scoped policies", func(t *testing.T) {
		predicate := And(MatchNames("check-labels"), ClusteredPolicy())
		assert.True(t, predicate(clustered))
		assert.False(t, predicate(teamA))
		assert.False(t, predicate(teamB))
	})

	t.Run("namespaced only selects policies from that namespace", func(t *testing.T) {
		predicate := And(MatchNames("check-labels"), NamespacedPolicy("team-a"))
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
