package engine

import (
	"encoding/json"
	"reflect"
	"testing"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/utils"
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
	policyContext := PolicyContext{
		Policy:      policy,
		Context:     ctx,
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

	policyContext := PolicyContext{
		Policy:      policy,
		Context:     ctx,
		NewResource: *resourceUnstructured}
	er := Mutate(policyContext)
	expectedErrorStr := "could not find variable request.object.metadata.name1 at path /spec/name"
	t.Log(er.PolicyResponse.Rules[0].Message)
	assert.Equal(t, er.PolicyResponse.Rules[0].Message, expectedErrorStr)
}
