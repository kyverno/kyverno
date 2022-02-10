package generate

import (
	"encoding/json"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
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
	err := json.Unmarshal(rawGenerate, &genRule)
	assert.NilError(t, err)
	checker := NewFakeGenerate(genRule)
	if _, err := checker.Validate(); err != nil {
		assert.Assert(t, err != nil)
	}
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
	err = json.Unmarshal(rawGenerate, &genRule)
	assert.NilError(t, err)
	checker := NewFakeGenerate(genRule)
	if _, err := checker.Validate(); err != nil {
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

	err = json.Unmarshal(rawGenerate, &genRule)
	assert.NilError(t, err)
	checker = NewFakeGenerate(genRule)
	if _, err := checker.Validate(); err != nil {
		assert.Assert(t, err != nil)
	}
}
