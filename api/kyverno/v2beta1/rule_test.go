package v2beta1

import (
	"encoding/json"
	"testing"

	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_Validate_RuleType_EmptyRule(t *testing.T) {
	subject := Rule{
		Name: "validate-user-privilege",
	}
	path := field.NewPath("dummy")
	errs := subject.Validate(path, false, "", nil)
	assert.Equal(t, len(errs), 1)
	assert.Equal(t, errs[0].Field, "dummy")
	assert.Equal(t, errs[0].Type, field.ErrorTypeInvalid)
	assert.Equal(t, errs[0].Detail, "No operation defined in the rule 'validate-user-privilege'.(supported operations: mutate,validate,generate,verifyImages)")
}

func Test_Validate_RuleType_MultipleRule(t *testing.T) {
	rawPolicy := []byte(`
	{
		"spec": {
		   "rules": [
			  {
				 "name": "validate-user-privilege",
				 "match": {
					"all": [
						{
							"resources": {
								"kinds": [
								"Deployment"
								],
								"selector": {
								"matchLabels": {
									"app.type": "prod"
								}
								}
							}
						}	 
					]
				 },
				 "mutate": {
					"patchStrategicMerge": {
					   "spec": {
						  "template": {
							 "spec": {
								"containers": [
								   {
									  "(name)": "*",
									  "resources": {
										 "limits": {
											"+(memory)": "300Mi",
											"+(cpu)": "100"
										 }
									  }
								   }
								]
							 }
						  }
					   }
					}
				 },
				 "validate": {
					"message": "validate container security contexts",
					"anyPattern": [
					   {
						  "spec": {
							 "template": {
								"spec": {
								   "containers": [
									  {
										 "securityContext": {
											"runAsNonRoot": true
										 }
									  }
								   ]
								}
							 }
						  }
					   }
					]
				 }
			  }
		   ]
		}
	 }`)

	var policy *ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)
	for _, rule := range policy.Spec.Rules {
		path := field.NewPath("dummy")
		errs := rule.Validate(path, false, "", nil)
		assert.Assert(t, len(errs) != 0)
	}
}

func Test_Validate_RuleType_SingleRule(t *testing.T) {
	rawPolicy := []byte(`
	{
		"spec": {
		   "rules": [
			  {
				 "name": "validate-user-privilege",
				 "match": {
					"all": [
						{
							"resources": {
							   "kinds": [
								  "Deployment"
							   ],
							   "selector": {
								  "matchLabels": {
									 "app.type": "prod"
								  }
							   }
							}
						}	 
					]
				 },
				 "validate": {
					"message": "validate container security contexts",
					"anyPattern": [
					   {
						  "spec": {
							 "template": {
								"spec": {
								   "containers": [
									  {
										 "securityContext": {
											"runAsNonRoot": "true"
										 }
									  }
								   ]
								}
							 }
						  }
					   }
					]
				 }
			  }
		   ]
		}
	 }
	`)

	var policy *ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)
	for _, rule := range policy.Spec.Rules {
		path := field.NewPath("dummy")
		errs := rule.Validate(path, false, "", nil)
		assert.Assert(t, len(errs) == 0)
	}
}

