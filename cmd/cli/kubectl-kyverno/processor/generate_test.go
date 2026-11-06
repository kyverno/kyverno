package processor

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func clusterPolicy(name string) *kyvernov1.ClusterPolicy {
	return &kyvernov1.ClusterPolicy{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func namespacedPolicy(namespace, name string) *kyvernov1.Policy {
	return &kyvernov1.Policy{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
}

func TestPolicyRuleKey(t *testing.T) {
	tests := []struct {
		name     string
		policy   kyvernov1.PolicyInterface
		rule     string
		expected string
	}{{
		name:   "cluster policy",
		policy: clusterPolicy("sync-secrets-a"),
		rule:   "sync-image-pull-secret",
		// cluster policies have no namespace, hence the empty segment
		expected: "ClusterPolicy//sync-secrets-a/sync-image-pull-secret",
	}, {
		name:     "namespaced policy",
		policy:   namespacedPolicy("team-a", "sync-secrets"),
		rule:     "sync-image-pull-secret",
		expected: "Policy/team-a/sync-secrets/sync-image-pull-secret",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, PolicyRuleKey(tt.policy, tt.rule))
		})
	}
}

// Rule names are only unique within a policy, so the clone-source key must also
// carry the policy identity. Otherwise one policy's clone source silently
// overwrites another's.
func TestPolicyRuleKey_DistinctPerPolicy(t *testing.T) {
	const rule = "sync-image-pull-secret"
	tests := []struct {
		name string
		a    kyvernov1.PolicyInterface
		b    kyvernov1.PolicyInterface
	}{{
		name: "cluster policies differing by name",
		a:    clusterPolicy("sync-secrets-a"),
		b:    clusterPolicy("sync-secrets-b"),
	}, {
		name: "namespaced policies differing by namespace",
		a:    namespacedPolicy("team-a", "sync-secrets"),
		b:    namespacedPolicy("team-b", "sync-secrets"),
	}, {
		name: "cluster policy and namespaced policy sharing a name",
		a:    clusterPolicy("sync-secrets"),
		b:    namespacedPolicy("team-a", "sync-secrets"),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotEqual(t, PolicyRuleKey(tt.a, rule), PolicyRuleKey(tt.b, rule))
		})
	}
}
