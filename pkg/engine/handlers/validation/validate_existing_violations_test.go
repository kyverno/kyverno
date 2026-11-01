package validation

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
)

func TestAllowExistingViolationsWithPreconditions(t *testing.T) {
	const resourceTemplate = `{
		"apiVersion": "v1",
		"kind": "ConfigMap",
		"metadata": {
			"name": "demo-config",
			"namespace": "default"
		},
		"data": {
			"mode": %q
		}
	}`

	const policyTemplate = `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
			"name": "existing-violations-repro"
		},
		"spec": {
			"validationFailureAction": "Enforce",
			"rules": [{
				"name": "require-compliant-mode",
				"match": {
					"any": [{
						"resources": {
							"kinds": ["ConfigMap"]
						}
					}]
				}%s,
				"validate": {
					"allowExistingViolations": true,
					"message": "mode must equal 1",
					"deny": {
						"conditions": {
							"all": [{
								"key": "{{ request.object.data.mode }}",
								"operator": "NotEquals",
								"value": "1"
							}]
						}
					}
				}
			}]
		}
	}`

	const truePrecondition = `,"preconditions": {
		"all": [{
			"key": true,
			"operator": "Equals",
			"value": true
		}]
	}`

	const falsePrecondition = `,"preconditions": {
		"all": [{
			"key": true,
			"operator": "Equals",
			"value": false
		}]
	}`

	tests := []struct {
		name          string
		oldMode       string
		newMode       string
		preconditions string
		expected      engineapi.RuleStatus
	}{
		{
			name:          "existing violation with preconditions",
			oldMode:       "2",
			newMode:       "2",
			preconditions: truePrecondition,
			expected:      engineapi.RuleStatusSkip,
		},
		{
			name:     "existing violation without preconditions",
			oldMode:  "2",
			newMode:  "2",
			expected: engineapi.RuleStatusSkip,
		},
		{
			name:          "new violation must be rejected",
			oldMode:       "1",
			newMode:       "2",
			preconditions: truePrecondition,
			expected:      engineapi.RuleStatusFail,
		},
		{
			name:          "corrected violation must be allowed",
			oldMode:       "2",
			newMode:       "1",
			preconditions: truePrecondition,
			expected:      engineapi.RuleStatusPass,
		},
		{
			name:          "false precondition must skip validation",
			oldMode:       "2",
			newMode:       "2",
			preconditions: falsePrecondition,
			expected:      engineapi.RuleStatusSkip,
		},
	}

	loader := func(
		ctx context.Context,
		entries []kyvernov1.ContextEntry,
		jsonContext enginecontext.Interface,
	) error {
		return nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := fmt.Sprintf(policyTemplate, tt.preconditions)
			oldResource := fmt.Sprintf(resourceTemplate, tt.oldMode)
			newResource := fmt.Sprintf(resourceTemplate, tt.newMode)

			policyContext := buildContext(
				t,
				kyvernov1.Update,
				policy,
				newResource,
				oldResource,
			)

			rule := policyContext.Policy().GetSpec().Rules[0]
			validator := newValidator(
				logr.Discard(),
				loader,
				policyContext,
				rule,
			)

			response := validator.validate(context.Background())
			if response == nil {
				t.Fatal("expected a validation response")
			}

			if response.Status() != tt.expected {
				t.Errorf(
					"expected %v, got %v: %s",
					tt.expected,
					response.Status(),
					response.Message(),
				)
			}
		})
	}
}
