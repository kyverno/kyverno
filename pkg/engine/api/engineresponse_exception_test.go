package api

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestEngineResponse_GetValidationFailureAction_WithExceptionOverride(t *testing.T) {
	resource := unstructured.Unstructured{}
	resource.SetNamespace("dev")

	audit := kyvernov1.Audit
	enforce := kyvernov1.Enforce

	type fields struct {
		PatchedResource unstructured.Unstructured
		GenericPolicy   GenericPolicy
		PolicyResponse  PolicyResponse
		namespaceLabels map[string]string
	}

	tests := []struct {
		name   string
		fields fields
		want   kyvernov1.ValidationFailureAction
	}{
		{
			name: "policy exception with Audit failureAction overrides Enforce policy",
			fields: fields{
				PatchedResource: resource,
				GenericPolicy: NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						ValidationFailureAction: kyvernov1.Enforce,
						Rules: []kyvernov1.Rule{
							{
								Validation: &kyvernov1.Validation{
									FailureAction: &enforce,
								},
							},
						},
					},
				}),
				PolicyResponse: PolicyResponse{
					Rules: []RuleResponse{
						*RuleFail("rule1", Validation, "validation failed", nil).WithExceptions([]GenericException{
							NewPolicyException(&kyvernov2.PolicyException{
								Spec: kyvernov2.PolicyExceptionSpec{
									FailureAction: &audit,
								},
							}),
						}),
					},
				},
			},
			want: kyvernov1.Audit,
		},
		{
			name: "policy exception with Enforce failureAction overrides Audit policy",
			fields: fields{
				PatchedResource: resource,
				GenericPolicy: NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						ValidationFailureAction: kyvernov1.Audit,
						Rules: []kyvernov1.Rule{
							{
								Validation: &kyvernov1.Validation{
									FailureAction: &audit,
								},
							},
						},
					},
				}),
				PolicyResponse: PolicyResponse{
					Rules: []RuleResponse{
						*RuleFail("rule1", Validation, "validation failed", nil).WithExceptions([]GenericException{
							NewPolicyException(&kyvernov2.PolicyException{
								Spec: kyvernov2.PolicyExceptionSpec{
									FailureAction: &enforce,
								},
							}),
						}),
					},
				},
			},
			want: kyvernov1.Enforce,
		},
		{
			name: "policy exception without failureAction uses policy default",
			fields: fields{
				PatchedResource: resource,
				GenericPolicy: NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						ValidationFailureAction: kyvernov1.Enforce,
						Rules: []kyvernov1.Rule{
							{
								Validation: &kyvernov1.Validation{
									FailureAction: &enforce,
								},
							},
						},
					},
				}),
				PolicyResponse: PolicyResponse{
					Rules: []RuleResponse{
						*RuleFail("rule1", Validation, "validation failed", nil).WithExceptions([]GenericException{
							NewPolicyException(&kyvernov2.PolicyException{
								Spec: kyvernov2.PolicyExceptionSpec{
									// No failureAction specified
									FailureAction: nil,
								},
							}),
						}),
					},
				},
			},
			want: kyvernov1.Enforce,
		},
		{
			name: "no exceptions - uses policy failureAction",
			fields: fields{
				PatchedResource: resource,
				GenericPolicy: NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						ValidationFailureAction: kyvernov1.Enforce,
						Rules: []kyvernov1.Rule{
							{
								Validation: &kyvernov1.Validation{
									FailureAction: &enforce,
								},
							},
						},
					},
				}),
				PolicyResponse: PolicyResponse{
					Rules: []RuleResponse{
						*RuleFail("rule1", Validation, "validation failed", nil),
					},
				},
			},
			want: kyvernov1.Enforce,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			er := EngineResponse{
				PatchedResource: tt.fields.PatchedResource,
				PolicyResponse:  tt.fields.PolicyResponse,
				namespaceLabels: tt.fields.namespaceLabels,
			}.WithPolicy(tt.fields.GenericPolicy)

			got := er.GetValidationFailureAction()
			if got != tt.want {
				t.Errorf("EngineResponse.GetValidationFailureAction() = %v, want %v", got, tt.want)
			}
		})
	}
}
