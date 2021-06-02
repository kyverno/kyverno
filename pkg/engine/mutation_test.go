package engine

import (
	"encoding/json"
	"reflect"
	"testing"

	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/kyverno/store"
	"gotest.tools/assert"
)

func Test_VariableSubstitutionOverlay(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
			"name": "add-label"
		},
		"spec": {
			"rules": [
				{
					"name": "add-name-label",
					"match": {
						"resources": {
							"kinds": [
								"Pod"
							]
						}
					},
					"mutate": {
						"overlay": {
							"metadata": {
								"labels": {
									"appname": "{{request.object.metadata.name}}"
								}
							}
						}
					}
				}
			]
		}
	}
	`)
	rawResource := []byte(`
	{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "check-root-user"
		},
		"spec": {
			"containers": [
				{
					"name": "check-root-user",
					"image": "nginxinc/nginx-unprivileged",
					"securityContext": {
						"runAsNonRoot": true
					}
				}
			]
		}
	}
	`)
	expectedPatch := []byte(`{"op":"add","path":"/metadata/labels","value":{"appname":"check-root-user"}}`)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	if err != nil {
		t.Error(err)
	}
	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	ctx := context.NewContext()
	err = ctx.AddResource(rawResource)
	if err != nil {
		t.Error(err)
	}
	value, err := ctx.Query("request.object.metadata.name")

	t.Log(value)
	if err != nil {
		t.Error(err)
	}
	policyContext := &PolicyContext{
		Policy:      policy,
		JSONContext: ctx,
		NewResource: *resourceUnstructured}
	er := Mutate(policyContext)
	t.Log(string(expectedPatch))
	t.Log(string(er.PolicyResponse.Rules[0].Patches[0]))
	if !reflect.DeepEqual(expectedPatch, er.PolicyResponse.Rules[0].Patches[0]) {
		t.Error("patches dont match")
	}
}

func Test_variableSubstitutionPathNotExist(t *testing.T) {
	resourceRaw := []byte(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "check-root-user"
		},
		"spec": {
			"containers": [
				{
					"name": "check-root-user",
					"image": "nginxinc/nginx-unprivileged",
					"securityContext": {
						"runAsNonRoot": true
					}
				}
			]
		}
	}`)

	policyraw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "substitute-variable"
		},
		"spec": {
		  "rules": [
			{
			  "name": "test-path-not-exist",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "mutate": {
				"overlay": {
				  "spec": {
					"name": "{{request.object.metadata.name1}}"
				  }
				}
			  }
			}
		  ]
		}
	  }`)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(policyraw, &policy)
	assert.NilError(t, err)
	resourceUnstructured, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = ctx.AddResource(resourceRaw)
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      policy,
		JSONContext: ctx,
		NewResource: *resourceUnstructured}
	er := Mutate(policyContext)
	expectedErrorStr := "variable substitution failed for rule test-path-not-exist: NotFoundVariableErr, variable request.object.metadata.name1 not resolved at path /mutate/overlay/spec/name"
	t.Log(er.PolicyResponse.Rules[0].Message)
	assert.Equal(t, er.PolicyResponse.Rules[0].Message, expectedErrorStr)
}

func Test_variableSubstitutionCLI(t *testing.T) {
	resourceRaw := []byte(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
		  "name": "nginx-config-test"
		},
		"spec": {
		  "containers": [
			{
			  "image": "nginx:latest",
			  "name": "test-nginx"
			}
		  ]
		}
	}`)

	policyraw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "cm-variable-example"
		},
		"spec": {
		  "rules": [
			{
			  "name": "example-configmap-lookup",
			  "context": [
				{
				  "name": "dictionary",
				  "configMap": {
					"name": "mycmap",
					"namespace": "default"
				  }
				}
			  ],
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "mutate": {
				"patchStrategicMerge": {
				  "metadata": {
					"labels": {
					  "my-environment-name": "{{dictionary.data.env}}"
					}
				  }
				}
			  }
			}
		  ]
		}
	  }`)

	configMapVariableContext := store.Context{
		Policies: []store.Policy{
			{
				Name: "cm-variable-example",
				Rules: []store.Rule{
					{
						Name: "example-configmap-lookup",
						Values: map[string]string{
							"dictionary.data.env": "dev1",
						},
					},
				},
			},
		},
	}

	expectedPatch := []byte(`{"op":"add","path":"/metadata/labels","value":{"my-environment-name":"dev1"}}`)

	store.SetContext(configMapVariableContext)
	store.SetMock(true)
	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(policyraw, &policy)
	assert.NilError(t, err)
	resourceUnstructured, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = ctx.AddResource(resourceRaw)
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      policy,
		JSONContext: ctx,
		NewResource: *resourceUnstructured,
	}

	er := Mutate(policyContext)
	t.Log(string(expectedPatch))
	t.Log(string(er.PolicyResponse.Rules[0].Patches[0]))
	if !reflect.DeepEqual(expectedPatch, er.PolicyResponse.Rules[0].Patches[0]) {
		t.Error("patches dont match")
	}
}
