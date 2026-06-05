package report

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestSelectorResourceUidEquals(t *testing.T) {
	tests := []struct {
		name    string
		uid     types.UID
		wantErr bool
	}{{
		name:    "valid UID",
		uid:     types.UID("12345-abcde-67890"),
		wantErr: false,
	}, {
		name:    "empty UID",
		uid:     types.UID(""),
		wantErr: false,
	}, {
		name:    "UUID format",
		uid:     types.UID("550e8400-e29b-41d4-a716-446655440000"),
		wantErr: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector, err := SelectorResourceUidEquals(tt.uid)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, selector)
			// Selector should match labels containing the UID
			matchLabels := map[string]string{
				LabelResourceUid: string(tt.uid),
			}
			assert.True(t, selector.Matches(mapToSet(matchLabels)))
			// Selector should NOT match a different UID
			noMatchLabels := map[string]string{
				LabelResourceUid: "different-uid",
			}
			if string(tt.uid) != "different-uid" {
				assert.False(t, selector.Matches(mapToSet(noMatchLabels)))
			}
		})
	}
}

func TestSelectorPolicyDoesNotExist(t *testing.T) {
	tests := []struct {
		name       string
		policyName string
		namespaced bool
		wantErr    bool
	}{{
		name:       "cluster policy does not exist",
		policyName: "require-labels",
		namespaced: false,
		wantErr:    false,
	}, {
		name:       "namespaced policy does not exist",
		policyName: "restrict-images",
		namespaced: true,
		wantErr:    false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := makeTestPolicy(tt.policyName, tt.namespaced)
			selector, err := SelectorPolicyDoesNotExist(policy)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, selector)
			// Selector should match when the policy label is absent
			assert.True(t, selector.Matches(mapToSet(map[string]string{})))
			// Selector should NOT match when the policy label is present
			policyLabel := PolicyLabel(policy)
			assert.False(t, selector.Matches(mapToSet(map[string]string{
				policyLabel: "1",
			})))
		})
	}
}

func TestSelectorPolicyExists(t *testing.T) {
	tests := []struct {
		name       string
		policyName string
		namespaced bool
		wantErr    bool
	}{{
		name:       "cluster policy exists",
		policyName: "disallow-privileged",
		namespaced: false,
		wantErr:    false,
	}, {
		name:       "namespaced policy exists",
		policyName: "require-probes",
		namespaced: true,
		wantErr:    false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := makeTestPolicy(tt.policyName, tt.namespaced)
			selector, err := SelectorPolicyExists(policy)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, selector)
			// Selector should match when the policy label is present
			policyLabel := PolicyLabel(policy)
			assert.True(t, selector.Matches(mapToSet(map[string]string{
				policyLabel: "v1",
			})))
			// Selector should NOT match when the policy label is absent
			assert.False(t, selector.Matches(mapToSet(map[string]string{})))
		})
	}
}

func TestSelectorPolicyNotEquals(t *testing.T) {
	tests := []struct {
		name            string
		policyName      string
		resourceVersion string
		namespaced      bool
		wantErr         bool
	}{{
		name:            "cluster policy version mismatch",
		policyName:      "require-labels",
		resourceVersion: "100",
		namespaced:      false,
		wantErr:         false,
	}, {
		name:            "namespaced policy version mismatch",
		policyName:      "limit-resources",
		resourceVersion: "42",
		namespaced:      true,
		wantErr:         false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := makeTestPolicyWithVersion(tt.policyName, tt.namespaced, tt.resourceVersion)
			selector, err := SelectorPolicyNotEquals(policy)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, selector)
			policyLabel := PolicyLabel(policy)
			// Selector should match when the version is different
			assert.True(t, selector.Matches(mapToSet(map[string]string{
				policyLabel: "999",
			})))
			// Selector should NOT match when the version is the same
			assert.False(t, selector.Matches(mapToSet(map[string]string{
				policyLabel: tt.resourceVersion,
			})))
		})
	}
}

// helpers

func makeTestPolicy(name string, namespaced bool) engineapi.GenericPolicy {
	if namespaced {
		return engineapi.NewKyvernoPolicy(&kyvernov1.Policy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
			},
		})
	}
	return engineapi.NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	})
}

func makeTestPolicyWithVersion(name string, namespaced bool, resourceVersion string) engineapi.GenericPolicy {
	if namespaced {
		return engineapi.NewKyvernoPolicy(&kyvernov1.Policy{
			ObjectMeta: metav1.ObjectMeta{
				Name:            name,
				Namespace:       "default",
				ResourceVersion: resourceVersion,
			},
		})
	}
	return engineapi.NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			ResourceVersion: resourceVersion,
		},
	})
}

// mapToSet converts a map to a labels.Set for use with selector.Matches.
type labelSet map[string]string

func mapToSet(m map[string]string) labelSet { return labelSet(m) }
func (s labelSet) Has(key string) bool      { _, ok := s[key]; return ok }
func (s labelSet) Get(key string) string     { return s[key] }
func (s labelSet) Lookup(key string) (string, bool) {
	v, ok := s[key]
	return v, ok
}
