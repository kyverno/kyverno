package api

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestEngineResponse_IsEmpty(t *testing.T) {
	type fields struct {
		PatchedResource unstructured.Unstructured
		Policy          kyvernov1.PolicyInterface
		PolicyResponse  PolicyResponse
		NamespaceLabels map[string]string
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
			NamespaceLabels: map[string]string{
				"a": "b",
			},
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			er := EngineResponse{
				PatchedResource: tt.fields.PatchedResource,
				Policy:          tt.fields.Policy,
				PolicyResponse:  tt.fields.PolicyResponse,
				NamespaceLabels: tt.fields.NamespaceLabels,
			}
			if got := er.IsEmpty(); got != tt.want {
				t.Errorf("EngineResponse.IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEngineResponse_IsNil(t *testing.T) {
	type fields struct {
		PatchedResource unstructured.Unstructured
		Policy          kyvernov1.PolicyInterface
		PolicyResponse  PolicyResponse
		NamespaceLabels map[string]string
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
			NamespaceLabels: map[string]string{
				"a": "b",
			},
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			er := EngineResponse{
				PatchedResource: tt.fields.PatchedResource,
				Policy:          tt.fields.Policy,
				PolicyResponse:  tt.fields.PolicyResponse,
				NamespaceLabels: tt.fields.NamespaceLabels,
			}
			if got := er.IsNil(); got != tt.want {
				t.Errorf("EngineResponse.IsNil() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEngineResponse_IsOneOf(t *testing.T) {
	type fields struct {
		PatchedResource unstructured.Unstructured
		Policy          kyvernov1.PolicyInterface
		PolicyResponse  PolicyResponse
		NamespaceLabels map[string]string
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
				Rules: []RuleResponse{{
					Status: RuleStatusFail,
				}},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{{
					Status: RuleStatusFail,
				}},
			},
		},
		args: args{
			status: []RuleStatus{RuleStatusFail},
		},
		want: true,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{{
					Status: RuleStatusFail,
				}},
			},
		},
		args: args{
			status: []RuleStatus{RuleStatusFail, RuleStatusPass},
		},
		want: true,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{{
					Status: RuleStatusFail,
				}},
			},
		},
		args: args{
			status: []RuleStatus{RuleStatusPass},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{{
					Status: RuleStatusFail,
				}},
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
				Policy:          tt.fields.Policy,
				PolicyResponse:  tt.fields.PolicyResponse,
				NamespaceLabels: tt.fields.NamespaceLabels,
			}
			if got := er.IsOneOf(tt.args.status...); got != tt.want {
				t.Errorf("EngineResponse.IsOneOf() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEngineResponse_IsSuccessful(t *testing.T) {
	type fields struct {
		PatchedResource unstructured.Unstructured
		Policy          kyvernov1.PolicyInterface
		PolicyResponse  PolicyResponse
		NamespaceLabels map[string]string
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
				Rules: []RuleResponse{{
					Status: RuleStatusPass,
				}},
			},
		},
		want: true,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{{
					Status: RuleStatusFail,
				}},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{{
					Status: RuleStatusWarn,
				}},
			},
		},
		want: true,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{{
					Status: RuleStatusError,
				}},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{{
					Status: RuleStatusSkip,
				}},
			},
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			er := EngineResponse{
				PatchedResource: tt.fields.PatchedResource,
				Policy:          tt.fields.Policy,
				PolicyResponse:  tt.fields.PolicyResponse,
				NamespaceLabels: tt.fields.NamespaceLabels,
			}
			if got := er.IsSuccessful(); got != tt.want {
				t.Errorf("EngineResponse.IsSuccessful() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEngineResponse_IsSkipped(t *testing.T) {
	type fields struct {
		PatchedResource unstructured.Unstructured
		Policy          kyvernov1.PolicyInterface
		PolicyResponse  PolicyResponse
		NamespaceLabels map[string]string
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
				Rules: []RuleResponse{{
					Status: RuleStatusPass,
				}},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{{
					Status: RuleStatusFail,
				}},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{{
					Status: RuleStatusWarn,
				}},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{{
					Status: RuleStatusError,
				}},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{{
					Status: RuleStatusSkip,
				}},
			},
		},
		want: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			er := EngineResponse{
				PatchedResource: tt.fields.PatchedResource,
				Policy:          tt.fields.Policy,
				PolicyResponse:  tt.fields.PolicyResponse,
				NamespaceLabels: tt.fields.NamespaceLabels,
			}
			if got := er.IsSkipped(); got != tt.want {
				t.Errorf("EngineResponse.IsSkipped() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEngineResponse_IsFailed(t *testing.T) {
	type fields struct {
		PatchedResource unstructured.Unstructured
		Policy          kyvernov1.PolicyInterface
		PolicyResponse  PolicyResponse
		NamespaceLabels map[string]string
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
				Rules: []RuleResponse{{
					Status: RuleStatusPass,
				}},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{{
					Status: RuleStatusFail,
				}},
			},
		},
		want: true,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{{
					Status: RuleStatusWarn,
				}},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{{
					Status: RuleStatusError,
				}},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{{
					Status: RuleStatusSkip,
				}},
			},
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			er := EngineResponse{
				PatchedResource: tt.fields.PatchedResource,
				Policy:          tt.fields.Policy,
				PolicyResponse:  tt.fields.PolicyResponse,
				NamespaceLabels: tt.fields.NamespaceLabels,
			}
			if got := er.IsFailed(); got != tt.want {
				t.Errorf("EngineResponse.IsFailed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEngineResponse_IsError(t *testing.T) {
	type fields struct {
		PatchedResource unstructured.Unstructured
		Policy          kyvernov1.PolicyInterface
		PolicyResponse  PolicyResponse
		NamespaceLabels map[string]string
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
				Rules: []RuleResponse{{
					Status: RuleStatusPass,
				}},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{{
					Status: RuleStatusFail,
				}},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{{
					Status: RuleStatusWarn,
				}},
			},
		},
		want: false,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{{
					Status: RuleStatusError,
				}},
			},
		},
		want: true,
	}, {
		fields: fields{
			PolicyResponse: PolicyResponse{
				Rules: []RuleResponse{{
					Status: RuleStatusSkip,
				}},
			},
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			er := EngineResponse{
				PatchedResource: tt.fields.PatchedResource,
				Policy:          tt.fields.Policy,
				PolicyResponse:  tt.fields.PolicyResponse,
				NamespaceLabels: tt.fields.NamespaceLabels,
			}
			if got := er.IsError(); got != tt.want {
				t.Errorf("EngineResponse.IsError() = %v, want %v", got, tt.want)
			}
		})
	}
}
