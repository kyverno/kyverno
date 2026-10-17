package policy

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/background/generate/provenance"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewGenerateURPreservesPolicyUIDAcrossBatches(t *testing.T) {
	t.Parallel()
	policy := &kyvernov1.ClusterPolicy{ObjectMeta: metav1.ObjectMeta{Name: "policy", UID: "policy-uid"}}
	ur := newGenerateUR(engineapi.NewKyvernoPolicy(policy))
	addRuleContext(ur, "first", kyvernov1.ResourceSpec{Kind: "Namespace", Name: "first"}, false, true, false)
	addRuleContext(ur, "second", kyvernov1.ResourceSpec{Kind: "Namespace", Name: "second"}, false, true, false)
	assert.Equal(t, string(policy.GetUID()), ur.GetAnnotations()[provenance.PolicyUIDAnnotation])
	batches := splitUR(ur, 1)
	require.Len(t, batches, 2)
	for _, batch := range batches {
		assert.Equal(t, string(policy.GetUID()), batch.GetAnnotations()[provenance.PolicyUIDAnnotation])
	}
}
