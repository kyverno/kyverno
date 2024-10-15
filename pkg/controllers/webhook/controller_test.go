package webhook

import (
	"testing"

	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestMergeWebhook(t *testing.T) {
	testCases := []struct {
		name          string
		policy        v1.PolicyInterface
		expectedRules sets.Set[GroupVersionResourceScopeOperation]
	}{
		{
			name: "Policy with Validate",
			policy: &v1.ClusterPolicy{
				Spec: v1.Spec{
					Rules: []v1.Rule{
						{
							MatchResources: v1.MatchResources{
								ResourceDescription: v1.ResourceDescription{
									Kinds: []string{"Deployment"},
									Operations: []v1.AdmissionOperation{
										v1.Create,
									},
								},
							},
							Validation: &v1.Validation{
								Message: "test",
							},
						},
					},
				},
			},
			expectedRules: sets.New[GroupVersionResourceScopeOperation](
				GroupVersionResourceScopeOperation{
					GroupVersionResource: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
					Scope:                admissionregistrationv1.AllScopes,
					Operation:            admissionregistrationv1.Create,
				},
			),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := &controller{
				discoveryClient: dclient.NewFakeDiscoveryClient(
					[]schema.GroupVersionResource{},
				),
				policyState: map[string]sets.Set[string]{},
			}

			dst := &webhook{
				rules:             sets.Set[GroupVersionResourceScopeOperation]{},
				failurePolicy:     admissionregistrationv1.Fail,
				maxWebhookTimeout: 10,
			}

			c.mergeWebhook(dst, tc.policy, true)

			assert.Equal(t, tc.expectedRules, dst.rules)
		})
	}
}
