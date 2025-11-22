package validation

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func Test_validateOldObject_AllowExistingViolations_Config(t *testing.T) {
	var (
		policy = `{
			"apiVersion": "kyverno.io/v1",
			"kind": "ClusterPolicy",
			"metadata": {
			  "name": "require-labels"
			},
			"spec": {
			  "validationFailureAction": "Enforce",
			  "rules": [
				{
				  "name": "require-labels",
				  "match": {
					"all": [
					  {
						"resources": {
						  "kinds": [
							"Pod"
						  ]
						}
					  }
					]
				  },
				  "validate": {
					"message": "label 'app' is required",
					"pattern": {
					  "metadata": {
						"labels": {
						  "app": "?*"
						}
					  }
					}
				  }
				}
			  ]
			}
		}`
		policyWithAllowTrue = `{
			"apiVersion": "kyverno.io/v1",
			"kind": "ClusterPolicy",
			"metadata": {
			  "name": "require-labels"
			},
			"spec": {
			  "validationFailureAction": "Enforce",
			  "rules": [
				{
				  "name": "require-labels",
				  "match": {
					"all": [
					  {
						"resources": {
						  "kinds": [
							"Pod"
						  ]
						}
					  }
					]
				  },
				  "validate": {
					"allowExistingViolations": true,
					"message": "label 'app' is required",
					"pattern": {
					  "metadata": {
						"labels": {
						  "app": "?*"
						}
					  }
					}
				  }
				}
			  ]
			}
		}`
		policyWithAllowFalse = `{
			"apiVersion": "kyverno.io/v1",
			"kind": "ClusterPolicy",
			"metadata": {
			  "name": "require-labels"
			},
			"spec": {
			  "validationFailureAction": "Enforce",
			  "rules": [
				{
				  "name": "require-labels",
				  "match": {
					"all": [
					  {
						"resources": {
						  "kinds": [
							"Pod"
						  ]
						}
					  }
					]
				  },
				  "validate": {
					"allowExistingViolations": false,
					"message": "label 'app' is required",
					"pattern": {
					  "metadata": {
						"labels": {
						  "app": "?*"
						}
					  }
					}
				  }
				}
			  ]
			}
		}`
		resource = `{
			"apiVersion": "v1",
			"kind": "Pod",
			"metadata": {
			  "name": "test",
			  "labels": {
				"other": "value"
			  }
			},
			"spec": {
				"containers": [
					{
						"image": "nginx",
						"name": "nginx"
					}
				]
			}
		}`
		oldResource = `{
			"apiVersion": "v1",
			"kind": "Pod",
			"metadata": {
			  "name": "test",
			  "labels": {
				"other": "value"
			  }
			},
			"spec": {
				"containers": [
					{
						"image": "nginx",
						"name": "nginx"
					}
				]
			}
		}`
	)

	tests := []struct {
		name                           string
		policy                         string
		defaultAllowExistingViolations string
		expectedStatus                 engineapi.RuleStatus
	}{
		{
			name:                           "rule allow=true overrides default=false",
			policy:                         policyWithAllowTrue,
			defaultAllowExistingViolations: "false",
			expectedStatus:                 engineapi.RuleStatusSkip,
		},
		{
			name:                           "rule allow=false overrides default=true",
			policy:                         policyWithAllowFalse,
			defaultAllowExistingViolations: "true",
			expectedStatus:                 engineapi.RuleStatusFail,
		},
		{
			name:                           "default=true used when rule unset",
			policy:                         policy,
			defaultAllowExistingViolations: "true",
			expectedStatus:                 engineapi.RuleStatusSkip,
		},
		{
			name:                           "default=false used when rule unset",
			policy:                         policy,
			defaultAllowExistingViolations: "false",
			expectedStatus:                 engineapi.RuleStatusFail,
		},
		{
			name:                           "default unset (false) used when rule unset",
			policy:                         policy,
			defaultAllowExistingViolations: "",
			expectedStatus:                 engineapi.RuleStatusFail,
		},
		{
			name:                           "rule allow=false (explicit) with default=false",
			policy:                         policyWithAllowFalse,
			defaultAllowExistingViolations: "false",
			expectedStatus:                 engineapi.RuleStatusFail,
		},
		{
			name:                           "rule allow=true (explicit) with default=true",
			policy:                         policyWithAllowTrue,
			defaultAllowExistingViolations: "true",
			expectedStatus:                 engineapi.RuleStatusSkip,
		},
	}

	mockCL := func(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error {
		return nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.NewDefaultConfiguration(false)
			if tt.defaultAllowExistingViolations != "" {
				cm := &corev1.ConfigMap{
					Data: map[string]string{
						"defaultAllowExistingViolations": tt.defaultAllowExistingViolations,
					},
				}
				cfg.Load(cm)
			}

			policyContext := buildContextWithConfig(t, kyvernov1.Update, tt.policy, resource, oldResource, cfg)
			rule := policyContext.Policy().GetSpec().Rules[0]
			v := newValidator(logr.Discard(), mockCL, policyContext, rule)

			ctx := context.TODO()
			resp := v.validate(ctx)
			assert.NotNil(t, resp)
			assert.Equal(t, tt.expectedStatus, resp.Status())
		})
	}
}

func buildContextWithConfig(t *testing.T, operation kyvernov1.AdmissionOperation, policy, resource string, oldResource string, cfg config.Configuration) engineapi.PolicyContext {
	var cpol kyvernov1.ClusterPolicy
	err := json.Unmarshal([]byte(policy), &cpol)
	assert.NoError(t, err)

	resourceUnstructured, err := kubeutils.BytesToUnstructured([]byte(resource))
	assert.NoError(t, err)

	policyContext, err := policycontext.NewPolicyContext(
		jp,
		*resourceUnstructured,
		operation,
		nil,
		cfg,
	)
	assert.NoError(t, err)

	policyContext = policyContext.
		WithPolicy(&cpol).
		WithNewResource(*resourceUnstructured)

	if oldResource != "" {
		oldResourceUnstructured, err := kubeutils.BytesToUnstructured([]byte(oldResource))
		assert.NoError(t, err)

		err = enginecontext.AddOldResource(policyContext.JSONContext(), []byte(oldResource))
		assert.NoError(t, err)

		policyContext = policyContext.WithOldResource(*oldResourceUnstructured)
	}

	return policyContext
}
