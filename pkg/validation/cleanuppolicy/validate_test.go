package cleanuppolicy

import (
	"context"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func newCleanupPolicy(name string, conditions *kyvernov2.AnyAllConditions) *kyvernov2.CleanupPolicy {
	return &kyvernov2.CleanupPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: kyvernov2.CleanupPolicySpec{
			Schedule: "* * * * *",
			MatchResources: kyvernov2.MatchResources{
				Any: kyvernov1.ResourceFilters{
					{
						ResourceDescription: kyvernov1.ResourceDescription{
							Kinds: []string{"Pod"},
						},
					},
				},
			},
			Conditions: conditions,
		},
	}
}

func conditionWithKey(key string) *kyvernov2.AnyAllConditions {
	return &kyvernov2.AnyAllConditions{
		AllConditions: []kyvernov2.Condition{
			{
				RawKey:   kyverno.ToAny(key),
				Operator: kyvernov2.ConditionOperators["Equals"],
				RawValue: kyverno.ToAny("value"),
			},
		},
	}
}

func TestValidatePolicy(t *testing.T) {
	clusterResources := sets.New("v1/Namespace", "Namespace")

	t.Run("valid policy passes", func(t *testing.T) {
		assert.NoError(t, validatePolicy(clusterResources, newCleanupPolicy("valid-policy", nil)))
	})

	t.Run("name longer than 63 characters is rejected", func(t *testing.T) {
		err := validatePolicy(clusterResources, newCleanupPolicy(strings.Repeat("a", 64), nil))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "metadata.name")
	})
}

// newAuthClient builds a dclient whose SubjectAccessReviews always answer with
// the supplied verdict, so validateAuth can be exercised without a cluster.
func newAuthClient(allowed bool) dclient.Interface {
	kube := kubefake.NewSimpleClientset()
	kube.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, &authorizationv1.SubjectAccessReview{
			Status: authorizationv1.SubjectAccessReviewStatus{Allowed: allowed},
		}, nil
	})
	return dclient.NewFakeClientWithDisco(nil, kube, dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{{Version: "v1", Resource: "pods"}}))
}

func TestValidateAuth(t *testing.T) {
	policy := newCleanupPolicy("cleanup-policy", nil)

	t.Run("permitted deletion passes", func(t *testing.T) {
		assert.NoError(t, validateAuth(context.Background(), newAuthClient(true), policy))
	})

	t.Run("denied deletion is reported", func(t *testing.T) {
		err := validateAuth(context.Background(), newAuthClient(false), policy)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no permission")
	})

	t.Run("named resources are checked individually", func(t *testing.T) {
		named := newCleanupPolicy("cleanup-policy", nil)
		named.Spec.MatchResources.Any[0].ResourceDescription.Names = []string{"first", "second"}
		assert.NoError(t, validateAuth(context.Background(), newAuthClient(true), named))
	})
}

func TestValidateVariables(t *testing.T) {
	tests := []struct {
		name       string
		conditions *kyvernov2.AnyAllConditions
		wantErr    bool
	}{
		{
			name:       "no conditions",
			conditions: nil,
		},
		{
			name:       "allowed target variable",
			conditions: conditionWithKey("{{ target.metadata.name }}"),
		},
		{
			name:       "static key without variables",
			conditions: conditionWithKey("some-static-key"),
		},
		{
			// allowedVariables only permits lowercase/underscore/digit runs,
			// target./images. prefixes and function calls, so an uppercase
			// variable falls outside it
			name:       "disallowed variable is rejected",
			conditions: conditionWithKey("{{ REQUEST }}"),
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVariables(logr.Discard(), newCleanupPolicy("cleanup-policy", tt.conditions))
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "variable substitution failed")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
