package utils

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGenerateEvents(t *testing.T) {
	policy := engineapi.NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
		ObjectMeta: v1.ObjectMeta{
			Name: "test-policy",
		},
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{
				{
					Name: "validate-rule",
					Validation: &kyvernov1.Validation{
						Message: "ok",
					},
				},
			},
		},
	})

	resource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"namespace": "default",
				"name":      "mypod",
			},
		},
	}

	type args struct {
		engineResponses []engineapi.EngineResponse
		blocked         bool
	}

	tests := []struct {
		name      string
		args      args
		wantCount int
		wantCheck func(t *testing.T, events []event.Info)
	}{
		{
			name: "failed rule generates policy and resource violation events",
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
				blocked: false,
			},
			wantCount: 2,
			wantCheck: func(t *testing.T, events []event.Info) {
				assert.Equal(t, event.PolicyViolation, events[0].Reason)
				assert.Equal(t, event.ResourcePassed, events[0].Action)
				assert.Contains(t, events[0].Message, "privileged containers are not allowed")

				assert.Equal(t, event.PolicyViolation, events[1].Reason)
				assert.Equal(t, event.ResourcePassed, events[1].Action)
			},
		},
		{
			name: "blocked failure marks policy event as blocked",
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
				blocked: true,
			},
			wantCount: 1,
			wantCheck: func(t *testing.T, events []event.Info) {
				assert.Equal(t, event.ResourceBlocked, events[0].Action)
				assert.Contains(t, events[0].Message, "(blocked)")
			},
		},
		{
			name: "successful policy generates policy applied event",
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
				blocked: false,
			},
			wantCount: 1,
			wantCheck: func(t *testing.T, events []event.Info) {
				assert.Equal(t, event.PolicyApplied, events[0].Reason)
				assert.Equal(t, event.ResourcePassed, events[0].Action)
				assert.Contains(t, events[0].Message, "pass")
			},
		},
		{
			name: "error rule generates violation events",
			args: args{
				engineResponses: []engineapi.EngineResponse{
					engineapi.NewEngineResponse(resource, policy, nil).
						WithPolicyResponse(engineapi.PolicyResponse{
							Rules: []engineapi.RuleResponse{
								*engineapi.RuleError(
									"rule-error",
									engineapi.Validation,
									"something went wrong",
									nil,
									nil,
								),
							},
						}),
				},
				blocked: false,
			},
			wantCount: 2,
			wantCheck: func(t *testing.T, events []event.Info) {
				assert.Equal(t, event.PolicyViolation, events[0].Reason)
				assert.Contains(t, events[0].Message, "error")
			},
		},
		{
			name: "skipped rule with exception generates policy exception events",
			args: args{
				engineResponses: []engineapi.EngineResponse{
					engineapi.NewEngineResponse(resource, policy, nil).
						WithPolicyResponse(engineapi.PolicyResponse{
							Rules: []engineapi.RuleResponse{
								*engineapi.RuleSkip(
									"skip-rule",
									engineapi.Validation,
									"skipped",
									nil,
								).WithExceptions([]engineapi.GenericException{
									engineapi.NewPolicyException(
										&kyvernov2.PolicyException{
											ObjectMeta: metav1.ObjectMeta{
												Name:      "my-exception",
												Namespace: "default",
											},
										},
									),
								}),
							},
						}),
				},
				blocked: false,
			},
			wantCount: 2,
			wantCheck: func(t *testing.T, events []event.Info) {
				var exceptionEvent, policyEvent *event.Info

				for i := range events {
					if events[i].Regarding.Kind == "PolicyException" {
						exceptionEvent = &events[i]
					} else {
						policyEvent = &events[i]
					}
				}

				require.NotNil(t, exceptionEvent)
				require.NotNil(t, policyEvent)

				assert.Equal(t, event.PolicySkipped, exceptionEvent.Reason)
				assert.Equal(t, event.ResourcePassed, exceptionEvent.Action)
				assert.Contains(t, exceptionEvent.Message, "was skipped")

				assert.Equal(t, event.PolicySkipped, policyEvent.Reason)
				assert.Contains(t, policyEvent.Message, "policy exceptions")
			},
		},
		{
			name: "empty engine response produces no events",
			args: args{
				engineResponses: []engineapi.EngineResponse{
					engineapi.NewEngineResponse(resource, policy, nil),
				},
				blocked: false,
			},
			wantCount: 0,
			wantCheck: func(t *testing.T, events []event.Info) {},
		},
		{
			name: "resource without name is ignored",
			args: args{
				engineResponses: []engineapi.EngineResponse{
					engineapi.NewEngineResponse(
						unstructured.Unstructured{
							Object: map[string]interface{}{
								"kind": "Pod",
							},
						},
						policy,
						nil,
					),
				},
				blocked: false,
			},
			wantCount: 0,
			wantCheck: func(t *testing.T, events []event.Info) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := GenerateEvents(tt.args.engineResponses, tt.args.blocked)
			assert.Len(t, events, tt.wantCount)
			tt.wantCheck(t, events)
		})
	}
}
