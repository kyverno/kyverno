package variables

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	ju "github.com/kyverno/kyverno/pkg/engine/jsonutils"
	"gotest.tools/v3/assert"
)

func Test_subVars_success(t *testing.T) {
	patternMap := []byte(`
	{
		"kind": "{{request.object.metadata.name}}",
		"name": "ns-owner-{{request.object.metadata.name}}",
		"data": {
			"rules": [
				{
					"apiGroups": [
						"{{request.object.metadata.name}}"
					],
					"resources": [
						"namespaces"
					],
					"verbs": [
						"*"
					],
					"resourceNames": [
						"{{request.object.metadata.name}}"
					]
				}
			]
		}
	}
	`)

	resourceRaw := []byte(`
	{
		"metadata": {
			"name": "temp",
			"namespace": "n1"
		},
		"spec": {
			"namespace": "n1",
			"name": "temp1"
		}
	}
		`)

	var pattern, resource interface{}
	var err error
	err = json.Unmarshal(patternMap, &pattern)
	if err != nil {
		t.Error(err)
	}
	err = json.Unmarshal(resourceRaw, &resource)
	if err != nil {
		t.Error(err)
	}
	// context
	ctx := context.NewContext(jp)
	err = context.AddResource(ctx, resourceRaw)
	if err != nil {
		t.Error(err)
	}

	if _, err := SubstituteAll(logr.Discard(), ctx, pattern); err != nil {
		t.Error(err)
	}
}

func Test_subVars_failed(t *testing.T) {
	patternMap := []byte(`
	{
		"kind": "{{request.object.metadata.name1}}",
		"name": "ns-owner-{{request.object.metadata.name}}",
		"data": {
			"rules": [
				{
					"apiGroups": [
						"{{request.object.metadata.name}}"
					],
					"resources": [
						"namespaces"
					],
					"verbs": [
						"*"
					],
					"resourceNames": [
						"{{request.object.metadata.name1}}"
					]
				}
			]
		}
	}
	`)

	resourceRaw := []byte(`
	{
		"metadata": {
			"name": "temp",
			"namespace": "n1"
		},
		"spec": {
			"namespace": "n1",
			"name": "temp1"
		}
	}
		`)

	var pattern, resource interface{}
	var err error
	err = json.Unmarshal(patternMap, &pattern)
	if err != nil {
		t.Error(err)
	}
	err = json.Unmarshal(resourceRaw, &resource)
	if err != nil {
		t.Error(err)
	}
	// context
	ctx := context.NewContext(jp)
	err = context.AddResource(ctx, resourceRaw)
	if err != nil {
		t.Error(err)
	}

	if _, err := SubstituteAll(logr.Discard(), ctx, pattern); err == nil {
		t.Error("error is expected")
	}
}

func Test_subVars_with_JMESPath_At(t *testing.T) {
	patternMap := []byte(`{
		"mutate": {
			"overlay": {
				"spec": {
					"kind": "{{@}}",
					"data": {
						"rules": [
							{
								"apiGroups": [
									"{{request.object.metadata.name}}"
								],
								"resources": [
									"namespaces"
								],
								"verbs": [
									"*"
								],
								"resourceNames": [
									"{{request.object.metadata.name}}"
								]
							}
						]
					}
				}
			}
		}
	}`)

	resourceRaw := []byte(`
	{
		"metadata": {
			"name": "temp",
			"namespace": "n1"
		},
		"spec": {
			"kind": "foo",
			"namespace": "n1",
			"name": "temp1"
		}
	}
		`)

	expectedRaw := []byte(`{
		"mutate":{
		   "overlay":{
			  "spec":{
				 "data":{
					"rules":[
					   {
						  "apiGroups":[
							 "temp"
						  ],
						  "resourceNames":[
							 "temp"
						  ],
						  "resources":[
							 "namespaces"
						  ],
						  "verbs":[
							 "*"
						  ]
					   }
					]
				 },
				 "kind":"foo"
			  }
		   }
		}
	}`)

	var err error
	var pattern, resource interface{}
	err = json.Unmarshal(patternMap, &pattern)
	assert.NilError(t, err)
	err = json.Unmarshal(resourceRaw, &resource)
	assert.NilError(t, err)
	// context
	ctx := context.NewContext(jp)
	err = context.AddResource(ctx, resourceRaw)
	assert.NilError(t, err)

	output, err := SubstituteAll(logr.Discard(), ctx, pattern)
	assert.NilError(t, err)
	out, err := json.Marshal(output)
	assert.NilError(t, err)
	assert.Equal(t, string(out), compact(t, expectedRaw))
}

func Test_subVars_withRegexMatch(t *testing.T) {
	patternMap := []byte(`{
		"mutate": {
			"overlay": {
				"spec": {
					"port": "{{ regex_match('(443)', '{{@}}') }}",
					"name": "ns-owner-{{request.object.metadata.name}}"
				}
			}
		}
	}`)

	resourceRaw := []byte(`
	{
		"metadata": {
			"name": "temp",
			"namespace": "n1"
		},
		"spec": {
			"port": "443",
			"namespace": "n1",
			"name": "temp1"
		}
	}`)

	expectedRaw := []byte(`{
		"mutate":{
		   "overlay":{
			  "spec":{
				 "name":"ns-owner-temp",
				 "port":true
			  }
		   }
		}
	 }`)

	var err error

	var pattern, resource interface{}
	err = json.Unmarshal(patternMap, &pattern)
	assert.NilError(t, err)
	err = json.Unmarshal(resourceRaw, &resource)
	assert.NilError(t, err)
	// context
	ctx := context.NewContext(jp)
	err = context.AddResource(ctx, resourceRaw)
	assert.NilError(t, err)

	output, err := SubstituteAll(logr.Discard(), ctx, pattern)
	assert.NilError(t, err)
	out, err := json.Marshal(output)
	assert.NilError(t, err)
	assert.Equal(t, string(out), compact(t, expectedRaw))
}

func Test_subVars_withMerge(t *testing.T) {
	patternMap := []byte(`{"map": "{{ merge(` + "`{\\\"a\\\": 1}`, `{\\\"b\\\": 1}`" + `)}}"}`)

	resourceRaw := []byte(`{}`)

	expectedRaw := []byte(`{"map": {"a":1,"b":1}}`)

	var err error

	var pattern, resource interface{}
	err = json.Unmarshal(patternMap, &pattern)
	assert.NilError(t, err)
	err = json.Unmarshal(resourceRaw, &resource)
	assert.NilError(t, err)
	// context
	ctx := context.NewContext(jp)
	err = context.AddResource(ctx, resourceRaw)
	assert.NilError(t, err)

	output, err := SubstituteAll(logr.Discard(), ctx, pattern)
	assert.NilError(t, err)
	out, err := json.Marshal(output)
	assert.NilError(t, err)
	assert.Equal(t, string(out), compact(t, expectedRaw))
}

