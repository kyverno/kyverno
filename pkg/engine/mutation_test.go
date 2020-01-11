package engine

import (
	"encoding/json"
	"reflect"
	"strings"
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
	expectedPatch := []byte(`{ "op": "add", "path": "/metadata/labels", "value":{"appname":"check-root-user"} }`)

	var policy kyverno.ClusterPolicy
	json.Unmarshal(rawPolicy, &policy)
	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	ctx := context.NewContext()
	ctx.AddResource(rawResource)
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
		  "name": "substitue-variable"
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
	json.Unmarshal(policyraw, &policy)
	resourceUnstructured, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContext()
	ctx.AddResource(resourceRaw)

	policyContext := PolicyContext{
		Policy:      policy,
		Context:     ctx,
		NewResource: *resourceUnstructured}
	er := Mutate(policyContext)
	assert.Assert(t, er.PolicyResponse.Rules[0].PathNotPresent, true)
}

func Test_variableSubstitutionPathNotExist_InRuleInfo(t *testing.T) {
	resourceRaw := []byte(`{
		"apiVersion": "v1",
		"kind": "Deployment",
		"metadata": {
		  "name": "check-root-user"
		}
	  }`)

	policyraw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "test-validate-variables"
		},
		"spec": {
		  "rules": [
			{
			  "name": "test-match",
			  "match": {
				"resources": {
				  "kinds": [
					"{{request.kind}}"
				  ]
				}
			  }
			}
		  ]
		}
	  }`)

	var policy kyverno.ClusterPolicy
	assert.NilError(t, json.Unmarshal(policyraw, &policy))
	resourceUnstructured, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContext()
	ctx.AddResource(resourceRaw)

	policyContext := PolicyContext{
		Policy:      policy,
		Context:     ctx,
		NewResource: *resourceUnstructured}
	er := Mutate(policyContext)
	assert.Assert(t, strings.Contains(er.PolicyResponse.Rules[0].Message, "path not present in rule info"))
}
