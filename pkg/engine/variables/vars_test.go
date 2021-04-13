package variables

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	ju "github.com/kyverno/kyverno/pkg/engine/json-utils"
	"gotest.tools/assert"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
	ctx := context.NewContext()
	err = ctx.AddResource(resourceRaw)
	if err != nil {
		t.Error(err)
	}

	if _, err := SubstituteAll(log.Log, ctx, pattern); err != nil {
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
	ctx := context.NewContext()
	err = ctx.AddResource(resourceRaw)
	if err != nil {
		t.Error(err)
	}

	if _, err := SubstituteAll(log.Log, ctx, pattern); err == nil {
		t.Error("error is expected")
	}
}

func Test_ReplacingPathWhenDeleting(t *testing.T) {
	patternRaw := []byte(`"{{request.object.metadata.annotations.target}}"`)

	var resourceRaw = []byte(`
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
	if err != nil {
		t.Error(err)
	}
	ctx := context.NewContext()
	err = ctx.AddJSON(resourceRaw)
	assert.NilError(t, err)

	pattern, err = SubstituteAll(log.Log, ctx, pattern)
	assert.NilError(t, err)

	assert.Equal(t, fmt.Sprintf("%v", pattern), "bar")
}

func Test_ReplacingNestedVariableWhenDeleting(t *testing.T) {
	patternRaw := []byte(`"{{request.object.metadata.annotations.{{request.object.metadata.annotations.targetnew}}}}"`)

	var resourceRaw = []byte(`
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
	ctx := context.NewContext()
	err = ctx.AddJSON(resourceRaw)
	assert.NilError(t, err)

	pattern, err = SubstituteAll(log.Log, ctx, pattern)
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
	ctx := context.NewContext()
	assert.Assert(t, ctx.AddResource(resourceRaw))

	var pattern interface{}
	patternRaw := []byte(`"{{request.object.metadata.annotations.test}}"`)
	assert.Assert(t, json.Unmarshal(patternRaw, &pattern))

	action := substituteVariablesIfAny(log.Log, ctx)
	results, err := action(&ju.ActionData{
		Document: nil,
		Element:  string(patternRaw),
		Path:     "/"})

	if err != nil {
		t.Errorf("substitution failed: %v", err.Error())
		return
	}

	if results.(string) != `"name"` {
		t.Errorf("expected %s received %v", "name", results)
	}
}

func Test_SubstituteRecursiveErrors(t *testing.T) {
	ctx := context.NewContext()
	assert.Assert(t, ctx.AddResource(resourceRaw))

	var pattern interface{}
	patternRaw := []byte(`"{{request.object.metadata.{{request.object.metadata.annotations.test2}}}}"`)
	assert.Assert(t, json.Unmarshal(patternRaw, &pattern))

	action := substituteVariablesIfAny(log.Log, ctx)
	results, err := action(&ju.ActionData{
		Document: nil,
		Element:  string(patternRaw),
		Path:     "/"})

	if err == nil {
		t.Errorf("expected error but received: %v", results)
	}

	patternRaw = []byte(`"{{request.object.metadata2.{{request.object.metadata.annotations.test}}}}"`)
	assert.Assert(t, json.Unmarshal(patternRaw, &pattern))

	action = substituteVariablesIfAny(log.Log, ctx)
	results, err = action(&ju.ActionData{
		Document: nil,
		Element:  string(patternRaw),
		Path:     "/"})

	if err == nil {
		t.Errorf("expected error but received: %v", results)
	}
}

func Test_SubstituteRecursive(t *testing.T) {
	ctx := context.NewContext()
	assert.Assert(t, ctx.AddResource(resourceRaw))

	var pattern interface{}
	patternRaw := []byte(`"{{request.object.metadata.{{request.object.metadata.annotations.test}}}}"`)
	assert.Assert(t, json.Unmarshal(patternRaw, &pattern))

	action := substituteVariablesIfAny(log.Log, ctx)
	results, err := action(&ju.ActionData{
		Document: nil,
		Element:  string(patternRaw),
		Path:     "/"})

	if err != nil {
		t.Errorf("substitution failed: %v", err.Error())
		return
	}

	if results.(string) != `"temp"` {
		t.Errorf("expected %s received %v", "temp", results)
	}
}

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

	ctx := context.NewContext("request.object")

	_, err = SubstituteAll(log.Log, ctx, contextMap)
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

	ctx := context.NewContext("request.object", "animals")
	ctx.AddJSON(configmapRaw)
	ctx.AddResource(resourceRaw)

	vars, err := SubstituteAllInRule(log.Log, ctx, rule)
	assert.NilError(t, err)

	assert.DeepEqual(t, vars.Validation.Message, "The animal cow is not in the allowed list of animals: snake\nbear\ncat\ndog.")
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

	ctx := context.NewContext()
	err = ctx.AddResource(jsonRaw)
	assert.NilError(t, err)

	actualDocument, err := SubstituteAll(log.Log, ctx, document)
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

func TestFormAbsolutePath_RelativePathWithBackToTopInTheBegining(t *testing.T) {
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

	pattern, err := resolveReference(log.Log, pattern, referencePath, absolutePath)

	assert.NilError(t, err)
	assert.DeepEqual(t, resolvedReference, pattern)
}

func TestFindAndShiftReferences_PositiveCase(t *testing.T) {
	message := "Message with $(./../../pattern/spec/containers/0/image) reference inside. Or maybe even two $(./../../pattern/spec/containers/0/image), but they are same."
	expectedMessage := strings.Replace(message, "$(./../../pattern/spec/containers/0/image)", "$(./../../pattern/spec/jobTemplate/spec/containers/0/image)", -1)
	actualMessage := FindAndShiftReferences(log.Log, message, "spec/jobTemplate", "pattern")

	assert.Equal(t, expectedMessage, actualMessage)
}

func TestFindAndShiftReferences_AnyPatternPositiveCase(t *testing.T) {
	message := "Message with $(./../../anyPattern/0/spec/containers/0/image)."
	expectedMessage := strings.Replace(message, "$(./../../anyPattern/0/spec/containers/0/image)", "$(./../../anyPattern/0/spec/jobTemplate/spec/containers/0/image)", -1)
	actualMessage := FindAndShiftReferences(log.Log, message, "spec/jobTemplate", "anyPattern")

	assert.Equal(t, expectedMessage, actualMessage)
}
