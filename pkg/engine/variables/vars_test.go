package variables

import (
	"encoding/json"
	"testing"

	"github.com/nirmata/kyverno/pkg/engine/context"
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

	if _, err := SubstituteVars(log.Log, ctx, pattern); err != nil {
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

	if _, err := SubstituteVars(log.Log, ctx, pattern); err == nil {
		t.Error("error is expected")
	}
}

func Test_SubvarRecursive(t *testing.T) {
	patternRaw := []byte(`"{{request.object.metadata.{{request.object.metadata.test}}}}"`)
	var pattern interface{}
	assert.Assert(t, json.Unmarshal(patternRaw, &pattern))

	resourceRaw := []byte(`
	{
		"metadata": {
			"name": "temp",
			"namespace": "n1",
			"test":"name"
		},
		"spec": {
			"namespace": "n1",
			"name": "temp1"
		}
	}
		`)

	ctx := context.NewContext()
	assert.Assert(t, ctx.AddResource(resourceRaw))
	errs := []error{}
	subValR(log.Log, ctx, string(patternRaw), "/", &errs)
}
