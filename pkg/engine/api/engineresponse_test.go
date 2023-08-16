package api

import (
	"reflect"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestEngineResponse_IsEmpty(t *testing.T) {
	type fields struct {
		PatchedResource unstructured.Unstructured
		GenericPolicy   GenericPolicy
		PolicyResponse  PolicyResponse
		namespaceLabels map[string]string
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{{
		want: true,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{{}},
			},
		},
		want: false,
	}, {
		fields: fields{
			namespaceLabels: map[string]string{
				"a": "b",
			},
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			er := EngineResponse{
				PatchedResource: tt.fields.PatchedResource,
				PolicyResponse:  tt.fields.PolicyResponse,
				namespaceLabels: tt.fields.namespaceLabels,
			}.WithPolicy(tt.fields.GenericPolicy)
			if got := er.IsEmpty(); got != tt.want {
				t.Errorf("EngineResponse.IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEngineResponse_IsNil(t *testing.T) {
	type fields struct {
		PatchedResource unstructured.Unstructured
		GenericPolicy   GenericPolicy
		PolicyResponse  PolicyResponse
		namespaceLabels map[string]string
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{{
		want: true,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{{}},
			},
		},
		want: false,
	}, {
		fields: fields{
			namespaceLabels: map[string]string{
				"a": "b",
			},
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			er := EngineResponse{
				PatchedResource: tt.fields.PatchedResource,
				PolicyResponse:  tt.fields.PolicyResponse,
				namespaceLabels: tt.fields.namespaceLabels,
			}.WithPolicy(tt.fields.GenericPolicy)
			if got := er.IsNil(); got != tt.want {
				t.Errorf("EngineResponse.IsNil() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEngineResponse_IsOneOf(t *testing.T) {
	type fields struct {
		PatchedResource unstructured.Unstructured
		GenericPolicy   GenericPolicy
		PolicyResponse  PolicyResponse
		namespaceLabels map[string]string
	}
	type args struct {
		status []RuleStatus
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{{
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleFail("", Validation, ""),
				},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleFail("", Validation, ""),
				},
			},
		},
		args: args{
			status: []RuleStatus{RuleStatusFail},
		},
		want: true,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleFail("", Validation, ""),
				},
			},
		},
		args: args{
			status: []RuleStatus{RuleStatusFail, RuleStatusPass},
		},
		want: true,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleFail("", Validation, ""),
				},
			},
		},
		args: args{
			status: []RuleStatus{RuleStatusPass},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleFail("", Validation, ""),
				},
			},
		},
		args: args{
			status: []RuleStatus{},
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			er := EngineResponse{
				PatchedResource: tt.fields.PatchedResource,
				PolicyResponse:  tt.fields.PolicyResponse,
				namespaceLabels: tt.fields.namespaceLabels,
			}.WithPolicy(tt.fields.GenericPolicy)
			if got := er.IsOneOf(tt.args.status...); got != tt.want {
				t.Errorf("EngineResponse.IsOneOf() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEngineResponse_IsSuccessful(t *testing.T) {
	type fields struct {
		PatchedResource unstructured.Unstructured
		GenericPolicy   GenericPolicy
		PolicyResponse  PolicyResponse
		namespaceLabels map[string]string
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{{
		want: true,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RulePass("", Validation, ""),
				},
			},
		},
		want: true,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleFail("", Validation, ""),
				},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleWarn("", Validation, ""),
				},
			},
		},
		want: true,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleError("", Validation, "", nil),
				},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleSkip("", Validation, ""),
				},
			},
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			er := EngineResponse{
				PatchedResource: tt.fields.PatchedResource,
				PolicyResponse:  tt.fields.PolicyResponse,
				namespaceLabels: tt.fields.namespaceLabels,
			}.WithPolicy(tt.fields.GenericPolicy)
			if got := er.IsSuccessful(); got != tt.want {
				t.Errorf("EngineResponse.IsSuccessful() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEngineResponse_IsSkipped(t *testing.T) {
	type fields struct {
		PatchedResource unstructured.Unstructured
		GenericPolicy   GenericPolicy
		PolicyResponse  PolicyResponse
		namespaceLabels map[string]string
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{{
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RulePass("", Validation, ""),
				},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleFail("", Validation, ""),
				},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleWarn("", Validation, ""),
				},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleError("", Validation, "", nil),
				},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleSkip("", Validation, ""),
				},
			},
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			er := EngineResponse{
				PatchedResource: tt.fields.PatchedResource,
				PolicyResponse:  tt.fields.PolicyResponse,
				namespaceLabels: tt.fields.namespaceLabels,
			}.WithPolicy(tt.fields.GenericPolicy)
			if got := er.IsSkipped(); got != tt.want {
				t.Errorf("EngineResponse.IsSkipped() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEngineResponse_IsFailed(t *testing.T) {
	type fields struct {
		PatchedResource unstructured.Unstructured
		GenericPolicy   GenericPolicy
		PolicyResponse  PolicyResponse
		namespaceLabels map[string]string
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{{
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RulePass("", Validation, ""),
				},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleFail("", Validation, ""),
				},
			},
		},
		want: true,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleWarn("", Validation, ""),
				},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleError("", Validation, "", nil),
				},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleSkip("", Validation, ""),
				},
			},
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			er := EngineResponse{
				PatchedResource: tt.fields.PatchedResource,
				PolicyResponse:  tt.fields.PolicyResponse,
				namespaceLabels: tt.fields.namespaceLabels,
			}.WithPolicy(tt.fields.GenericPolicy)
			if got := er.IsFailed(); got != tt.want {
				t.Errorf("EngineResponse.IsFailed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEngineResponse_IsError(t *testing.T) {
	type fields struct {
		PatchedResource unstructured.Unstructured
		GenericPolicy   GenericPolicy
		PolicyResponse  PolicyResponse
		namespaceLabels map[string]string
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{{
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RulePass("", Validation, ""),
				},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleFail("", Validation, ""),
				},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleWarn("", Validation, ""),
				},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleError("", Validation, "", nil),
				},
			},
		},
		want: true,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleSkip("", Validation, ""),
				},
			},
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			er := EngineResponse{
				PatchedResource: tt.fields.PatchedResource,
				PolicyResponse:  tt.fields.PolicyResponse,
				namespaceLabels: tt.fields.namespaceLabels,
			}.WithPolicy(tt.fields.GenericPolicy)
			if got := er.IsError(); got != tt.want {
				t.Errorf("EngineResponse.IsError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEngineResponse_GetFailedRules(t *testing.T) {
	type fields struct {
		PatchedResource unstructured.Unstructured
		GenericPolicy   GenericPolicy
		PolicyResponse  PolicyResponse
		namespaceLabels map[string]string
	}
	tests := []struct {
		name   string
		fields fields
		want   []string
	}{{
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleSkip("skip", Validation, ""),
				},
			},
		},
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleWarn("warn", Validation, ""),
				},
			},
		},
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RulePass("pass", Validation, ""),
				},
			},
		},
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleFail("fail", Validation, ""),
				},
			},
		},
		want: []string{"fail"},
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleFail("fail-1", Validation, ""),
					*RuleFail("fail-2", Validation, ""),
				},
			},
		},
		want: []string{"fail-1", "fail-2"},
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleFail("fail-1", Validation, ""),
					*RuleError("error-1", Validation, "", nil),
				},
			},
		},
		want: []string{"fail-1", "error-1"},
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleError("error-1", Validation, "", nil),
					*RuleError("error-2", Validation, "", nil),
				},
			},
		},
		want: []string{"error-1", "error-2"},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			er := EngineResponse{
				PatchedResource: tt.fields.PatchedResource,
				PolicyResponse:  tt.fields.PolicyResponse,
				namespaceLabels: tt.fields.namespaceLabels,
			}.WithPolicy(tt.fields.GenericPolicy)
			if got := er.GetFailedRules(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EngineResponse.GetFailedRules() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEngineResponse_GetSuccessRules(t *testing.T) {
	type fields struct {
		PatchedResource unstructured.Unstructured
		GenericPolicy   GenericPolicy
		PolicyResponse  PolicyResponse
		namespaceLabels map[string]string
	}
	tests := []struct {
		name   string
		fields fields
		want   []string
	}{{
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleSkip("skip", Validation, ""),
				},
			},
		},
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleWarn("warn", Validation, ""),
				},
			},
		},
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RulePass("pass-1", Validation, ""),
					*RulePass("pass-2", Validation, ""),
				},
			},
		},
		want: []string{"pass-1", "pass-2"},
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RulePass("pass", Validation, ""),
				},
			},
		},
		want: []string{"pass"},
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RulePass("pass", Validation, ""),
					*RuleFail("fail", Validation, ""),
				},
			},
		},
		want: []string{"pass"},
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RulePass("pass", Validation, ""),
					*RuleSkip("skip", Validation, ""),
				},
			},
		},
		want: []string{"pass"},
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleFail("fail", Validation, ""),
				},
			},
		},
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleFail("fail-1", Validation, ""),
					*RuleFail("fail-2", Validation, ""),
				},
			},
		},
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleFail("fail-1", Validation, ""),
					*RuleError("error-1", Validation, "", nil),
				},
			},
		},
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{
					*RuleError("error-1", Validation, "", nil),
					*RuleError("error-2", Validation, "", nil),
				},
			},
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			er := EngineResponse{
				PatchedResource: tt.fields.PatchedResource,
				PolicyResponse:  tt.fields.PolicyResponse,
				namespaceLabels: tt.fields.namespaceLabels,
			}.WithPolicy(tt.fields.GenericPolicy)
			if got := er.GetSuccessRules(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EngineResponse.GetSuccessRules() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEngineResponse_GetValidationFailureAction(t *testing.T) {
	resource := unstructured.Unstructured{}
	resource.SetNamespace("foo")
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
	}{{
		fields: fields{
			GenericPolicy: NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
				Spec: kyvernov1.Spec{
					ValidationFailureAction: kyvernov1.Audit,
				},
			}),
		},
		want: kyvernov1.Audit,
	}, {
		fields: fields{
			GenericPolicy: NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
				Spec: kyvernov1.Spec{
					ValidationFailureAction: kyvernov1.Enforce,
				},
			}),
		},
		want: kyvernov1.Enforce,
	}, {
		fields: fields{
			GenericPolicy: NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
				Spec: kyvernov1.Spec{
					ValidationFailureAction: kyvernov1.Enforce,
					ValidationFailureActionOverrides: []kyvernov1.ValidationFailureActionOverride{{
						Action:     kyvernov1.Audit,
						Namespaces: []string{"*"},
					}},
				},
			}),
		},
		want: kyvernov1.Audit,
	}, {
		fields: fields{
			GenericPolicy: NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
				Spec: kyvernov1.Spec{
					ValidationFailureAction: kyvernov1.Enforce,
					ValidationFailureActionOverrides: []kyvernov1.ValidationFailureActionOverride{{
						Action:     "invalid",
						Namespaces: []string{"*"},
					}},
				},
			}),
		},
		want: kyvernov1.Enforce,
	}, {
		fields: fields{
			PatchedResource: resource,
			GenericPolicy: NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
				Spec: kyvernov1.Spec{
					ValidationFailureAction: kyvernov1.Enforce,
					ValidationFailureActionOverrides: []kyvernov1.ValidationFailureActionOverride{{
						Action:     kyvernov1.Audit,
						Namespaces: []string{"foo"},
					}},
				},
			}),
		},
		want: kyvernov1.Audit,
	}, {
		fields: fields{
			PatchedResource: resource,
			GenericPolicy: NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
				Spec: kyvernov1.Spec{
					ValidationFailureAction: kyvernov1.Enforce,
					ValidationFailureActionOverrides: []kyvernov1.ValidationFailureActionOverride{{
						Action:     kyvernov1.Audit,
						Namespaces: []string{"bar"},
					}},
				},
			}),
		},
		want: kyvernov1.Enforce,
	}, {
		fields: fields{
			namespaceLabels: map[string]string{
				"foo": "bar",
			},
			PatchedResource: resource,
			GenericPolicy: NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
				Spec: kyvernov1.Spec{
					ValidationFailureAction: kyvernov1.Enforce,
					ValidationFailureActionOverrides: []kyvernov1.ValidationFailureActionOverride{{
						Action: kyvernov1.Audit,
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"bar": "foo",
							},
						},
					}},
				},
			}),
		},
		want: kyvernov1.Enforce,
	}, {
		fields: fields{
			namespaceLabels: map[string]string{
				"foo": "bar",
			},
			PatchedResource: resource,
			GenericPolicy: NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
				Spec: kyvernov1.Spec{
					ValidationFailureAction: kyvernov1.Enforce,
					ValidationFailureActionOverrides: []kyvernov1.ValidationFailureActionOverride{{
						Action: kyvernov1.Audit,
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"foo": "bar",
							},
						},
					}},
				},
			}),
		},
		want: kyvernov1.Audit,
	}, {
		fields: fields{
			namespaceLabels: map[string]string{
				"foo": "bar",
			},
			PatchedResource: resource,
			GenericPolicy: NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
				Spec: kyvernov1.Spec{
					ValidationFailureAction: kyvernov1.Enforce,
					ValidationFailureActionOverrides: []kyvernov1.ValidationFailureActionOverride{{
						Action:     kyvernov1.Audit,
						Namespaces: []string{"foo"},
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"bar": "foo",
							},
						},
					}},
				},
			}),
		},
		want: kyvernov1.Enforce,
	}, {
		fields: fields{
			namespaceLabels: map[string]string{
				"foo": "bar",
			},
			PatchedResource: resource,
			GenericPolicy: NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
				Spec: kyvernov1.Spec{
					ValidationFailureAction: kyvernov1.Enforce,
					ValidationFailureActionOverrides: []kyvernov1.ValidationFailureActionOverride{{
						Action:     kyvernov1.Audit,
						Namespaces: []string{"bar"},
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"foo": "bar",
							},
						},
					}},
				},
			}),
		},
		want: kyvernov1.Enforce,
	}, {
		fields: fields{
			namespaceLabels: map[string]string{
				"foo": "bar",
			},
			PatchedResource: resource,
			GenericPolicy: NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
				Spec: kyvernov1.Spec{
					ValidationFailureAction: kyvernov1.Enforce,
					ValidationFailureActionOverrides: []kyvernov1.ValidationFailureActionOverride{{
						Action:     kyvernov1.Audit,
						Namespaces: []string{"foo"},
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"foo": "bar",
							},
						},
					}},
				},
			}),
		},
		want: kyvernov1.Audit,
	}, {
		fields: fields{
			namespaceLabels: map[string]string{
				"foo": "bar",
			},
			PatchedResource: resource,
			GenericPolicy: NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
				Spec: kyvernov1.Spec{
					ValidationFailureAction: kyvernov1.Enforce,
					ValidationFailureActionOverrides: []kyvernov1.ValidationFailureActionOverride{{
						Action:     kyvernov1.Audit,
						Namespaces: []string{"*"},
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"foo": "bar",
							},
						},
					}},
				},
			}),
		},
		want: kyvernov1.Audit,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			er := EngineResponse{
				PatchedResource: tt.fields.PatchedResource,
				PolicyResponse:  tt.fields.PolicyResponse,
				namespaceLabels: tt.fields.namespaceLabels,
			}.WithPolicy(tt.fields.GenericPolicy)
			if got := er.GetValidationFailureAction(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EngineResponse.GetValidationFailureAction() = %v, want %v", got, tt.want)
			}
		})
	}
}

