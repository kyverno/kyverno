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
	errs := subject.Validate(path, false, nil)
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
		errs := rule.Validate(path, false, nil)
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
		errs := rule.Validate(path, false, nil)
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
