package utils

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGetErrorMsg(t *testing.T) {
	policy := engineapi.NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
		ObjectMeta: v1.ObjectMeta{
			Name: "restrict-privileged",
		},
	})

	resource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind": "Pod",
			"metadata": map[string]interface{}{
				"namespace": "default",
				"name":      "mypod",
			},
		},
	}

	type args struct {
		engineResponses []engineapi.EngineResponse
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "single failed rule",
			args: args{
				engineResponses: []engineapi.EngineResponse{
					engineapi.NewEngineResponse(resource, policy, nil).
						WithPolicyResponse(engineapi.PolicyResponse{
							Rules: []engineapi.RuleResponse{
								*engineapi.RuleFail(
									"deny-privileged",
									engineapi.Validation,
									"privileged containers are not allowed",
									nil,
								),
							},
						}),
				},
			},
			want: "Resource Pod/default/mypod failed policy restrict-privileged:;rule deny-privileged (Validation): privileged containers are not allowed",
		},
		{
			name: "only pass rule",
			args: args{
				engineResponses: []engineapi.EngineResponse{
					engineapi.NewEngineResponse(resource, policy, nil).
						WithPolicyResponse(engineapi.PolicyResponse{
							Rules: []engineapi.RuleResponse{
								*engineapi.RulePass(
									"allow",
									engineapi.Validation,
									"allowed",
									nil,
								),
							},
						}),
				},
			},
			want: "Resource  ",
		},
		{
			name: "multiple failed rules",
			args: args{
				engineResponses: []engineapi.EngineResponse{
					engineapi.NewEngineResponse(resource, policy, nil).
						WithPolicyResponse(engineapi.PolicyResponse{
							Rules: []engineapi.RuleResponse{
								*engineapi.RuleFail("rule-fail", engineapi.Validation, "failure", nil),
								*engineapi.RuleError("rule-error", engineapi.Validation, "error", nil, nil),
							},
						}),
				},
			},
			want: "Resource Pod/default/mypod failed policy restrict-privileged:;rule rule-fail (Validation): failure;rule rule-error (Validation): error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetErrorMsg(tt.args.engineResponses)
			assert.Equal(t, tt.want, got)
		})
	}
}