// func TestEngineResponse_GetPatches(t *testing.T) {
// 	type fields struct {
// 		PatchedResource unstructured.Unstructured
// 		GenericPolicy          kyvernov1.PolicyInterface
// 		PolicyResponse  PolicyResponse
// 		namespaceLabels map[string]string
// 	}
// 	tests := []struct {
// 		name   string
// 		fields fields
// 		want   []jsonpatch.JsonPatchOperation
// 	}{{}, {
// 		fields: fields{
// 			PolicyResponse: PolicyResponse{
// 				Rules: nil,
// 			},
// 		},
// 	}, {
// 		fields: fields{
// 			PolicyResponse: PolicyResponse{
// 				Rules: []RuleResponse{},
// 			},
// 		},
// 	}, {
// 		fields: fields{
// 			PolicyResponse: PolicyResponse{
// 				Rules: []RuleResponse{{}},
// 			},
// 		},
// 	}, {
// 		fields: fields{
// 			PolicyResponse: PolicyResponse{
// 				Rules: []RuleResponse{
// 					{},
// 					*RuleResponse{}.WithPatches([]jsonpatch.JsonPatchOperation{{
// 						Operation: "add",
// 						Path:      "/1",
// 						Value:     0,
// 					}, {
// 						Operation: "add",
// 						Path:      "/2",
// 						Value:     1,
// 					}}...),
// 				},
// 			},
// 		},
// 		want: []jsonpatch.JsonPatchOperation{{
// 			Operation: "add",
// 			Path:      "/1",
// 			Value:     0,
// 		}, {
// 			Operation: "add",
// 			Path:      "/2",
// 			Value:     1,
// 		}},
// 	}, {
// 		fields: fields{
// 			PolicyResponse: PolicyResponse{
// 				Rules: []RuleResponse{
// 					{},
// 					*RuleResponse{}.WithPatches([]jsonpatch.JsonPatchOperation{{
// 						Operation: "add",
// 						Path:      "/1",
// 						Value:     0,
// 					}, {
// 						Operation: "add",
// 						Path:      "/2",
// 						Value:     1,
// 					}}...),
// 					*RuleResponse{}.WithPatches([]jsonpatch.JsonPatchOperation{{
// 						Operation: "add",
// 						Path:      "/3",
// 						Value:     2,
// 					}}...),
// 				},
// 			},
// 		},
// 		want: []jsonpatch.JsonPatchOperation{{
// 			Operation: "add",
// 			Path:      "/1",
// 			Value:     0,
// 		}, {
// 			Operation: "add",
// 			Path:      "/2",
// 			Value:     1,
// 		}, {
// 			Operation: "add",
// 			Path:      "/3",
// 			Value:     2,
// 		}},
// 	}}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			er := EngineResponse{
// 				PatchedResource: tt.fields.PatchedResource,
// 				PolicyResponse:  tt.fields.PolicyResponse,
// 				namespaceLabels: tt.fields.namespaceLabels,
// 			}.WithPolicy(tt.fields.GenericPolicy)
// 			if got := er.GetPatches(); !reflect.DeepEqual(got, tt.want) {
// 				t.Errorf("EngineResponse.GetPatches() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }

func TestEngineResponse_GetResourceSpec(t *testing.T) {
	namespacedResource := unstructured.Unstructured{}
	namespacedResource.SetKind("Something")
	namespacedResource.SetAPIVersion("test/v1")
	namespacedResource.SetNamespace("foo")
	namespacedResource.SetName("bar")
	namespacedResource.SetUID("12345")
	clusteredResource := unstructured.Unstructured{}
	clusteredResource.SetKind("Something")
	clusteredResource.SetAPIVersion("test/v1")
	clusteredResource.SetName("bar")
	clusteredResource.SetUID("12345")
	type fields struct {
		PatchedResource unstructured.Unstructured
		GenericPolicy   GenericPolicy
		PolicyResponse  PolicyResponse
		namespaceLabels map[string]string
	}
	tests := []struct {
		name   string
		fields fields
		want   ResourceSpec
	}{{
		fields: fields{
			PatchedResource: namespacedResource,
		},
		want: ResourceSpec{
			Kind:       "Something",
			APIVersion: "test/v1",
			Namespace:  "foo",
			Name:       "bar",
			UID:        "12345",
		},
	}, {
		fields: fields{
			PatchedResource: clusteredResource,
		},
		want: ResourceSpec{
			Kind:       "Something",
			APIVersion: "test/v1",
			Name:       "bar",
			UID:        "12345",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			er := EngineResponse{
				PatchedResource: tt.fields.PatchedResource,
				PolicyResponse:  tt.fields.PolicyResponse,
				namespaceLabels: tt.fields.namespaceLabels,
			}.WithPolicy(tt.fields.GenericPolicy)
			if got := er.GetResourceSpec(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EngineResponse.GetResourceSpec() = %v, want %v", got, tt.want)
			}
		})
	}
}