func compact(t *testing.T, in []byte) string {
	var tmp map[string]interface{}
	err := json.Unmarshal(in, &tmp)
	assert.NilError(t, err)

	out, err := json.Marshal(tmp)
	assert.NilError(t, err)
	return string(out)
}

func Test_subVars_withRegexReplaceAll(t *testing.T) {
	patternMap := []byte(`{
		"mutate": {
			"overlay": {
				"spec": {
					"port": "{{ regex_replace_all_literal('.*', '{{@}}', '1313') }}",
					"name": "ns-owner-{{request.object.metadata.name}}"
				}
			}
		}
	}`)

	resourceRaw := []byte(`{
		"metadata": {
			"name": "temp",
			"namespace": "n1"
		},
		"spec": {
			"port": "43123",
			"namespace": "n1",
			"name": "temp1"
		}
	}`)
	expected := []byte(`{"mutate":{"overlay":{"spec":{"name":"ns-owner-temp","port":"1313"}}}}`)

	var pattern, resource interface{}
	var err error
	err = json.Unmarshal(patternMap, &pattern)
	assert.NilError(t, err)
	err = json.Unmarshal(resourceRaw, &resource)
	assert.NilError(t, err)
	// context
	ctx := context.NewContext(jp)
	err = context.AddResource(ctx, resourceRaw)
	assert.NilError(t, err)

	output, err := SubstituteAll(logr.Discard(), ctx, pattern)
	assert.NilError(t, err)
	out, err := json.Marshal(output)
	assert.NilError(t, err)
	assert.Equal(t, string(out), string(expected))
}

func Test_ReplacingPathWhenDeleting(t *testing.T) {
	patternRaw := []byte(`"{{request.object.metadata.annotations.target}}"`)

	resourceRaw := []byte(`
	{
		"request": {
			"operation": "DELETE",
			"object": {
				"metadata": {
					"name": "curr",
					"namespace": "ns",
					"annotations": {
					  "target": "foo"
					}
				}
			},
			"oldObject": {
				"metadata": {
					"name": "old",
					"annotations": {
					  "target": "bar"
					}
				}
			}
		}
	}
`)

	var pattern interface{}
	var err error
	err = json.Unmarshal(patternRaw, &pattern)
	assert.NilError(t, err)

	ctxMap, err := unmarshalToMap(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContextFromRaw(jp, ctxMap)
	assert.NilError(t, err)

	pattern, err = SubstituteAll(logr.Discard(), ctx, pattern)
	assert.NilError(t, err)

	assert.Equal(t, fmt.Sprintf("%v", pattern), "bar")
}

func unmarshalToMap(jsonBytes []byte) (map[string]interface{}, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		return nil, err
	}

	return data, nil
}

