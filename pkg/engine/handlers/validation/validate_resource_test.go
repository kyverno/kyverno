package validation

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	"github.com/stretchr/testify/assert"
)

func Test_validateOldObject(t *testing.T) {
	mockCL := func(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error {
		return nil
	}

	policyContext := buildTestNamespaceLabelsContext(t, validateDenyPolicy, resource, oldResource)
	rule := policyContext.Policy().GetSpec().Rules[0]
	v := newValidator(logr.Discard(), mockCL, policyContext, rule)

	ctx := context.TODO()
	resp := v.validate(ctx)
	assert.NotNil(t, resp)
	assert.Equal(t, api.RuleStatusPass, resp.Status())

	rule2 := policyContext.Policy().GetSpec().Rules[1]
	v2 := newValidator(logr.Discard(), mockCL, policyContext, rule2)
	resp2 := v2.validate(ctx)
	assert.NotNil(t, resp2 != nil)
	assert.Equal(t, api.RuleStatusFail, resp2.Status())
}

func buildTestNamespaceLabelsContext(t *testing.T, policy string, resource string, oldResource string) api.PolicyContext {
	return buildContext(t, kyvernov1.Update, policy, resource, oldResource)
}

func Test_validateOldObjectForeach(t *testing.T) {
	mockCL := func(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error {
		return nil
	}

	policyContext := buildTestNamespaceLabelsContext(t, validateForeachPolicy, resource, oldResource)
	rule := policyContext.Policy().GetSpec().Rules[0]
	v := newValidator(logr.Discard(), mockCL, policyContext, rule)

	ctx := context.TODO()
	resp := v.validate(ctx)
	assert.NotNil(t, resp)
	assert.Equal(t, api.RuleStatusSkip, resp.Status())
}

func Test_validateForEach_ElementError_NonLastElement_ReturnsError(t *testing.T) {
	// A context loader that always fails, simulating an API call timeout
	failingCL := func(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error {
		if len(contextEntries) > 0 {
			return fmt.Errorf("simulated API call timeout")
		}
		return nil
	}

	// The resource has 2 containers — the error occurs on element 0 (not the last).
	// Before the fix, this would silently continue and return Pass.
	policyContext := buildTestNamespaceLabelsContext(t, validateForeachWithContextPolicy, resource, oldResource)
	rule := policyContext.Policy().GetSpec().Rules[0]
	v := newValidator(logr.Discard(), failingCL, policyContext, rule)

	ctx := context.TODO()
	resp := v.validateForEach(ctx)

	assert.NotNil(t, resp, "validateForEach should return error when a non-last element fails")
	assert.Equal(t, api.RuleStatusError, resp.Status(), "status should be Error when any element's context loading fails")
}

func Test_validateForEach_ListEvalError_ReturnsError(t *testing.T) {
	mockCL := func(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error {
		return nil
	}

	policyContext := buildTestNamespaceLabelsContext(t, validateForeachInvalidListPolicy, resource, oldResource)
	rule := policyContext.Policy().GetSpec().Rules[0]
	v := newValidator(logr.Discard(), mockCL, policyContext, rule)

	ctx := context.TODO()
	resp := v.validateForEach(ctx)

	assert.NotNil(t, resp, "validateForEach should return error response when list evaluation fails")
	assert.Equal(t, api.RuleStatusError, resp.Status(), "status should be Error when list evaluation fails")
}

// Test_validateOldObjectContextReload verifies that rule-level context variables
// are re-evaluated against the old resource, not reused from the new resource.
func Test_validateOldObjectContextReload(t *testing.T) {
	contextLoaderFactory := factories.DefaultContextLoaderFactory(nil)
	policyContext := buildTestNamespaceLabelsContext(t, validateContextVarPolicy, podWithProdLabel, podWithDevLabel)
	rule := policyContext.Policy().GetSpec().Rules[0]
	loader := contextLoaderFactory(policyContext.Policy(), rule)

	engineCL := func(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error {
		return loader.Load(ctx, jp, nil, nil, contextEntries, jsonContext)
	}

	// simulate engine pre-loading rule context before calling the handler
	assert.NoError(t, engineCL(context.TODO(), rule.Context, policyContext.JSONContext()))

	v := newValidator(logr.Discard(), engineCL, policyContext, rule)
	resp := v.validate(context.TODO())
	assert.NotNil(t, resp)
	assert.Equal(t, api.RuleStatusFail, resp.Status())
}

var (
	validateDenyPolicy = `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "block-label-changes"
		},
		"spec": {
		  "background": false,
		  "rules": [
			{
			  "name": "require-labels",
			  "match": {
				"all": [
				  {
					"resources": {
					  "operations": [
						"CREATE",
						"UPDATE"
					  ],
					  "kinds": [
						"Pod"
					  ]
					}
				  }
				]
			  },
			  "validate": {
			    "failureAction": "Enforce",
				"message": "The label size is required",
				"pattern": {
				  "metadata": {
					"labels": {
					  "size": "small | medium | large"
					}
				  }
				}
			  }
			},
			{
			  "name": "check-mutable-labels",
			  "match": {
				"all": [
				  {
					"resources": {
					  "operations": [
						"UPDATE"
					  ],
					  "kinds": [
						"Pod"
					  ]
					}
				  }
				]
			  },
			  "validate": {
			    "failureAction": "Enforce",
				"message": "The label size cannot be changed for a namespace",
				"deny": {
				  "conditions": {
					"all": [
					  {
						"key": "{{ request.object.metadata.labels.size || '' }}",
						"operator": "NotEquals",
						"value": "{{ request.oldObject.metadata.labels.size }}"
					  }
					]
				  }
				}
			  }
			}
		  ]
		}
	}`

	validateForeachPolicy = `{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "validate-image-list"
  },
  "spec": {
    "admission": true,
    "background": true,
    "rules": [
      {
        "match": {
          "any": [
            {
              "resources": {
                "kinds": [
                  "Pod"
                ]
              }
            }
          ]
        },
        "name": "check-image",
        "validate": {
    	    "failureAction": "Enforce",
		      "allowExistingViolations": true,
            "foreach": [
            {
              "deny": {
                "conditions": {
                  "all": [
                    {
                      "key": "{{ element }}",
                      "operator": "NotEquals",
                      "value": "ghcr.io"
                    }
                  ]
                }
              },
              "list": "request.object.spec.containers[].image"
            }
          ],
          "message": "images must begin with ghcr.io"
        }
      }
    ]
  }
}
	`

	resource = `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
		  "annotations": {},
		  "labels": {
			"kubernetes.io/metadata.name": "test",
			"size": "large"
		  },
		  "name": "test"
		},
		"spec": {
			"containers": [
				{
					"image": "ghcr.io/test-webserver",
					"name": "test1",
					"volumeMounts": [
						{
							"mountPath": "/tmp/cache",
							"name": "cache-volume"
						}
					]
				},
				{
					"image": "ghcr.io/test-webserver",
					"name": "test2",
					"volumeMounts": [
						{
							"mountPath": "/tmp/cache",
							"name": "cache-volume"
						},
						{
							"mountPath": "/gce",
							"name": "gce"
						}
					]
				}
			],
			"volumes": [
				{
					"name": "cache-volume",
					"emptyDir": {}
				},
				{
					"name": "gce",
					"gcePersistentDisk": {}
				}
			]
		}
	}`

	oldResource = `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
		  "labels": {
			"kubernetes.io/metadata.name": "test",
			"size": "small"
		  },
		  "name": "test"
		},
		"spec": {
			"containers": [
				{
					"image": "ghcr.io/test-webserver",
					"name": "test1",
					"volumeMounts": [
						{
							"mountPath": "/tmp/cache",
							"name": "cache-volume"
						}
					]
				},
				{
					"image": "ghcr.io/test-webserver",
					"name": "test2",
					"volumeMounts": [
						{
							"mountPath": "/tmp/cache",
							"name": "cache-volume"
						},
						{
							"mountPath": "/gce",
							"name": "gce"
						}
					]
				}
			],
			"volumes": [
				{
					"name": "cache-volume",
					"emptyDir": {}
				},
				{
					"name": "gce",
					"gcePersistentDisk": {}
				}
			]
		}
	}`

	validateForeachWithContextPolicy = `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {"name": "test-foreach-context"},
		"spec": {
			"rules": [{
				"name": "check-images",
				"match": {"any": [{"resources": {"kinds": ["Pod"]}}]},
				"validate": {
					"failureAction": "Enforce",
					"foreach": [{
						"list": "request.object.spec.containers[]",
						"context": [{
							"name": "registry",
							"variable": {"value": "test"}
						}],
						"deny": {
							"conditions": {
								"all": [{
									"key": "{{ element.name }}",
									"operator": "Equals",
									"value": "blocked"
								}]
							}
						}
					}]
				}
			}]
		}
	}`

	validateForeachInvalidListPolicy = `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {"name": "test-invalid-list"},
		"spec": {
			"rules": [{
				"name": "invalid-list-rule",
				"match": {"any": [{"resources": {"kinds": ["Pod"]}}]},
				"validate": {
					"failureAction": "Enforce",
					"foreach": [{
						"list": "invalid_jmespath_expression[",
						"deny": {"conditions": {"all": [{"key": "{{ element }}", "operator": "Equals", "value": "test"}]}}
					}]
				}
			}]
		}
	}`

	validateContextVarPolicy = `{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "check-env-label"
		},
		"spec": {
		  "background": false,
		  "rules": [
			{
			  "name": "deny-prod-env",
			  "context": [
				{
				  "name": "currentEnv",
				  "variable": {
					"jmesPath": "request.object.metadata.labels.env || ''"
				  }
				}
			  ],
			  "match": {
				"any": [
				  {
					"resources": {
					  "kinds": ["Pod"],
					  "operations": ["UPDATE"]
					}
				  }
				]
			  },
			  "validate": {
				"failureAction": "Enforce",
				"allowExistingViolations": true,
				"message": "prod env is not allowed",
				"deny": {
				  "conditions": {
					"all": [
					  {
						"key": "{{ currentEnv }}",
						"operator": "Equals",
						"value": "prod"
					  }
					]
				  }
				}
			  }
			}
		  ]
		}
	}`

	podWithProdLabel = `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
		  "name": "test",
		  "labels": {"env": "prod"}
		},
		"spec": {
		  "containers": [{"name": "c", "image": "nginx"}]
		}
	}`

	podWithDevLabel = `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
		  "name": "test",
		  "labels": {"env": "dev"}
		},
		"spec": {
		  "containers": [{"name": "c", "image": "nginx"}]
		}
	}`
)
