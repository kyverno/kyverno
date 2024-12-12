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
		  "validationFailureAction": "Enforce",
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