func Test_doesMatchExcludeConflict(t *testing.T) {
	path := field.NewPath("dummy")
	testcases := []struct {
		description string
		rule        []byte
		errors      func(r *Rule) field.ErrorList
	}{
		{
			description: "Same match and exclude",
			rule:        []byte(`{ "name": "set-image-pull-policy-2", "match": { "any": [ { "resources": { "kinds": [ "Pod", "Namespace" ], "name": "something", "namespaces": [ "something", "something1" ], "selector": { "matchLabels": { "memory": "high" }, "matchExpressions": [ { "key": "tier", "operator": "In", "values": [ "database" ] } ] } } } ], "subjects": [ { "name": "something", "kind": "something", "Namespace": "something", "apiGroup": "something" }, { "name": "something1", "kind": "something1", "Namespace": "something1", "apiGroup": "something1" } ], "clusterroles": [ "something", "something1" ], "roles": [ "something", "something1" ] }, "exclude": { "any": [ { "resources": { "kinds": [ "Pod", "Namespace" ], "name": "something", "namespaces": [ "something", "something1" ], "selector": { "matchLabels": { "memory": "high" }, "matchExpressions": [ { "key": "tier", "operator": "In", "values": [ "database" ] } ] } } } ], "subjects": [ { "name": "something", "kind": "something", "Namespace": "something", "apiGroup": "something" }, { "name": "something1", "kind": "something1", "Namespace": "something1", "apiGroup": "something1" } ], "clusterroles": [ "something", "something1" ], "roles": [ "something", "something1" ] } }`),
			errors: func(r *Rule) (errs field.ErrorList) {
				return append(errs, field.Invalid(path, r, "Rule is matching an empty set"))
			},
		},
		{
			description: "Failed to exclude kind",
			rule:        []byte(`{ "name": "set-image-pull-policy-2", "match": { "all": [ { "resources": { "kinds": [ "Pod", "Namespace" ], "name": "something", "namespaces": [ "something", "something1" ], "selector": { "matchLabels": { "memory": "high" }, "matchExpressions": [ { "key": "tier", "operator": "In", "values": [ "database" ] } ] } } } ], "subjects": [ { "name": "something", "kind": "something", "Namespace": "something", "apiGroup": "something" }, { "name": "something1", "kind": "something1", "Namespace": "something1", "apiGroup": "something1" } ], "clusterroles": [ "something", "something1" ], "roles": [ "something", "something1" ] }, "exclude": { "all": [ { "resources": { "kinds": [ "Namespace" ], "name": "something", "namespaces": [ "something", "something1" ], "selector": { "matchLabels": { "memory": "high" }, "matchExpressions": [ { "key": "tier", "operator": "In", "values": [ "database" ] } ] } } } ], "subjects": [ { "name": "something", "kind": "something", "Namespace": "something", "apiGroup": "something" }, { "name": "something1", "kind": "something1", "Namespace": "something1", "apiGroup": "something1" } ], "clusterroles": [ "something", "something1" ], "roles": [ "something", "something1" ] } }`),
		},
		{
			description: "Failed to exclude name",
			rule:        []byte(`{ "name": "set-image-pull-policy-2", "match": { "all": [ { "resources": { "kinds": [ "Pod", "Namespace" ], "name": "something", "namespaces": [ "something", "something1" ], "selector": { "matchLabels": { "memory": "high" }, "matchExpressions": [ { "key": "tier", "operator": "In", "values": [ "database" ] } ] } } } ], "subjects": [ { "name": "something", "kind": "something", "Namespace": "something", "apiGroup": "something" }, { "name": "something1", "kind": "something1", "Namespace": "something1", "apiGroup": "something1" } ], "clusterroles": [ "something", "something1" ], "roles": [ "something", "something1" ] }, "exclude": { "all": [ { "resources": { "kinds": [ "Pod", "Namespace" ], "name": "something-*", "namespaces": [ "something", "something1" ], "selector": { "matchLabels": { "memory": "high" }, "matchExpressions": [ { "key": "tier", "operator": "In", "values": [ "database" ] } ] } } } ], "subjects": [ { "name": "something", "kind": "something", "Namespace": "something", "apiGroup": "something" }, { "name": "something1", "kind": "something1", "Namespace": "something1", "apiGroup": "something1" } ], "clusterroles": [ "something", "something1" ], "roles": [ "something", "something1" ] } }`),
		},
		{
			description: "Failed to exclude namespace",
			rule:        []byte(`{"name":"set-image-pull-policy-2","match": { "all": [ { "resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"] } ] } } } ],"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]},"exclude": { "all": [ { "resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something3","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"] } ] } } } ],"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]}}`),
		},
		{
			description: "Failed to exclude labels",
			rule:        []byte(`{"name":"set-image-pull-policy-2","match": { "all": [ { "resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"] } ] } } } ],"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]},"exclude": { "all": [ { "resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"higha"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"] } ] } } } ],"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]}}`),
		},
		{
			description: "Failed to exclude expression",
			rule:        []byte(`{"name":"set-image-pull-policy-2","match": { "all": [ { "resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"] } ] } } } ],"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]},"exclude": { "all": [ { "resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["databases"] } ] } } } ],"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]}}`),
		},
		{
			description: "Failed to exclude subjects",
			rule:        []byte(`{"name":"set-image-pull-policy-2","match": { "all": [ { "resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"] } ] } } } ],"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]},"exclude": { "all": [ { "resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"] } ] } } } ],"subjects":[{"name":"something2","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]}}`),
		},
		{
			description: "Failed to exclude clusterroles",
			rule:        []byte(`{"name":"set-image-pull-policy-2","match": { "all": [ { "resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"] } ] } } } ],"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]},"exclude": { "all": [ { "resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"] } ] } } } ],"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something3","something1"],"roles":["something","something1"]}}`),
		},
		{
			description: "Failed to exclude roles",
			rule:        []byte(`{"name":"set-image-pull-policy-2","match": { "all": [ { "resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"] } ] } } } ],"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something","something1"]},"exclude": { "all": [ { "resources":{"kinds":["Pod","Namespace"],"name":"something","namespaces":["something","something1"],"selector":{"matchLabels":{"memory":"high"},"matchExpressions":[{"key":"tier","operator":"In","values":["database"] } ] } } } ],"subjects":[{"name":"something","kind":"something","Namespace":"something","apiGroup":"something"},{"name":"something1","kind":"something1","Namespace":"something1","apiGroup":"something1"}],"clusterroles":["something","something1"],"roles":["something3","something1"]}}`),
		},
		{
			description: "simple",
			rule:        []byte(`{ "name": "set-image-pull-policy-2", "match": { "any": [ { "resources": { "kinds": [ "Pod", "Namespace" ], "name": "something", "namespaces": [ "something", "something1" ] } } ] }, "exclude": { "any": [ { "resources": { "kinds": [ "Pod", "Namespace" ], "name": "something", "namespaces": [ "something", "something1" ] } } ] } }`),
			errors: func(r *Rule) (errs field.ErrorList) {
				return append(errs, field.Invalid(path, r, "Rule is matching an empty set"))
			},
		},
		{
			description: "simple - fail",
			rule:        []byte(`{ "name": "set-image-pull-policy-2", "match": { "any": [ { "resources": { "kinds": [ "Pod", "Namespace" ], "name": "somxething", "namespaces": [ "something", "something1" ] } } ] }, "exclude": { "any": [ { "resources": { "kinds": [ "Pod", "Namespace", "Job" ], "name": "some*", "namespaces": [ "something", "something1", "something2" ] } } ] } }`),
		},
		{
			description: "empty case",
			rule:        []byte(`{ "name": "check-allow-deletes", "match": { "all": [ { "resources": { "selector": { "matchLabels": { "allow-deletes": "false" } } } } ] }, "exclude": { "clusterRoles": [ "random" ] }, "validate": { "message": "Deleting {{request.object.kind}}/{{request.object.metadata.name}} is not allowed", "deny": { "conditions": { "all": [ { "key": "{{request.operation}}", "operator": "Equal", "value": "DELETE" } ] } } } }`),
		},
	}
	for _, testcase := range testcases {
		var rule Rule
		err := json.Unmarshal(testcase.rule, &rule)
		assert.NilError(t, err)
		errs := rule.ValidateMatchExcludeConflict(path)
		var expectedErrs field.ErrorList
		if testcase.errors != nil {
			expectedErrs = testcase.errors(&rule)
		}
		assert.Equal(t, len(errs), len(expectedErrs))
		for i := range errs {
			assert.Equal(t, errs[i].Error(), expectedErrs[i].Error())
		}
	}
}

