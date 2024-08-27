package validation

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/stretchr/testify/assert"
)

func Test_validateOldObject(t *testing.T) {
	mockCL := func(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error {
		return nil
	}

	policyContext := buildTestNamespaceLabelsContext(t)
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

func buildTestNamespaceLabelsContext(t *testing.T) api.PolicyContext {
	policy := `{
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
						"Namespace"
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
						"Namespace"
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

	resource := `{
		"apiVersion": "v1",
		"kind": "Namespace",
		"metadata": {
		  "annotations": {},
		  "labels": {
			"kubernetes.io/metadata.name": "test",
			"size": "large"
		  },
		  "name": "test"
		},
		"spec": {}
	  }`

	oldResource := `{
		"apiVersion": "v1",
		"kind": "Namespace",
		"metadata": {
		  "labels": {
			"kubernetes.io/metadata.name": "test",
			"size": "small"
		  },
		  "name": "test"
		},
		"spec": {}
	  }`

	return buildContext(t, kyvernov1.Update, policy, resource, oldResource)
}

func Test_validateResourceWithVariableSubstitution(t *testing.T) {
	mockCL := func(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error {
		if err := jsonContext.AddVariable("bar", "hello"); err != nil {
			return err
		}
		return nil
	}

	policyContext := buildTestPodContext(t)
	rule := policyContext.Policy().GetSpec().Rules[0]
	v := newValidator(logr.Discard(), mockCL, policyContext, rule)

	ctx := context.TODO()
	resp := v.validate(ctx)
	assert.NotNil(t, resp)
	if resp.Status() == api.RuleStatusError {
		assert.Fail(t, "Policy validation failed with error: "+resp.Message())
	} else {
		assert.Equal(t, api.RuleStatusFail, resp.Status())
		assert.Contains(t, resp.Message(), "hello world!")
	}
}

func buildTestPodContext(t *testing.T) api.PolicyContext {
	policy := `{
        "apiVersion": "kyverno.io/v1",
        "kind": "ClusterPolicy",
        "metadata": {
            "name": "var-in-message"
        },
        "spec": {
            "rules": [
                {
                    "name": "display",
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
                    "context": [
                        {
                            "name": "foo",
                            "variable": {
                                "value": "hello"
                            }
                        }
                    ],
                    "validate": {
                        "message": "{{ bar }} world!",
                        "deny": {}
                    }
                }
            ]
        }
    }`

	resource := `{
        "apiVersion": "v1",
        "kind": "Pod",
        "metadata": {
            "name": "goodpod08"
        },
        "spec": {
            "initContainers": [
                {
                    "name": "initcontainer01",
                    "image": "docker.io/istio1/proxyv2",
                    "securityContext": {
                        "runAsUser": 0
                    }
                }
            ],
            "containers": [
                {
                    "name": "container01",
                    "image": "dummyimagename",
                    "securityContext": {
                        "runAsUser": 100
                    }
                }
            ]
        }
    }`

	return buildContext(t, kyvernov1.Update, policy, resource, "")
}
