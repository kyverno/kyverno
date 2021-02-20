package variables

import (
	"encoding/json"
	"testing"

	"github.com/kyverno/kyverno/pkg/engine/context"
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
	results, err := subValR(log.Log, ctx, string(patternRaw), "/")
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
	results, err := subValR(log.Log, ctx, string(patternRaw), "/")
	if err == nil {
		t.Errorf("expected error but received: %v", results)
	}

	patternRaw = []byte(`"{{request.object.metadata2.{{request.object.metadata.annotations.test}}}}"`)
	assert.Assert(t, json.Unmarshal(patternRaw, &pattern))
	results, err = subValR(log.Log, ctx, string(patternRaw), "/")
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
	results, err := subValR(log.Log, ctx, string(patternRaw), "/")
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

	_, err = SubstituteVars(log.Log, ctx, contextMap)
	assert.Assert(t, err != nil, err)
}
