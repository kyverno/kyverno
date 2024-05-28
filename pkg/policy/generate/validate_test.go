package generate

import (
	"context"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	"gotest.tools/assert"
)

func Test_Validate_Generate(t *testing.T) {
	rawGenerate := []byte(`
	{
		"kind": "NetworkPolicy",
		"name": "defaultnetworkpolicy",
		"data": {
		   "spec": {
			  "podSelector": {},
			  "policyTypes": [
				 "Ingress",
				 "Egress"
			  ],
			  "ingress": [
				 {}
			  ],
			  "egress": [
				 {}
			  ]
		   }
		}
	 }`)

	var genRule kyverno.Generation
	err := jsonutils.Unmarshal(rawGenerate, &genRule)
	assert.NilError(t, err)
	checker := NewFakeGenerate(genRule)
	_, err = checker.Validate(context.TODO())
	t.Log(err)
	assert.Assert(t, err != nil)
}

func Test_Validate_Generate_HasAnchors(t *testing.T) {
	var err error
	rawGenerate := []byte(`
	{
		"kind": "NetworkPolicy",
		"name": "defaultnetworkpolicy",
		"data": {
		   "spec": {
			  "(podSelector)": {},
			  "policyTypes": [
				 "Ingress",
				 "Egress"
			  ],
			  "ingress": [
				 {}
			  ],
			  "egress": [
				 {}
			  ]
		   }
		}
	 }`)

	var genRule kyverno.Generation
	err = jsonutils.Unmarshal(rawGenerate, &genRule)
	assert.NilError(t, err)
	checker := NewFakeGenerate(genRule)
	if _, err := checker.Validate(context.TODO()); err != nil {
		assert.Assert(t, err != nil)
	}

	rawGenerate = []byte(`
	{
		"kind": "ConfigMap",
		"name": "copied-cm",
		"clone": {
		   "^(namespace)": "default",
		   "name": "game"
		}
	 }`)

	err = jsonutils.Unmarshal(rawGenerate, &genRule)
	assert.NilError(t, err)
	checker = NewFakeGenerate(genRule)
	if _, err := checker.Validate(context.TODO()); err != nil {
		assert.Assert(t, err != nil)
	}
}