func Test_ReplacingNestedVariableWhenDeleting(t *testing.T) {
	patternRaw := []byte(`"{{request.object.metadata.annotations.{{request.object.metadata.annotations.targetnew}}}}"`)

	resourceRaw := []byte(`
	{
		"request":{
		   "operation":"DELETE",
		   "oldObject":{
			  "metadata":{
				 "name":"current",
				 "namespace":"ns",
				 "annotations":{
					"target":"nested_target",
					"targetnew":"target"
				 }
			  }
		   }
		}
	}`)

	var pattern interface{}
	var err error
	err = json.Unmarshal(patternRaw, &pattern)
	assert.NilError(t, err)

	ctxMap, err := unmarshalToMap(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContextFromRaw(jp, ctxMap)
	pattern, err = SubstituteAll(logr.Discard(), ctx, pattern)
	assert.NilError(t, err)

	assert.Equal(t, fmt.Sprintf("%v", pattern), "nested_target")
}

var resourceRaw = []byte(`
	{
		"metadata": {
			"name": "temp",
			"namespace": "n1",
			"annotations": {
			  "test": "name"
            }
		},
		"spec": {
			"namespace": "n1",
			"name": "temp1"
		}
	}
`)

func Test_SubstituteSuccess(t *testing.T) {
	ctx := context.NewContext(jp)
	assert.Assert(t, context.AddResource(ctx, resourceRaw))

	var pattern interface{}
	patternRaw := []byte(`"{{request.object.metadata.annotations.test}}"`)
	assert.Assert(t, json.Unmarshal(patternRaw, &pattern))

	action := substituteVariablesIfAny(logr.Discard(), ctx, DefaultVariableResolver)
	results, err := action(&ju.ActionData{
		Document: nil,
		Element:  string(patternRaw),
		Path:     "/",
	})
	if err != nil {
		t.Errorf("substitution failed: %v", err.Error())
		return
	}

	if results.(string) != `"name"` {
		t.Errorf("expected %s received %v", "name", results)
	}
}

func Test_SubstituteRecursiveErrors(t *testing.T) {
	ctx := context.NewContext(jp)
	assert.Assert(t, context.AddResource(ctx, resourceRaw))

	var pattern interface{}
	patternRaw := []byte(`"{{request.object.metadata.{{request.object.metadata.annotations.test2}}}}"`)
	assert.Assert(t, json.Unmarshal(patternRaw, &pattern))

	action := substituteVariablesIfAny(logr.Discard(), ctx, DefaultVariableResolver)
	results, err := action(&ju.ActionData{
		Document: nil,
		Element:  string(patternRaw),
		Path:     "/",
	})

	if err == nil {
		t.Errorf("expected error but received: %v", results)
	}

	patternRaw = []byte(`"{{request.object.metadata2.{{request.object.metadata.annotations.test}}}}"`)
	assert.Assert(t, json.Unmarshal(patternRaw, &pattern))

	action = substituteVariablesIfAny(logr.Discard(), ctx, DefaultVariableResolver)
	results, err = action(&ju.ActionData{
		Document: nil,
		Element:  string(patternRaw),
		Path:     "/",
	})

	if err == nil {
		t.Errorf("expected error but received: %v", results)
	}
}

func Test_SubstituteRecursive(t *testing.T) {
	ctx := context.NewContext(jp)
	assert.Assert(t, context.AddResource(ctx, resourceRaw))

	var pattern interface{}
	patternRaw := []byte(`"{{request.object.metadata.{{request.object.metadata.annotations.test}}}}"`)
	assert.Assert(t, json.Unmarshal(patternRaw, &pattern))

	action := substituteVariablesIfAny(logr.Discard(), ctx, DefaultVariableResolver)
	results, err := action(&ju.ActionData{
		Document: nil,
		Element:  string(patternRaw),
		Path:     "/",
	})
	if err != nil {
		t.Errorf("substitution failed: %v", err.Error())
		return
	}

	if results.(string) != `"temp"` {
		t.Errorf("expected %s received %v", "temp", results)
	}
}

func Test_SubstituteShallow(t *testing.T) {
	ctx := context.NewContext(jp)
	data := map[string]interface{}{
		"variableWithVariables": "{{ DO_NOT_SUBSTITUTE_ME {{OR_ME}} }}",
		"foo":                   "bar",
		"foo2":                  "bar2",
		"variablesNested":       "{{foo2}}",
	}

	assert.NilError(t, context.AddJSONObject(ctx, data))

	patternRaw := []byte(`"{{- variableWithVariables }} {{foo}} {{variablesNested}}"`)
	action := substituteVariablesIfAny(logr.Discard(), ctx, DefaultVariableResolver)
	results, err := action(&ju.ActionData{
		Document: nil,
		Element:  string(patternRaw),
		Path:     "/",
	})

	assert.NilError(t, err)
	assert.Equal(t, results.(string), "\"{{ DO_NOT_SUBSTITUTE_ME {{OR_ME}} }} bar bar2\"")

	patternRaw = []byte(`"{{foo}} {{- variableWithVariables }} {{variablesNested}}"`)
	action = substituteVariablesIfAny(logr.Discard(), ctx, DefaultVariableResolver)
	results, err = action(&ju.ActionData{
		Document: nil,
		Element:  string(patternRaw),
		Path:     "/",
	})

	assert.NilError(t, err)
	assert.Equal(t, results.(string), "\"bar {{ DO_NOT_SUBSTITUTE_ME {{OR_ME}} }} bar2\"")

	patternRaw = []byte(`"{{- variableWithVariables {{foo}} {{variablesNested}} }}"`)
	action = substituteVariablesIfAny(logr.Discard(), ctx, DefaultVariableResolver)
	_, err = action(&ju.ActionData{
		Document: nil,
		Element:  string(patternRaw),
		Path:     "/",
	})

	assert.ErrorContains(t, err, "failed to resolve variableWithVariables bar bar2")
}

// TODO: this test fails, not sure how we could merge this !
// func Test_subVars_withShallowReplaceAll(t *testing.T) {
// 	patternMap := []byte(`{
// 		"mutate": {
// 			"overlay": {
// 				"data": {
// 					"config": "{{ replace_all('{{- request.object.data.config }}', 'from_string', 'to_string') }}"
// 				}
// 			}
// 		}
// 	}`)

// 	resourceRaw := []byte(`{
// 		"metadata": {
// 			"name": "temp",
// 			"namespace": "n1"
// 		},
// 		"data": {
// 			"config": "some_prefix from_string some_suffix {{ unresolvable_token }}"
// 		}
// 	}`)
// 	expected := []byte(`{"mutate":{"overlay":{"data":{"config":"some_prefix to_string some_suffix {{ unresolvable_token }}"}}}}`)

// 	var pattern, resource interface{}
// 	var err error
// 	err = json.Unmarshal(patternMap, &pattern)
// 	assert.NilError(t, err)
// 	err = json.Unmarshal(resourceRaw, &resource)
// 	assert.NilError(t, err)
// 	// context
// 	ctx := context.NewContext(jp)
// 	err = context.AddResource(ctx, resourceRaw)
// 	assert.NilError(t, err)

// 	output, err := SubstituteAll(logr.Discard(), ctx, pattern)
// 	assert.NilError(t, err)
// 	out, err := json.Marshal(output)
// 	assert.NilError(t, err)
// 	assert.Equal(t, string(out), string(expected))
// }

func Test_policyContextValidation(t *testing.T) {
	policyContext := []byte(`
	{
		"context": [
			{
				"name": "myconfigmap",
				"apiCall": {
					"urlPath": "/api/v1/namespaces/{{ request.namespace }}/configmaps/generate-pod"
				}
			}
		]
	}
	`)

	var contextMap interface{}
	err := json.Unmarshal(policyContext, &contextMap)
	assert.NilError(t, err)

	ctx := context.NewMockContext(nil, "request.object")

	_, err = SubstituteAll(logr.Discard(), ctx, contextMap)
	assert.Assert(t, err != nil, err)
}

func Test_variableSubstitution_array(t *testing.T) {
	configmapRaw := []byte(`
{
    "animals": {
        "apiVersion": "v1",
        "kind": "ConfigMap",
        "metadata": {
            "name": "animals",
            "namespace": "default"
        },
        "data": {
            "animals": "snake\nbear\ncat\ndog"
        }
    }
}`)

	ruleRaw := []byte(`
{
    "name": "validate-role-annotation",
    "context": [
        {
            "name": "animals",
            "configMap": {
                "name": "animals",
                "namespace": "default"
            }
        }
    ],
    "match": {
        "resources": {
            "kinds": [
                "Deployment"
            ]
        }
    },
    "validate": {
        "message": "The animal {{ request.object.metadata.labels.animal }} is not in the allowed list of animals: {{ animals.data.animals }}.",
        "deny": {
            "conditions": [
                {
                    "key": "{{ request.object.metadata.labels.animal }}",
                    "operator": "NotIn",
                    "value": "{{ animals.data.animals }}"
                }
            ]
        }
    }
}`)

	resourceRaw := []byte(`
{
    "apiVersion": "apps/v1",
    "kind": "Deployment",
    "metadata": {
        "name": "busybox",
        "labels": {
            "app": "busybox",
            "color": "red",
            "animal": "cow",
            "food": "pizza",
            "car": "jeep",
            "env": "qa"
        }
    }
}
`)

	var rule v1.Rule
	err := json.Unmarshal(ruleRaw, &rule)
	assert.NilError(t, err)

	ctxMap, err := unmarshalToMap(configmapRaw)
	assert.NilError(t, err)

	ctx := context.NewContextFromRaw(jp, ctxMap)
	context.AddResource(ctx, resourceRaw)

	vars, err := SubstituteAllInRule(logr.Discard(), ctx, rule)
	assert.NilError(t, err)

	assert.DeepEqual(t, vars.Validation.Message, "The animal cow is not in the allowed list of animals: snake\nbear\ncat\ndog.")
}

var variableObject = []byte(`
{
	"complex_object_array": [
		"value1",
		"value2",
		"value3"
	],
	"complex_object_map": {
		"key1": "value1",
		"key2": "value2",
		"key3": "value3"
	},
	"simple_object_bool": false,
	"simple_object_int": 5,
	"simple_object_float": -5.5,
	"simple_object_string": "example",
	"simple_object_null": null
}
`)

func Test_SubstituteNull(t *testing.T) {
	patternRaw := []byte(`
	{
		"spec": {
			"content": "{{ request.object.simple_object_null }}"
		}
	}
	`)

	var err error
	var pattern, resource map[string]interface{}
	err = json.Unmarshal(patternRaw, &pattern)
	assert.NilError(t, err)

	err = json.Unmarshal(variableObject, &resource)
	assert.NilError(t, err)

	ctx := context.NewContext(jp)
	context.AddResource(ctx, variableObject)

	resolved, err := SubstituteAll(logr.Discard(), ctx, pattern)
	assert.NilError(t, err)

	content := resolved.(map[string]interface{})["spec"].(map[string]interface{})["content"]
	var expected interface{}

	assert.DeepEqual(t, expected, content)
}

func Test_SubstituteNullInString(t *testing.T) {
	patternRaw := []byte(`
	{
		"spec": {
			"content": "content = {{ request.object.simple_object_null }}"
		}
	}
	`)

	var err error
	var pattern, resource map[string]interface{}
	err = json.Unmarshal(patternRaw, &pattern)
	assert.NilError(t, err)

	err = json.Unmarshal(variableObject, &resource)
	assert.NilError(t, err)

	ctx := context.NewContext(jp)
	context.AddResource(ctx, variableObject)

	resolved, err := SubstituteAll(logr.Discard(), ctx, pattern)
	assert.NilError(t, err)

	content := resolved.(map[string]interface{})["spec"].(map[string]interface{})["content"]
	expected := "content = null"

	assert.DeepEqual(t, expected, content)
}

func Test_SubstituteArray(t *testing.T) {
	patternRaw := []byte(`
	{
		"spec": {
			"content": "{{ request.object.complex_object_array }}"
		}
	}
	`)

	var err error
	var pattern, resource map[string]interface{}
	err = json.Unmarshal(patternRaw, &pattern)
	assert.NilError(t, err)

	err = json.Unmarshal(variableObject, &resource)
	assert.NilError(t, err)

	ctx := context.NewContext(jp)
	context.AddResource(ctx, variableObject)

	resolved, err := SubstituteAll(logr.Discard(), ctx, pattern)
	assert.NilError(t, err)

	content := resolved.(map[string]interface{})["spec"].(map[string]interface{})["content"]
	expected := resource["complex_object_array"]

	assert.DeepEqual(t, expected, content)
}

func Test_SubstituteArrayInString(t *testing.T) {
	patternRaw := []byte(`
	{
		"spec": {
			"content": "content is {{ request.object.complex_object_map }}"
		}
	}
	`)

	var err error
	var pattern, resource map[string]interface{}
	err = json.Unmarshal(patternRaw, &pattern)
	assert.NilError(t, err)

	err = json.Unmarshal(variableObject, &resource)
	assert.NilError(t, err)

	ctx := context.NewContext(jp)
	context.AddResource(ctx, variableObject)

	resolved, err := SubstituteAll(logr.Discard(), ctx, pattern)
	assert.NilError(t, err)

	content := resolved.(map[string]interface{})["spec"].(map[string]interface{})["content"]
	expected := `content is {"key1":"value1","key2":"value2","key3":"value3"}`

	assert.DeepEqual(t, expected, content)
}

func Test_SubstituteInt(t *testing.T) {
	patternRaw := []byte(`
	{
		"spec": {
			"content": "{{ request.object.simple_object_int }}"
		}
	}
	`)

	var err error
	var pattern, resource map[string]interface{}
	err = json.Unmarshal(patternRaw, &pattern)
	assert.NilError(t, err)

	err = json.Unmarshal(variableObject, &resource)
	assert.NilError(t, err)

	ctx := context.NewContext(jp)
	context.AddResource(ctx, variableObject)

	resolved, err := SubstituteAll(logr.Discard(), ctx, pattern)
	assert.NilError(t, err)

	content := resolved.(map[string]interface{})["spec"].(map[string]interface{})["content"]
	expected := resource["simple_object_int"]

	assert.DeepEqual(t, expected, content)
}

func Test_SubstituteIntInString(t *testing.T) {
	patternRaw := []byte(`
	{
		"spec": {
			"content": "content = {{ request.object.simple_object_int }}"
		}
	}
	`)

	var err error
	var pattern, resource map[string]interface{}
	err = json.Unmarshal(patternRaw, &pattern)
	assert.NilError(t, err)

	err = json.Unmarshal(variableObject, &resource)
	assert.NilError(t, err)

	ctx := context.NewContext(jp)
	context.AddResource(ctx, variableObject)

	resolved, err := SubstituteAll(logr.Discard(), ctx, pattern)
	assert.NilError(t, err)

	content := resolved.(map[string]interface{})["spec"].(map[string]interface{})["content"]
	expected := "content = 5"

	assert.DeepEqual(t, expected, content)
}

func Test_SubstituteBool(t *testing.T) {
	patternRaw := []byte(`
	{
		"spec": {
			"content": "{{ request.object.simple_object_bool }}"
		}
	}
	`)

	var err error
	var pattern, resource map[string]interface{}
	err = json.Unmarshal(patternRaw, &pattern)
	assert.NilError(t, err)

	err = json.Unmarshal(variableObject, &resource)
	assert.NilError(t, err)

	ctx := context.NewContext(jp)
	context.AddResource(ctx, variableObject)

	resolved, err := SubstituteAll(logr.Discard(), ctx, pattern)
	assert.NilError(t, err)

	content := resolved.(map[string]interface{})["spec"].(map[string]interface{})["content"]
	expected := false

	assert.DeepEqual(t, expected, content)
}

func Test_SubstituteBoolInString(t *testing.T) {
	patternRaw := []byte(`
	{
		"spec": {
			"content": "content = {{ request.object.simple_object_bool }}"
		}
	}
	`)

	var err error
	var pattern, resource map[string]interface{}
	err = json.Unmarshal(patternRaw, &pattern)
	assert.NilError(t, err)

	err = json.Unmarshal(variableObject, &resource)
	assert.NilError(t, err)

	ctx := context.NewContext(jp)
	context.AddResource(ctx, variableObject)

	resolved, err := SubstituteAll(logr.Discard(), ctx, pattern)
	assert.NilError(t, err)

	content := resolved.(map[string]interface{})["spec"].(map[string]interface{})["content"]
	expected := "content = false"

	assert.DeepEqual(t, expected, content)
}

func Test_SubstituteString(t *testing.T) {
	patternRaw := []byte(`
	{
		"spec": {
			"content": "{{ request.object.simple_object_string }}"
		}
	}
	`)

	var err error
	var pattern, resource map[string]interface{}
	err = json.Unmarshal(patternRaw, &pattern)
	assert.NilError(t, err)

	err = json.Unmarshal(variableObject, &resource)
	assert.NilError(t, err)

	ctx := context.NewContext(jp)
	context.AddResource(ctx, variableObject)

	resolved, err := SubstituteAll(logr.Discard(), ctx, pattern)
	assert.NilError(t, err)

	content := resolved.(map[string]interface{})["spec"].(map[string]interface{})["content"]
	expected := "example"

	assert.DeepEqual(t, expected, content)
}

func Test_SubstituteStringInString(t *testing.T) {
	patternRaw := []byte(`
	{
		"spec": {
			"content": "content = {{ request.object.simple_object_string }}"
		}
	}
	`)

	var err error
	var pattern, resource map[string]interface{}
	err = json.Unmarshal(patternRaw, &pattern)
	assert.NilError(t, err)

	err = json.Unmarshal(variableObject, &resource)
	assert.NilError(t, err)

	ctx := context.NewContext(jp)
	context.AddResource(ctx, variableObject)

	resolved, err := SubstituteAll(logr.Discard(), ctx, pattern)
	assert.NilError(t, err)

	content := resolved.(map[string]interface{})["spec"].(map[string]interface{})["content"]
	expected := "content = example"

	assert.DeepEqual(t, expected, content)
}

func Test_ReferenceSubstitution(t *testing.T) {
	jsonRaw := []byte(`
	{
		"metadata": {
			"name": "temp",
			"namespace": "n1",
			"annotations": {
			  "test": "$(../../../../spec/namespace)"
            }
		},
		"(spec)": {
			"namespace": "n1",
			"name": "temp1"
		}
	}`)

	expectedJSON := []byte(`
	{
		"metadata": {
			"name": "temp",
			"namespace": "n1",
			"annotations": {
			  "test": "n1"
            }
		},
		"(spec)": {
			"namespace": "n1",
			"name": "temp1"
		}
	}`)

	var document interface{}
	err := json.Unmarshal(jsonRaw, &document)
	assert.NilError(t, err)

	var expectedDocument interface{}
	err = json.Unmarshal(expectedJSON, &expectedDocument)
	assert.NilError(t, err)

	ctx := context.NewContext(jp)
	err = context.AddResource(ctx, jsonRaw)
	assert.NilError(t, err)

	actualDocument, err := SubstituteAll(logr.Discard(), ctx, document)
	assert.NilError(t, err)

	assert.DeepEqual(t, expectedDocument, actualDocument)
}

func TestFormAbsolutePath_RelativePathExists(t *testing.T) {
	absolutePath := "/spec/containers/0/resources/requests/memory"
	referencePath := "./../../limits/memory"
	expectedString := "/spec/containers/0/resources/limits/memory"

	result := formAbsolutePath(referencePath, absolutePath)

	assert.Assert(t, result == expectedString)
}

func TestFormAbsolutePath_RelativePathWithBackToTopInTheBeginning(t *testing.T) {
	absolutePath := "/spec/containers/0/resources/requests/memory"
	referencePath := "../../limits/memory"
	expectedString := "/spec/containers/0/resources/limits/memory"

	result := formAbsolutePath(referencePath, absolutePath)

	assert.Assert(t, result == expectedString)
}

func TestFormAbsolutePath_AbsolutePathExists(t *testing.T) {
	absolutePath := "/spec/containers/0/resources/requests/memory"
	referencePath := "/spec/containers/0/resources/limits/memory"

	result := formAbsolutePath(referencePath, absolutePath)

	assert.Assert(t, result == referencePath)
}

func TestFormAbsolutePath_EmptyPath(t *testing.T) {
	absolutePath := "/spec/containers/0/resources/requests/memory"
	referencePath := ""

	result := formAbsolutePath(referencePath, absolutePath)

	assert.Assert(t, result == absolutePath)
}

func TestActualizePattern_GivenRelativePathThatExists(t *testing.T) {
	absolutePath := "/spec/containers/0/resources/requests/memory"
	referencePath := "$(<=./../../limits/memory)"

	rawPattern := []byte(`{
		"spec":{
			"containers":[
				{
					"name":"*",
					"resources":{
						"requests":{
							"memory":"$(<=./../../limits/memory)"
						},
						"limits":{
							"memory":"2048Mi"
						}
					}
				}
			]
		}
	}`)

	resolvedReference := "<=2048Mi"

	var pattern interface{}
	assert.NilError(t, json.Unmarshal(rawPattern, &pattern))

	// pattern, err := actualizePattern(log.Log, pattern, referencePath, absolutePath)

	pattern, err := resolveReference(pattern, referencePath, absolutePath)

	assert.NilError(t, err)
	assert.DeepEqual(t, resolvedReference, pattern)
}

func TestFindAndShiftReferences_PositiveCase(t *testing.T) {
	message := "Message with $(./../../pattern/spec/containers/0/image) reference inside. Or maybe even two $(./../../pattern/spec/containers/0/image), but they are same."
	expectedMessage := strings.Replace(message, "$(./../../pattern/spec/containers/0/image)", "$(./../../pattern/spec/jobTemplate/spec/containers/0/image)", -1)
	actualMessage := FindAndShiftReferences(logr.Discard(), message, "spec/jobTemplate", "pattern")

	assert.Equal(t, expectedMessage, actualMessage)
}

func TestFindAndShiftReferences_AnyPatternPositiveCase(t *testing.T) {
	message := "Message with $(./../../anyPattern/0/spec/containers/0/image)."
	expectedMessage := strings.Replace(message, "$(./../../anyPattern/0/spec/containers/0/image)", "$(./../../anyPattern/0/spec/jobTemplate/spec/containers/0/image)", -1)
	actualMessage := FindAndShiftReferences(logr.Discard(), message, "spec/jobTemplate", "anyPattern")

	assert.Equal(t, expectedMessage, actualMessage)
}

func Test_EscpReferenceSubstitution(t *testing.T) {
	jsonRaw := []byte(`
	{
		"metadata": {
			"name": "temp",
			"namespace": "n1",
			"annotations": {
			  "test1": "$(../../../../spec/namespace)",
			  "test2": "\\$(ENV_VAR)",
			  "test3": "\\${ENV_VAR}",
			  "test4": "\\\\\\${ENV_VAR}",
			  "test5": "\\$(NODE_NAME)/postgres/\\$(POD_NAME)"
            }
		},
		"(spec)": {
			"namespace": "n1",
			"name": "temp1"
		}
	}`)

	expectedJSON := []byte(`
	{
		"metadata": {
			"name": "temp",
			"namespace": "n1",
			"annotations": {
			  "test1": "n1",
			  "test2": "$(ENV_VAR)",
			  "test3": "\\${ENV_VAR}",
			  "test4": "\\\\\\${ENV_VAR}",
			  "test5": "$(NODE_NAME)/postgres/$(POD_NAME)"
            }
		},
		"(spec)": {
			"namespace": "n1",
			"name": "temp1"
		}
	}`)

	var document interface{}
	err := json.Unmarshal(jsonRaw, &document)
	assert.NilError(t, err)

	var expectedDocument interface{}
	err = json.Unmarshal(expectedJSON, &expectedDocument)
	assert.NilError(t, err)

	ctx := context.NewContext(jp)
	err = context.AddResource(ctx, jsonRaw)
	assert.NilError(t, err)

	actualDocument, err := SubstituteAll(logr.Discard(), ctx, document)
	assert.NilError(t, err)

	assert.DeepEqual(t, expectedDocument, actualDocument)
}

func Test_ReplacingEscpNestedVariableWhenDeleting(t *testing.T) {
	patternRaw := []byte(`"\\{{request.object.metadata.annotations.{{request.object.metadata.annotations.targetnew}}}}"`)

	resourceRaw := []byte(`
	{
		"request":{
		   "operation":"DELETE",
		   "oldObject":{
			  "metadata":{
				 "name":"current",
				 "namespace":"ns",
				 "annotations":{
					"target":"nested_target",
					"targetnew":"target"
				 }
			  }
		   }
		}
	}`)

	var pattern interface{}
	var err error
	err = json.Unmarshal(patternRaw, &pattern)
	if err != nil {
		t.Error(err)
	}

	ctxMap, err := unmarshalToMap(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContextFromRaw(jp, ctxMap)
	assert.NilError(t, err)

	pattern, err = SubstituteAll(logr.Discard(), ctx, pattern)
	assert.NilError(t, err)

	assert.Equal(t, fmt.Sprintf("%v", pattern), "{{request.object.metadata.annotations.target}}")
}

func Test_ReplaceAllVars(t *testing.T) {
	result := ReplaceAllVars("{{ foo }}", func(s string) string { return "test" })
	assert.Equal(t, result, "test")

	result = ReplaceAllVars("\"{{ foo }}\"", func(s string) string { return "test" })
	assert.Equal(t, result, "\"test\"")

	result = ReplaceAllVars("/s/{{elementIndex}}/r", func(s string) string { return "test" })
	assert.Equal(t, result, "/s/test/r")

	result = ReplaceAllVars("{{ foo }} {{foo}} {{foo}}", func(s string) string { return "test" })
	assert.Equal(t, result, "test test test")

	result = ReplaceAllVars("{{ foo }} \\{{foo}} {{foo}}", func(s string) string { return "test" })
	assert.Equal(t, result, "test \\{{foo}} test")

	result = ReplaceAllVars("{{ foo {{foo}} }}", func(s string) string { return "test" })
	assert.Equal(t, result, "{{ foo test }}")
}

func Test_SubstituteDoesNotRescanSubstitutedValue(t *testing.T) {
	ctx := context.NewContext(jp)
	data := map[string]interface{}{
		"badValue": "prefix {{ badValue }} suffix",
	}
	assert.NilError(t, context.AddJSONObject(ctx, data))

	patternRaw := []byte(`"prefix {{ badValue }} suffix"`)
	action := substituteVariablesIfAny(logr.Discard(), ctx, DefaultVariableResolver)
	results, err := action(&ju.ActionData{
		Document: nil,
		Element:  string(patternRaw),
		Path:     "/",
	})

	assert.NilError(t, err)
	assert.Equal(t, results.(string), `"prefix prefix {{ badValue }} suffix suffix"`)
}

func Test_SubstituteTerminatesWhenValueReintroducesTemplate(t *testing.T) {
	ctx := context.NewContext(jp)
	data := map[string]interface{}{
		"msg": "X{{ msg }}",
	}
	assert.NilError(t, context.AddJSONObject(ctx, data))

	action := substituteVariablesIfAny(logr.Discard(), ctx, DefaultVariableResolver)
	results, err := action(&ju.ActionData{
		Document: nil,
		Element:  "A{{ msg }}",
		Path:     "/",
	})

	assert.NilError(t, err)
	assert.Equal(t, results.(string), "AX{{ msg }}")
}

func Test_SubstitutedValueTailIsNotRescanned(t *testing.T) {
	ctx := context.NewContext(jp)
	data := map[string]interface{}{
		"obj": map[string]interface{}{
			"k": "{{ x }}",
		},
		"x": "RESOLVED",
	}
	assert.NilError(t, context.AddJSONObject(ctx, data))

	action := substituteVariablesIfAny(logr.Discard(), ctx, DefaultVariableResolver)
	results, err := action(&ju.ActionData{
		Document: nil,
		Element:  "{{ obj }}Z",
		Path:     "/",
	})

	assert.NilError(t, err)
	assert.Equal(t, results.(string), `{"k":"{{ x }}"}Z`)
}

func Test_AdjacentVariableAfterLongSubstitutionStillResolves(t *testing.T) {
	ctx := context.NewContext(jp)
	longValue := strings.Repeat("x", 200)
	data := map[string]interface{}{
		"a": longValue,
		"b": "resolved_b",
	}
	assert.NilError(t, context.AddJSONObject(ctx, data))

	action := substituteVariablesIfAny(logr.Discard(), ctx, DefaultVariableResolver)
	results, err := action(&ju.ActionData{
		Document: nil,
		Element:  "{{ a }}-{{ b }}",
		Path:     "/",
	})

	assert.NilError(t, err)
	assert.Equal(t, results.(string), longValue+"-resolved_b")
}

func Test_DuplicateTokensBothResolve(t *testing.T) {
	ctx := context.NewContext(jp)
	data := map[string]interface{}{
		"x": "dup",
	}
	assert.NilError(t, context.AddJSONObject(ctx, data))

	action := substituteVariablesIfAny(logr.Discard(), ctx, DefaultVariableResolver)
	results, err := action(&ju.ActionData{
		Document: nil,
		Element:  "{{ x }} and {{ x }}",
		Path:     "/",
	})

	assert.NilError(t, err)
	assert.Equal(t, results.(string), "dup and dup")
}

func Test_NestedSubstitutionCountEqualsBraceCountDoesNotError(t *testing.T) {
	ctx := context.NewContext(jp)
	data := map[string]interface{}{
		"inner": "b",
		"meta": map[string]interface{}{
			"b": "resolved_meta_b",
		},
	}
	assert.NilError(t, context.AddJSONObject(ctx, data))

	patternRaw := []byte(`"{{ meta.{{ inner }} }}"`)
	action := substituteVariablesIfAny(logr.Discard(), ctx, DefaultVariableResolver)
	results, err := action(&ju.ActionData{
		Document: nil,
		Element:  string(patternRaw),
		Path:     "/",
	})

	assert.NilError(t, err)
	assert.Equal(t, results.(string), `"resolved_meta_b"`)
}

func Test_AliasChainResolvesThroughHops(t *testing.T) {
	ctx := context.NewContext(jp)
	data := map[string]interface{}{
		"x": "{{y}}",
		"y": "{{z}}",
		"z": "final",
	}
	assert.NilError(t, context.AddJSONObject(ctx, data))

	action := substituteVariablesIfAny(logr.Discard(), ctx, DefaultVariableResolver)
	results, err := action(&ju.ActionData{
		Document: nil,
		Element:  "A{{ x }}B",
		Path:     "/",
	})

	assert.NilError(t, err)
	assert.Equal(t, results.(string), "AfinalB")
}

func Test_SelfReferentialAliasReturnsError(t *testing.T) {
	ctx := context.NewContext(jp)
	data := map[string]interface{}{
		"x": "{{x}}",
	}
	assert.NilError(t, context.AddJSONObject(ctx, data))

	action := substituteVariablesIfAny(logr.Discard(), ctx, DefaultVariableResolver)
	_, err := action(&ju.ActionData{
		Document: nil,
		Element:  "A{{ x }}B",
		Path:     "/",
	})

	assert.ErrorContains(t, err, "cyclic")
}

func Test_AliasCycleReturnsError(t *testing.T) {
	ctx := context.NewContext(jp)
	data := map[string]interface{}{
		"x": "{{y}}",
		"y": "{{x}}",
	}
	assert.NilError(t, context.AddJSONObject(ctx, data))

	action := substituteVariablesIfAny(logr.Discard(), ctx, DefaultVariableResolver)
	_, err := action(&ju.ActionData{
		Document: nil,
		Element:  "A{{ x }}B",
		Path:     "/",
	})

	assert.ErrorContains(t, err, "cyclic")
}

func Test_EmbeddedTokenInResolvedValueStaysLiteral(t *testing.T) {
	ctx := context.NewContext(jp)
	data := map[string]interface{}{
		"outer": "A{{ msg }}B",
		"msg":   "oops",
	}
	assert.NilError(t, context.AddJSONObject(ctx, data))

	action := substituteVariablesIfAny(logr.Discard(), ctx, DefaultVariableResolver)
	results, err := action(&ju.ActionData{
		Document: nil,
		Element:  "X{{ outer }}Y",
		Path:     "/",
	})

	assert.NilError(t, err)
	assert.Equal(t, results.(string), "XA{{ msg }}BY")
}

func Test_MultiTokenResolvedValueStaysLiteral(t *testing.T) {
	ctx := context.NewContext(jp)
	big := strings.Repeat("{{tok}} ", 50)
	data := map[string]interface{}{
		"big": big,
	}
	assert.NilError(t, context.AddJSONObject(ctx, data))

	action := substituteVariablesIfAny(logr.Discard(), ctx, DefaultVariableResolver)
	results, err := action(&ju.ActionData{
		Document: nil,
		Element:  "prefix {{ big }} suffix",
		Path:     "/",
	})

	assert.NilError(t, err)
	assert.Equal(t, results.(string), "prefix "+big+" suffix")
}

func Test_WholeLeafAliasReturnsRawValue(t *testing.T) {
	ctx := context.NewContext(jp)
	data := map[string]interface{}{
		"variablesNested": "{{foo2}}",
		"foo2":            "bar2",
	}
	assert.NilError(t, context.AddJSONObject(ctx, data))

	action := substituteVariablesIfAny(logr.Discard(), ctx, DefaultVariableResolver)
	results, err := action(&ju.ActionData{
		Document: nil,
		Element:  "{{variablesNested}}",
		Path:     "/",
	})

	assert.NilError(t, err)
	assert.Equal(t, results.(string), "{{foo2}}")
}

func Test_SubstituteShallowNonStringValueEscapesEmbeddedBraces(t *testing.T) {
	ctx := context.NewContext(jp)
	data := map[string]interface{}{
		"obj": map[string]interface{}{
			"k": "{{ x }}",
		},
		"x": "should-not-resolve",
	}
	assert.NilError(t, context.AddJSONObject(ctx, data))

	action := substituteVariablesIfAny(logr.Discard(), ctx, DefaultVariableResolver)
	results, err := action(&ju.ActionData{
		Document: nil,
		Element:  "prefix {{- obj }} suffix",
		Path:     "/",
	})

	assert.NilError(t, err)
	assert.Equal(t, results.(string), `prefix {"k":"{{ x }}"} suffix`)
}

// Test_AcyclicAliasChainHitsHardSubstitutionCap pins the hard backstop
// (independent of budget and the visited cycle-detection set): a long,
// strictly acyclic alias chain -- each hop introduces a brand new name, so
// the visited set never repeats and budget keeps replenishing by one per
// alias hop -- must still stop once it crosses the absolute cap, rather
// than running for as many substitutions as there are distinct addressable
// names in context.
func Test_AcyclicAliasChainHitsHardSubstitutionCap(t *testing.T) {
	ctx := context.NewContext(jp)

	chainLength := maxTotalSubstitutions + 1
	data := make(map[string]interface{}, chainLength+1)
	for i := 0; i < chainLength; i++ {
		data[fmt.Sprintf("v%d", i)] = fmt.Sprintf("{{v%d}}", i+1)
	}
	data[fmt.Sprintf("v%d", chainLength)] = "terminal"

	assert.NilError(t, context.AddJSONObject(ctx, data))

	action := substituteVariablesIfAny(logr.Discard(), ctx, DefaultVariableResolver)
	_, err := action(&ju.ActionData{
		Document: nil,
		Element:  "A{{ v0 }}B",
		Path:     "/",
	})

	assert.ErrorContains(t, err, "exceeded the hard limit on total variable substitutions")
}

// Test_SubstitutionOutputSizeIsBoundedPerLeaf pins the output-size cap: a
// single substitution whose resolved value, spliced into surrounding text,
// exceeds maxSubstitutedOutputBytes is rejected. The count cap alone does not
// catch this -- it is one substitution.
func Test_SubstitutionOutputSizeIsBoundedPerLeaf(t *testing.T) {
	ctx := context.NewContext(jp)
	big := strings.Repeat("a", maxSubstitutedOutputBytes+1)
	assert.NilError(t, context.AddJSONObject(ctx, map[string]interface{}{"big": big}))

	action := substituteVariablesIfAny(logr.Discard(), ctx, DefaultVariableResolver)
	_, err := action(&ju.ActionData{
		Document: nil,
		Element:  "A{{ big }}B",
		Path:     "/",
	})

	assert.ErrorContains(t, err, "exceeded the maximum substituted output size")
}

// Test_SubstitutionOutputSizeIsBoundedPerDocument pins that the output cap is
// per-document, not per-leaf: many leaves each under the cap cannot
// collectively expand the document past it.
func Test_SubstitutionOutputSizeIsBoundedPerDocument(t *testing.T) {
	ctx := context.NewContext(jp)
	chunk := strings.Repeat("a", 1<<20) // 1 MiB, well under the per-leaf cap
	assert.NilError(t, context.AddJSONObject(ctx, map[string]interface{}{"c": chunk}))

	// 12 leaves, each embedding a 1 MiB substitution: ~12 MiB total output,
	// over the 10 MiB per-document cap though no single leaf exceeds it.
	doc := make(map[string]interface{}, 12)
	for i := 0; i < 12; i++ {
		doc[fmt.Sprintf("f%d", i)] = "x{{ c }}y"
	}

	_, err := SubstituteAll(logr.Discard(), ctx, doc)
	assert.ErrorContains(t, err, "exceeded the maximum substituted output size")
}

// Test_SubstitutionOutputSizeBoundsWholeLeafForm pins that the cap also covers
// a leaf that is exactly "{{ var }}" (the most common authoring shape), which
// returns the resolved value directly. Many such leaves each resolving a large
// value expand the marshalled document without bound, so this path must be
// accounted too, not only the embedded "x{{ var }}y" form.
func Test_SubstitutionOutputSizeBoundsWholeLeafForm(t *testing.T) {
	ctx := context.NewContext(jp)
	chunk := strings.Repeat("a", 1<<20) // 1 MiB, well under the per-leaf cap
	assert.NilError(t, context.AddJSONObject(ctx, map[string]interface{}{"c": chunk}))

	// 12 whole-leaf "{{ c }}" fields: ~12 MiB total, over the 10 MiB cap.
	doc := make(map[string]interface{}, 12)
	for i := 0; i < 12; i++ {
		doc[fmt.Sprintf("f%d", i)] = "{{ c }}"
	}

	_, err := SubstituteAll(logr.Discard(), ctx, doc)
	assert.ErrorContains(t, err, "exceeded the maximum substituted output size")
}

// Test_SubstitutionManyTokensPerLeafIsCorrect pins that a leaf carrying many
// tokens resolves every one correctly, both adjacent and separated by text.
// The substitution loop resolves a whole pass of tokens at once rather than one
// per full-string rescan; this guards that batching keeps per-token semantics.
func Test_SubstitutionManyTokensPerLeafIsCorrect(t *testing.T) {
	ctx := context.NewContext(jp)
	assert.NilError(t, context.AddJSONObject(ctx, map[string]interface{}{"a": "A", "b": "B"}))

	const n = 1000
	adjacent := strings.Repeat("{{ a }}", n)
	out, err := SubstituteAll(logr.Discard(), ctx, adjacent)
	assert.NilError(t, err)
	assert.Equal(t, out, strings.Repeat("A", n))

	separated := strings.Repeat("{{ a }}-{{ b }}|", n)
	out, err = SubstituteAll(logr.Discard(), ctx, separated)
	assert.NilError(t, err)
	assert.Equal(t, out, strings.Repeat("A-B|", n))
}

// Test_SubstitutionCostIsNotQuadratic guards the substitution loop against the
// O(tokens x length) shape it once had, where a full-string rescan ran per
// token. A leaf of many tokens whose combined output stays under the size cap
// must still complete in roughly linear time. The bound is deliberately loose
// so it is not timing-flaky: the quadratic form took tens of seconds at 3 MiB
// where the linear form takes well under a second.
func Test_SubstitutionCostIsNotQuadratic(t *testing.T) {
	if testing.Short() {
		t.Skip("timing-sensitive; skipped under -short")
	}
	ctx := context.NewContext(jp)
	chunk := strings.Repeat("a", 5000)
	assert.NilError(t, context.AddJSONObject(ctx, map[string]interface{}{"c": chunk}))

	// ~3 MiB of output across ~629 tokens, well under the 10 MiB cap.
	leaf := strings.Repeat("{{ c }}", 629)

	start := time.Now()
	out, err := SubstituteAll(logr.Discard(), ctx, leaf)
	elapsed := time.Since(start)

	assert.NilError(t, err)
	assert.Equal(t, len(out.(string)), 629*5000)
	// Measured on this workload: linear 0.22s, the quadratic predecessor 29.4s.
	// A flat budget cannot serve both builds -- 10s is below the instrumented
	// linear cost on a CI runner (10.17s), and anything high enough to clear that
	// stops catching a 29.4s quadratic. So scale with the instrumentation instead:
	// CI runs `go test -race -covermode atomic`, roughly 20x uninstrumented.
	budget := 10 * time.Second
	if raceEnabled {
		budget = 120 * time.Second
	}
	if elapsed > budget {
		t.Fatalf("substitution of %d tokens took %s (budget %s), expected roughly linear (quadratic regression?)", 629, elapsed, budget)
	}
}
