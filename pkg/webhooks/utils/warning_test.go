package utils

import (
	"testing"

	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetWarningMessages(t *testing.T) {
	type args struct {
		engineResponses []engineapi.EngineResponse
	}
	tests := []struct {
		name string
		args args
		want []string
	}{{
		name: "nil response",
		args: args{nil},
		want: nil,
	}, {
		name: "enmpty response",
		args: args{[]engineapi.EngineResponse{}},
		want: nil,
	}, {
		name: "warning",
		args: args{[]engineapi.EngineResponse{
			engineapi.EngineResponse{
				PolicyResponse: engineapi.PolicyResponse{
					Rules: []engineapi.RuleResponse{
						*engineapi.NewRuleResponse("rule", engineapi.Validation, "message warn", engineapi.RuleStatusWarn),
					},
				},
			}.WithPolicy(&v1.ClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			}),
		}},
		want: []string{
			"policy test.rule: message warn",
		},
	}, {
		name: "multiple rules",
		args: args{[]engineapi.EngineResponse{
			engineapi.EngineResponse{
				PolicyResponse: engineapi.PolicyResponse{
					Rules: []engineapi.RuleResponse{
						*engineapi.RulePass("rule-pass", engineapi.Validation, "message pass"),
						*engineapi.NewRuleResponse("rule-warn", engineapi.Validation, "message warn", engineapi.RuleStatusWarn),
						*engineapi.RuleFail("rule-fail", engineapi.Validation, "message fail"),
						*engineapi.RuleError("rule-error", engineapi.Validation, "message error", nil),
						*engineapi.RuleSkip("rule-skip", engineapi.Validation, "message skip"),
					},
				},
			}.WithPolicy(&v1.ClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			}),
		}},
		want: []string{
			"policy test.rule-warn: message warn",
			"policy test.rule-fail: message fail",
			"policy test.rule-error: message error",
		},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetWarningMessages(tt.args.engineResponses)
			assert.Equal(t, tt.want, got)
		})
	}
}