func Test_Validate_ClusterPolicy_Generate_Variables(t *testing.T) {
	path := field.NewPath("dummy")
	testcases := []struct {
		name       string
		rule       []byte
		shouldFail bool
	}{
		{
			name: "clone-name",
			rule: []byte(`
			{
				"name": "clone-secret",
				"match": {
					"any": [
						{
							"resources": {
								"kinds": [
									"Namespace"
								]
							}
						}
					]
				},
				"generate": {
					"apiVersion": "v1",
					"kind": "Secret",
					"name": "regcred",
					"namespace": "test",
					"synchronize": true,
					"clone": {
						"namespace": "default",
						"name": "{{request.object.metadata.name}}"
					}
				}
			}`),
			shouldFail: true,
		},
		{
			name: "clone-namespace",
			rule: []byte(`
			{
				"name": "clone-secret",
				"match": {
					"any": [
						{
							"resources": {
								"kinds": [
									"Namespace"
								]
							}
						}
					]
				},
				"generate": {
					"apiVersion": "v1",
					"kind": "Secret",
					"name": "regcred",
					"namespace": "test",
					"synchronize": true,
					"clone": {
						"namespace": "{{request.object.metadata.name}}",
						"name": "regcred"
					}
				}
			}`),
			shouldFail: true,
		},
		{
			name: "cloneList-namespace",
			rule: []byte(`
			{
				"name": "sync-secret",
				"match": {
					"any": [
						{
							"resources": {
								"kinds": [
									"Namespace"
								]
							}
						}
					]
				},
				"generate": {
					"namespace": "test",
					"synchronize": true,
					"cloneList": {
						"namespace": "{{request.object.metadata.name}}",
						"kinds": [
							"v1/Secret",
							"v1/ConfigMap"
						],
						"selector": {
							"matchLabels": {
								"allowedToBeCloned": "true"
							}
						}
					}
				}
			}`),
			shouldFail: true,
		},
		{
			name: "cloneList-kinds",
			rule: []byte(`
			{
				"name": "sync-secret",
				"match": {
					"any": [
						{
							"resources": {
								"kinds": [
									"Namespace"
								]
							}
						}
					]
				},
				"generate": {
					"namespace": "test",
					"synchronize": true,
					"cloneList": {
						"namespace": "default",
						"kinds": [
							"{{request.object.metadata.kind}}",
							"v1/ConfigMap"
						],
						"selector": {
							"matchLabels": {
								"allowedToBeCloned": "true"
							}
						}
					}
				}
			}`),
			shouldFail: true,
		},
		{
			name: "cloneList-selector",
			rule: []byte(`
			{
				"name": "sync-secret",
				"match": {
					"any": [
						{
							"resources": {
								"kinds": [
									"Namespace"
								]
							}
						}
					]
				},
				"generate": {
					"namespace": "test",
					"synchronize": true,
					"cloneList": {
						"namespace": "default",
						"kinds": [
							"v1/Secret",
							"v1/ConfigMap"
						],
						"selector": {
							"matchLabels": {
								"{{request.object.metadata.name}}": "clone"
							}
						}
					}
				}
			}`),
			shouldFail: true,
		},
		{
			name: "generate-downstream-namespace",
			rule: []byte(`
			{
				"name": "clone-secret",
				"match": {
					"any": [
						{
							"resources": {
								"kinds": [
									"Namespace"
								]
							}
						}
					]
				},
				"generate": {
					"apiVersion": "v1",
					"kind": "Secret",
					"name": "regcred",
					"namespace": "{{request.object.metadata.name}}",
					"synchronize": true,
					"clone": {
						"namespace": "default",
						"name": "regcred"
					}
				}
			}`),
			shouldFail: false,
		},
		{
			name: "generate-downstream-kind",
			rule: []byte(`
			{
				"name": "clone-secret",
				"match": {
					"any": [
						{
							"resources": {
								"kinds": [
									"Namespace"
								]
							}
						}
					]
				},
				"generate": {
					"apiVersion": "v1",
					"kind": "{{request.object.metadata.kind}}",
					"name": "regcred",
					"namespace": "default",
					"synchronize": true,
					"clone": {
						"namespace": "default",
						"name": "regcred"
					}
				}
			}`),
			shouldFail: true,
		},
		{
			name: "generate-downstream-apiversion",
			rule: []byte(`
			{
				"name": "clone-secret",
				"match": {
					"any": [
						{
							"resources": {
								"kinds": [
									"Namespace"
								]
							}
						}
					]
				},
				"generate": {
					"kind": "Secret",
					"apiVersion": "{{request.object.metadata.apiVersion}}",
					"name": "regcred",
					"namespace": "default",
					"synchronize": true,
					"clone": {
						"namespace": "default",
						"name": "regcred"
					}
				}
			}`),
			shouldFail: true,
		},
		{
			name: "generate-downstream-name",
			rule: []byte(`
			{
				"name": "clone-secret",
				"match": {
					"any": [
						{
							"resources": {
								"kinds": [
									"Namespace"
								]
							}
						}
					]
				},
				"generate": {
					"apiVersion": "v1",
					"kind": "Secret",
					"name": "{{request.object.metadata.name}}",
					"namespace": "default",
					"synchronize": true,
					"clone": {
						"namespace": "default",
						"name": "regcred"
					}
				}
			}`),
			shouldFail: false,
		},
	}

	for _, testcase := range testcases {
		var rule *Rule
		err := json.Unmarshal(testcase.rule, &rule)
		assert.NilError(t, err, testcase.name)
		errs := rule.ValidateGenerate(path, false, "", nil)
		assert.Equal(t, len(errs) != 0, testcase.shouldFail, testcase.name)
	}
}
