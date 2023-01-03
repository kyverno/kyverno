package utils

import (
	"testing"

	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetWarningMessages(t *testing.T) {
	type args struct {
		engineResponses []*api.EngineResponse
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
		args: args{[]*api.EngineResponse{}},
		want: nil,
	}, {
		name: "warning",
		args: args{[]*api.EngineResponse{
			{
				Policy: &v1.ClusterPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				PolicyResponse: api.PolicyResponse{
					Rules: []api.RuleResponse{
						{
							Name:    "rule",
							Status:  api.RuleStatusWarn,
							Message: "message warn",
						},
					},
				},
			},
		}},
		want: []string{
			"policy test.rule: message warn",
		},
	}, {
		name: "multiple rules",
		args: args{[]*api.EngineResponse{
			{
				Policy: &v1.ClusterPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
				PolicyResponse: api.PolicyResponse{
					Rules: []api.RuleResponse{
						{
							Name:    "rule-pass",
							Status:  api.RuleStatusPass,
							Message: "message pass",
						},
						{
							Name:    "rule-warn",
							Status:  api.RuleStatusWarn,
							Message: "message warn",
						},
						{
							Name:    "rule-fail",
							Status:  api.RuleStatusFail,
							Message: "message fail",
						},
						{
							Name:    "rule-error",
							Status:  api.RuleStatusError,
							Message: "message error",
						},
						{
							Name:    "rule-skip",
							Status:  api.RuleStatusSkip,
							Message: "message skip",
						},
					},
				},
			},
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
