package webhooks

import (
	"encoding/json"
	"fmt"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	log "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"gotest.tools/assert"
)

func TestValidate_failure_action_overrides(t *testing.T) {

	testcases := []struct {
		rawPolicy   []byte
		rawResource []byte
		blocked     bool
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
		},
	}

	for i, tc := range testcases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			var policy kyverno.ClusterPolicy
			err := json.Unmarshal(tc.rawPolicy, &policy)
			assert.NilError(t, err)
			resourceUnstructured, err := utils.ConvertToUnstructured(tc.rawResource)
			assert.NilError(t, err)
			msgs := []string{
				"validation error: The label 'app' is required. Rule check-label-app failed at path /metadata/labels/",
			}

			er := engine.Validate(&engine.PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
			if tc.blocked {
				for index, r := range er.PolicyResponse.Rules {
					assert.Equal(t, r.Message, msgs[index])
				}
			}

			blocked := toBlockResource([]*response.EngineResponse{er}, log.Log.WithName("WebhookServer"))
			assert.Assert(t, tc.blocked == blocked)
		})
	}
}
