package engine

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPredicates(t *testing.T) {
	// Policies sharing a name across scopes: the cluster-scoped one, and one in each namespace.
	clustered := &policiesv1beta1.ImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "check-images"},
	}
	teamA := &policiesv1beta1.NamespacedImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "check-images", Namespace: "team-a"},
	}
	teamB := &policiesv1beta1.NamespacedImageValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "check-images", Namespace: "team-b"},
	}

	t.Run("clustered only selects cluster scoped policies", func(t *testing.T) {
		predicate := And(MatchNames("check-images"), ClusteredPolicy())
		assert.True(t, predicate(clustered))
		assert.False(t, predicate(teamA))
		assert.False(t, predicate(teamB))
	})

	t.Run("namespaced only selects policies from that namespace", func(t *testing.T) {
		predicate := And(MatchNames("check-images"), NamespacedPolicy("team-a"))
		assert.True(t, predicate(teamA))
		assert.False(t, predicate(teamB), "a policy from another namespace must not be evaluated")
		assert.False(t, predicate(clustered), "a cluster scoped policy must not be evaluated on the namespaced route")
	})
}
