package resource

import (
	"encoding/json"
	"fmt"
	"testing"

	log "github.com/kyverno/kyverno/pkg/logging"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	"gotest.tools/assert"
)

func TestValidate_failure_action_overrides(t *testing.T) {

	testcases := []struct {
		rawPolicy   []byte
		rawResource []byte
		blocked     bool
		messages    map[string]string
	}{
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "validationFailureAction": "audit",
					   "validationFailureActionOverrides": 
							[
								{
									"action": "enforce",
									"namespaces": [
										"default"
									]
								},
								{
									"action": "audit",
									"namespaces": [
										"test"
									]
								}
							],
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"message": "The label 'app' is required.",
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
				 }
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
					   "namespace": "default"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: true,
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "validationFailureAction": "audit",
					   "validationFailureActionOverrides": 
							[
								{
									"action": "enforce",
									"namespaces": [
										"default"
									]
								},
								{
									"action": "audit",
									"namespaces": [
										"test"
									]
								}
							],
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"message": "The label 'app' is required.",
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
				 }
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
					   "labels": {
						   "app": "my-app"
					   }
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: false,
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "validationFailureAction": "audit",
					   "validationFailureActionOverrides": 
							[
								{
									"action": "enforce",
									"namespaces": [
										"default"
									]
								},
								{
									"action": "audit",
									"namespaces": [
										"test"
									]
								}
							],
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"message": "The label 'app' is required.",
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
				 }
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
					   "namespace": "test"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: false,
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "validationFailureAction": "enforce",
					   "validationFailureActionOverrides": 
							[
								{
									"action": "enforce",
									"namespaces": [
										"default"
									]
								},
								{
									"action": "audit",
									"namespaces": [
										"test"
									]
								}
							],
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"message": "The label 'app' is required.",
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
				 }
		 	`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
					   "namespace": "default"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: true,
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "validationFailureAction": "enforce",
					   "validationFailureActionOverrides": 
							[
								{
									"action": "enforce",
									"namespaces": [
										"default"
									]
								},
								{
									"action": "audit",
									"namespaces": [
										"test"
									]
								}
							],
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"message": "The label 'app' is required.",
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
			 	}
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
					   "labels": {
						   "app": "my-app"
					   }
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: false,
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "validationFailureAction": "enforce",
					   "validationFailureActionOverrides": 
							[
								{
									"action": "enforce",
									"namespaces": [
										"default"
									]
								},
								{
									"action": "audit",
									"namespaces": [
										"test"
									]
								}
							],
					   "rules": [
						  {
							 "name": "check-label-app",
							 "match": {
								"resources": {
								   "kinds": [
									  "Pod"
								   ]
								}
							 },
							 "validate": {
								"message": "The label 'app' is required.",
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
				 }
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
					   "namespace": "test"
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: false,
		},
		{
			rawPolicy: []byte(`
				{
					"apiVersion": "kyverno.io/v1",
					"kind": "ClusterPolicy",
					"metadata": {
					   "name": "check-label-app"
					},
					"spec": {
					   "validationFailureAction": "enforce",
					   "validationFailureActionOverrides": 
							[
								{
									"action": "enforce",
									"namespaces": [
										"default"
									]
								},
								{
									"action": "audit",
									"namespaces": [
										"test"
									]
								}
							],
					   "rules": [
						  {
							"name": "check-label-app",
							"match": {
							   "resources": {
								  "kinds": [
									 "Pod"
								  ]
							   }
							},
							"validate": {
							   "message": "The label 'app' is required.",
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
				 }
			`),
			rawResource: []byte(`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
					   "name": "test-pod",
					   "namespace": ""
					},
					"spec": {
					   "containers": [
						  {
							 "name": "nginx",
							 "image": "nginx:latest"
						  }
					   ]
					}
				 }
			`),
			blocked: true,
			messages: map[string]string{
				"check-label-app": "validation error: The label 'app' is required. rule check-label-app failed at path /metadata/labels/",
			},
		},
	}

	for i, tc := range testcases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			var policy kyvernov1.ClusterPolicy
			err := json.Unmarshal(tc.rawPolicy, &policy)
			assert.NilError(t, err)
			resourceUnstructured, err := utils.ConvertToUnstructured(tc.rawResource)
			assert.NilError(t, err)

			er := engine.Validate(&engine.PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
			if tc.blocked && tc.messages != nil {
				for _, r := range er.PolicyResponse.Rules {
					msg := tc.messages[r.Name]
					assert.Equal(t, r.Message, msg)
				}
			}

			failurePolicy := kyvernov1.Fail
			blocked := webhookutils.BlockRequest([]*response.EngineResponse{er}, failurePolicy, log.WithName("WebhookServer"))
			assert.Assert(t, tc.blocked == blocked)
		})
	}
}

func Test_RuleSelector(t *testing.T) {
	var rawPolicy = []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {"name": "check-label-app"},
		"spec": {
		   "validationFailureAction": "enforce",
		   "rules": [
			  {
				"name": "check-label-test",
				"match": {"name": "test-*", "resources": {"kinds": ["Pod"]}},
				"validate": {
				   "message": "The label 'app' is required.",
				   "pattern": { "metadata": { "labels": { "app": "?*" } } } 
				}
			  },
			  {
				"name": "check-labels",
				"match": {"name": "*", "resources": {"kinds": ["Pod"]}},
				"validate": {
				   "message": "The label 'app' is required.",
				   "pattern": { "metadata": { "labels": { "app": "?*", "test" : "?*" } } } 
				}
			  }
		   ]
		}
	 }`)

	var rawResource = []byte(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {"name": "test-pod", "namespace": "", "labels": { "app" : "test-pod" }},
		"spec": {"containers": [{"name": "nginx", "image": "nginx:latest"}]}
	}`)

	var policy kyvernov1.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	assert.Assert(t, resourceUnstructured != nil)

	ctx := &engine.PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()}
	resp := engine.Validate(ctx)
	assert.Assert(t, resp.PolicyResponse.RulesAppliedCount == 2)
	assert.Assert(t, resp.PolicyResponse.RulesErrorCount == 0)

	log := log.WithName("Test_RuleSelector")
	blocked := webhookutils.BlockRequest([]*response.EngineResponse{resp}, kyvernov1.Fail, log)
	assert.Assert(t, blocked == true)

	applyOne := kyvernov1.ApplyOne
	policy.Spec.ApplyRules = &applyOne

	resp = engine.Validate(ctx)
	assert.Assert(t, resp.PolicyResponse.RulesAppliedCount == 1)
	assert.Assert(t, resp.PolicyResponse.RulesErrorCount == 0)

	blocked = webhookutils.BlockRequest([]*response.EngineResponse{resp}, kyvernov1.Fail, log)
	assert.Assert(t, blocked == false)
}
