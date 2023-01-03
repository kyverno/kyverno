package utils

import (
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
)

func Test_getAction(t *testing.T) {
	type args struct {
		hasViolations bool
		i             int
	}
	tests := []struct {
		name string
		args args
		want string
	}{{
		name: "violation",
		args: args{true, 1},
		want: "violation",
	}, {
		name: "violations",
		args: args{true, 5},
		want: "violations",
	}, {
		name: "error",
		args: args{false, 1},
		want: "error",
	}, {
		name: "errors",
		args: args{false, 5},
		want: "errors",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getAction(tt.args.hasViolations, tt.args.i)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBlockRequest(t *testing.T) {
	type args struct {
		engineResponses []*api.EngineResponse
		failurePolicy   kyvernov1.FailurePolicyType
		log             logr.Logger
	}
	tests := []struct {
		name string
		args args
		want bool
	}{{
		name: "failure - enforce",
		args: args{
			engineResponses: []*api.EngineResponse{
				{
					PolicyResponse: api.PolicyResponse{
						ValidationFailureAction: "Enforce",
						Rules: []api.RuleResponse{
							{
								Name:    "rule-fail",
								Status:  api.RuleStatusFail,
								Message: "message fail",
							},
						},
					},
				},
			},
			failurePolicy: kyvernov1.Fail,
			log:           logr.Discard(),
		},
		want: true,
	}, {
		name: "failure - audit",
		args: args{
			engineResponses: []*api.EngineResponse{
				{
					PolicyResponse: api.PolicyResponse{
						ValidationFailureAction: "Audit",
						Rules: []api.RuleResponse{
							{
								Name:    "rule-fail",
								Status:  api.RuleStatusFail,
								Message: "message fail",
							},
						},
					},
				},
			},
			failurePolicy: kyvernov1.Fail,
			log:           logr.Discard(),
		},
		want: false,
	}, {
		name: "error - fail",
		args: args{
			engineResponses: []*api.EngineResponse{
				{
					PolicyResponse: api.PolicyResponse{
						ValidationFailureAction: "Audit",
						Rules: []api.RuleResponse{
							{
								Name:    "rule-error",
								Status:  api.RuleStatusError,
								Message: "message error",
							},
						},
					},
				},
			},
			failurePolicy: kyvernov1.Fail,
			log:           logr.Discard(),
		},
		want: true,
	}, {
		name: "error - ignore",
		args: args{
			engineResponses: []*api.EngineResponse{
				{
					PolicyResponse: api.PolicyResponse{
						ValidationFailureAction: "Audit",
						Rules: []api.RuleResponse{
							{
								Name:    "rule-error",
								Status:  api.RuleStatusError,
								Message: "message error",
							},
						},
					},
				},
			},
			failurePolicy: kyvernov1.Ignore,
			log:           logr.Discard(),
		},
		want: false,
	}, {
		name: "warning - ignore",
		args: args{
			engineResponses: []*api.EngineResponse{
				{
					PolicyResponse: api.PolicyResponse{
						ValidationFailureAction: "Audit",
						Rules: []api.RuleResponse{
							{
								Name:    "rule-warning",
								Status:  api.RuleStatusWarn,
								Message: "message warning",
							},
						},
					},
				},
			},
			failurePolicy: kyvernov1.Ignore,
			log:           logr.Discard(),
		},
		want: false,
	}, {
		name: "warning - fail",
		args: args{
			engineResponses: []*api.EngineResponse{
				{
					PolicyResponse: api.PolicyResponse{
						ValidationFailureAction: "Audit",
						Rules: []api.RuleResponse{
							{
								Name:    "rule-warning",
								Status:  api.RuleStatusWarn,
								Message: "message warning",
							},
						},
					},
				},
			},
			failurePolicy: kyvernov1.Fail,
			log:           logr.Discard(),
		},
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BlockRequest(tt.args.engineResponses, tt.args.failurePolicy, tt.args.log)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetBlockedMessages(t *testing.T) {
	type args struct {
		engineResponses []*api.EngineResponse
	}
	tests := []struct {
		name string
		args args
		want string
	}{{
		name: "failure - enforce",
		args: args{
			engineResponses: []*api.EngineResponse{
				{
					PolicyResponse: api.PolicyResponse{
						Policy: api.PolicySpec{
							Name: "test",
						},
						ValidationFailureAction: "Enforce",
						Rules: []api.RuleResponse{
							{
								Name:    "rule-fail",
								Status:  api.RuleStatusFail,
								Message: "message fail",
							},
						},
						Resource: api.ResourceSpec{
							Kind:      "foo",
							Namespace: "bar",
							Name:      "baz",
						},
					},
				},
			},
		},
		want: "\n\npolicy foo/bar/baz for resource violation: \n\ntest:\n  rule-fail: message fail\n",
	}, {
		name: "error - enforce",
		args: args{
			engineResponses: []*api.EngineResponse{
				{
					PolicyResponse: api.PolicyResponse{
						Policy: api.PolicySpec{
							Name: "test",
						},
						ValidationFailureAction: "Enforce",
						Rules: []api.RuleResponse{
							{
								Name:    "rule-error",
								Status:  api.RuleStatusError,
								Message: "message error",
							},
						},
						Resource: api.ResourceSpec{
							Kind:      "foo",
							Namespace: "bar",
							Name:      "baz",
						},
					},
				},
			},
		},
		want: "\n\npolicy foo/bar/baz for resource error: \n\ntest:\n  rule-error: message error\n",
	}, {
		name: "error and failure - enforce",
		args: args{
			engineResponses: []*api.EngineResponse{
				{
					PolicyResponse: api.PolicyResponse{
						Policy: api.PolicySpec{
							Name: "test",
						},
						ValidationFailureAction: "Enforce",
						Rules: []api.RuleResponse{
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
						},
						Resource: api.ResourceSpec{
							Kind:      "foo",
							Namespace: "bar",
							Name:      "baz",
						},
					},
				},
			},
		},
		want: "\n\npolicy foo/bar/baz for resource violation: \n\ntest:\n  rule-error: message error\n  rule-fail: message fail\n",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetBlockedMessages(tt.args.engineResponses)
			assert.Equal(t, tt.want, got)
		})
	}
}
