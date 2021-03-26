package engine

import (
	"encoding/json"
	"testing"

	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"gotest.tools/assert"
)

var rawPolicy = []byte(`
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

var rawResource = []byte(`
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

func Test_ForceMutateSubstituteVars(t *testing.T) {
	expectedRawResource := []byte(`
	{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "check-root-user",
			"labels": {
				"appname": "check-root-user"
			}
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

	var expectedResource interface{}
	assert.NilError(t, json.Unmarshal(expectedRawResource, &expectedResource))

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	ctx := context.NewContext()
	err = ctx.AddResource(rawResource)
	assert.NilError(t, err)

	mutatedResource, err := ForceMutate(ctx, policy, *resourceUnstructured)
	assert.NilError(t, err)

	assert.DeepEqual(t, expectedResource, mutatedResource.UnstructuredContent())
}

func Test_ForceMutateSubstituteVarsWithNilContext(t *testing.T) {
	expectedRawResource := []byte(`
	{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "check-root-user",
			"labels": {
				"appname": "placeholderValue"
			}
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

	var expectedResource interface{}
	assert.NilError(t, json.Unmarshal(expectedRawResource, &expectedResource))

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)

	mutatedResource, err := ForceMutate(nil, policy, *resourceUnstructured)
	assert.NilError(t, err)

	assert.DeepEqual(t, expectedResource, mutatedResource.UnstructuredContent())
}
