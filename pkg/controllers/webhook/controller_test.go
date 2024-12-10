package webhook

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
)

func TestCheckIfPolicyIsConfiguredInWebhooks(t *testing.T) {
	tests := []struct {
		name       string
		policy     kyvernov1.PolicyInterface
		mExists    bool
		vExists    bool
		wantResult bool
	}{
		{
			name: "Policy with mutate rule and mutating webhook exists",
			policy: &kyvernov1.ClusterPolicy{
				Spec: kyvernov1.Spec{
					Rules: []kyvernov1.Rule{
						{
							Mutation: &kyvernov1.Mutation{
								PatchesJSON6902: "test",
							},
						},
					},
				},
			},
			mExists:    true,
			vExists:    false,
			wantResult: true,
		},
		{
			name: "Policy with validate rule and validating webhook exists",
			policy: &kyvernov1.ClusterPolicy{
				Spec: kyvernov1.Spec{
					Rules: []kyvernov1.Rule{
						{
							Validation: &kyvernov1.Validation{
								Message: "test",
							},
						},
					},
				},
			},
			mExists:    false,
			vExists:    true,
			wantResult: true,
		},
		{
			name: "Policy with generate rule and validating webhook exists",
			policy: &kyvernov1.ClusterPolicy{
				Spec: kyvernov1.Spec{
					Rules: []kyvernov1.Rule{
						{
							Generation: &kyvernov1.Generation{
								Synchronize: true,
							},
						},
					},
				},
			},
			mExists:    false,
			vExists:    true,
			wantResult: true,
		},
		{
			name: "Policy with mutateExisting rule while validating and mutating webhook exists",
			policy: &kyvernov1.ClusterPolicy{
				Spec: kyvernov1.Spec{
					Rules: []kyvernov1.Rule{
						{
							Mutation: &kyvernov1.Mutation{
								PatchesJSON6902: "test",
							},
						},
					},
				},
			},
			mExists:    true,
			vExists:    true,
			wantResult: true,
		},
		{
			name: "Policy with verifyImages rule and both webhooks exist",
			policy: &kyvernov1.ClusterPolicy{
				Spec: kyvernov1.Spec{
					Rules: []kyvernov1.Rule{
						{
							VerifyImages: []kyvernov1.ImageVerification{
								{
									Image: "test",
								},
							},
						},
					},
				},
			},
			mExists:    true,
			vExists:    true,
			wantResult: true,
		},
		{
			name: "Policy with mutate rule and mutating webhook does not exist",
			policy: &kyvernov1.ClusterPolicy{
				Spec: kyvernov1.Spec{
					Rules: []kyvernov1.Rule{
						{
							Mutation: &kyvernov1.Mutation{
								PatchesJSON6902: "test",
							},
						},
					},
				},
			},
			mExists:    false,
			vExists:    false,
			wantResult: false,
		},
		{
			name: "Policy with validate rule and validating webhook does not exist",
			policy: &kyvernov1.ClusterPolicy{
				Spec: kyvernov1.Spec{
					Rules: []kyvernov1.Rule{
						{
							Validation: &kyvernov1.Validation{
								Message: "test",
							},
						},
					},
				},
			},
			mExists:    false,
			vExists:    false,
			wantResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkIfPolicyIsConfiguredInWebhooks(tt.policy, tt.mExists, tt.vExists)
			assert.Equal(t, tt.wantResult, result)
		})
	}
}
