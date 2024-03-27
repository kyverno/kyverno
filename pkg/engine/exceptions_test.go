package engine

import (
	"encoding/json"
	"testing"

	"gotest.tools/assert"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/kyverno/v2beta1"
)

var rawpolicy = []byte(`
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
					"patchStrategicMerge": {
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

func TestGetPolicyExceptions(t *testing.T) {
	var e engine
	var expectedResource []v2beta1.PolicyException

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawpolicy, &policy)
	assert.NilError(t, err)

	mutatedResource, err := e.GetPolicyExceptions(&policy, "add-name-label")
	assert.NilError(t, err)

	assert.DeepEqual(t, expectedResource, mutatedResource)
}
