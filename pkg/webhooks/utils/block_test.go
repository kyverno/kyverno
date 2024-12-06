package utils

import (
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	auditPolicy := engineapi.NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
		ObjectMeta: v1.ObjectMeta{
			Name: "test",
		},
		Spec: kyvernov1.Spec{
			ValidationFailureAction: kyvernov1.Audit,
		},
	})
	enforcePolicy := engineapi.NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
		ObjectMeta: v1.ObjectMeta{
			Name: "test",
		},
		Spec: kyvernov1.Spec{
			ValidationFailureAction: kyvernov1.Enforce,
		},
	})
	audit := kyvernov1.Audit
	auditRule := engineapi.NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
		ObjectMeta: v1.ObjectMeta{
			Name: "test",
		},
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{
				{
					Name: "rule-audit",
					Validation: &kyvernov1.Validation{
						FailureAction: &audit,
					},
				},
			},
		},
	})
	enforce := kyvernov1.Enforce
	enforceRule := engineapi.NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
		ObjectMeta: v1.ObjectMeta{
			Name: "test",
		},
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{
				{
					Name: "rule-enforce",
					Validation: &kyvernov1.Validation{
						FailureAction: &enforce,
					},
				},
			},
		},
	})
	resource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind": "foo",
			"metadata": map[string]interface{}{
				"namespace": "bar",
				"name":      "baz",
			},
		},
	}
	type args struct {
		engineResponses []engineapi.EngineResponse
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
			engineResponses: []engineapi.EngineResponse{
				engineapi.NewEngineResponse(resource, enforcePolicy, nil).WithPolicyResponse(engineapi.PolicyResponse{
					Rules: []engineapi.RuleResponse{
						*engineapi.RuleFail("rule-fail", engineapi.Validation, "message fail", nil),
					},
				}),
			},
			failurePolicy: kyvernov1.Fail,
			log:           logr.Discard(),
		},
		want: true,
	}, {
		name: "failure - audit",
		args: args{
			engineResponses: []engineapi.EngineResponse{
				engineapi.NewEngineResponse(resource, auditPolicy, nil).WithPolicyResponse(engineapi.PolicyResponse{
					Rules: []engineapi.RuleResponse{
						*engineapi.RuleFail("rule-fail", engineapi.Validation, "message fail", nil),
					},
				}),
			},
			failurePolicy: kyvernov1.Fail,
			log:           logr.Discard(),
		},
		want: false,
	}, {
		name: "error - fail",
		args: args{
			engineResponses: []engineapi.EngineResponse{
				engineapi.NewEngineResponse(resource, auditPolicy, nil).WithPolicyResponse(engineapi.PolicyResponse{
					Rules: []engineapi.RuleResponse{
						*engineapi.RuleError("rule-error", engineapi.Validation, "message error", nil, nil),
					},
				}),
			},
			failurePolicy: kyvernov1.Fail,
			log:           logr.Discard(),
		},
		want: true,
	}, {
		name: "error - ignore",
		args: args{
			engineResponses: []engineapi.EngineResponse{
				engineapi.NewEngineResponse(resource, auditPolicy, nil).WithPolicyResponse(engineapi.PolicyResponse{
					Rules: []engineapi.RuleResponse{
						*engineapi.RuleError("rule-error", engineapi.Validation, "message error", nil, nil),
					},
				}),
			},
			failurePolicy: kyvernov1.Ignore,
			log:           logr.Discard(),
		},
		want: false,
	}, {
		name: "warning - ignore",
		args: args{
			engineResponses: []engineapi.EngineResponse{
				engineapi.NewEngineResponse(resource, auditPolicy, nil).WithPolicyResponse(engineapi.PolicyResponse{
					Rules: []engineapi.RuleResponse{
						*engineapi.NewRuleResponse("rule-warning", engineapi.Validation, "message warning", engineapi.RuleStatusWarn, nil),
					},
				}),
			},
			failurePolicy: kyvernov1.Ignore,
			log:           logr.Discard(),
		},
		want: false,
	}, {
		name: "warning - fail",
		args: args{
			engineResponses: []engineapi.EngineResponse{
				engineapi.NewEngineResponse(resource, auditPolicy, nil).WithPolicyResponse(engineapi.PolicyResponse{
					Rules: []engineapi.RuleResponse{
						*engineapi.NewRuleResponse("rule-warning", engineapi.Validation, "message warning", engineapi.RuleStatusWarn, nil),
					},
				}),
			},
			failurePolicy: kyvernov1.Fail,
			log:           logr.Discard(),
		},
		want: false,
	}, {
		name: "failure - enforce",
		args: args{
			engineResponses: []engineapi.EngineResponse{
				engineapi.NewEngineResponse(resource, enforceRule, nil).WithPolicyResponse(engineapi.PolicyResponse{
					Rules: []engineapi.RuleResponse{
						*engineapi.RuleFail("rule-fail", engineapi.Validation, "message fail", nil),
					},
				}),
			},
			failurePolicy: kyvernov1.Fail,
			log:           logr.Discard(),
		},
		want: true,
	}, {
		name: "failure - audit",
		args: args{
			engineResponses: []engineapi.EngineResponse{
				engineapi.NewEngineResponse(resource, auditRule, nil).WithPolicyResponse(engineapi.PolicyResponse{
					Rules: []engineapi.RuleResponse{
						*engineapi.RuleFail("rule-fail", engineapi.Validation, "message fail", nil),
					},
				}),
			},
			failurePolicy: kyvernov1.Fail,
			log:           logr.Discard(),
		},
		want: false,
	}, {
		name: "error - fail",
		args: args{
			engineResponses: []engineapi.EngineResponse{
				engineapi.NewEngineResponse(resource, auditRule, nil).WithPolicyResponse(engineapi.PolicyResponse{
					Rules: []engineapi.RuleResponse{
						*engineapi.RuleError("rule-error", engineapi.Validation, "message error", nil, nil),
					},
				}),
			},
			failurePolicy: kyvernov1.Fail,
			log:           logr.Discard(),
		},
		want: true,
	}, {
		name: "error - ignore",
		args: args{
			engineResponses: []engineapi.EngineResponse{
				engineapi.NewEngineResponse(resource, auditRule, nil).WithPolicyResponse(engineapi.PolicyResponse{
					Rules: []engineapi.RuleResponse{
						*engineapi.RuleError("rule-error", engineapi.Validation, "message error", nil, nil),
					},
				}),
			},
			failurePolicy: kyvernov1.Ignore,
			log:           logr.Discard(),
		},
		want: false,
	}, {
		name: "warning - ignore",
		args: args{
			engineResponses: []engineapi.EngineResponse{
				engineapi.NewEngineResponse(resource, auditRule, nil).WithPolicyResponse(engineapi.PolicyResponse{
					Rules: []engineapi.RuleResponse{
						*engineapi.NewRuleResponse("rule-warning", engineapi.Validation, "message warning", engineapi.RuleStatusWarn, nil),
					},
				}),
			},
			failurePolicy: kyvernov1.Ignore,
			log:           logr.Discard(),
		},
		want: false,
	}, {
		name: "warning - fail",
		args: args{
			engineResponses: []engineapi.EngineResponse{
				engineapi.NewEngineResponse(resource, auditRule, nil).WithPolicyResponse(engineapi.PolicyResponse{
					Rules: []engineapi.RuleResponse{
						*engineapi.NewRuleResponse("rule-warning", engineapi.Validation, "message warning", engineapi.RuleStatusWarn, nil),
					},
				}),
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
	enforcePolicy := engineapi.NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
		ObjectMeta: v1.ObjectMeta{
			Name: "test",
		},
		Spec: kyvernov1.Spec{
			ValidationFailureAction: kyvernov1.Enforce,
		},
	})
	resource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind": "foo",
			"metadata": map[string]interface{}{
				"namespace": "bar",
				"name":      "baz",
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
	}{{
		name: "failure - enforce",
		args: args{
			engineResponses: []engineapi.EngineResponse{
				engineapi.NewEngineResponse(resource, enforcePolicy, nil).WithPolicyResponse(engineapi.PolicyResponse{
					Rules: []engineapi.RuleResponse{
						*engineapi.RuleFail("rule-fail", engineapi.Validation, "message fail", nil),
					},
				}),
			},
		},
		want: "\n\nresource foo/bar/baz was blocked due to the following policies \n\ntest:\n  rule-fail: message fail\n",
	}, {
		name: "error - enforce",
		args: args{
			engineResponses: []engineapi.EngineResponse{
				engineapi.NewEngineResponse(resource, enforcePolicy, nil).WithPolicyResponse(engineapi.PolicyResponse{
					Rules: []engineapi.RuleResponse{
						*engineapi.RuleError("rule-error", engineapi.Validation, "message error", nil, nil),
					},
				}),
			},
		},
		want: "\n\nresource foo/bar/baz was blocked due to the following policies \n\ntest:\n  rule-error: message error\n",
	}, {
		name: "error and failure - enforce",
		args: args{
			engineResponses: []engineapi.EngineResponse{
				engineapi.NewEngineResponse(resource, enforcePolicy, nil).WithPolicyResponse(engineapi.PolicyResponse{
					Rules: []engineapi.RuleResponse{
						*engineapi.RuleFail("rule-fail", engineapi.Validation, "message fail", nil),
						*engineapi.RuleError("rule-error", engineapi.Validation, "message error", nil, nil),
					},
				}),
			},
		},
		want: "\n\nresource foo/bar/baz was blocked due to the following policies \n\ntest:\n  rule-error: message error\n  rule-fail: message fail\n",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetBlockedMessages(tt.args.engineResponses)
			assert.Equal(t, tt.want, got)
		})
	}
}
