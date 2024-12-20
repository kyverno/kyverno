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
)
