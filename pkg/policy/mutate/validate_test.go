package mutate

import (
	"encoding/json"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"gotest.tools/assert"
)

func Test_Validate_Mutate_ConditionAnchor(t *testing.T) {
	rawMutate := []byte(`
	{
		"overlay": {
		  "spec": {
			"(serviceAccountName)": "*",
			"automountServiceAccountToken": false
		  }
		}
	  }`)

	var mutate kyverno.Mutation
	err := json.Unmarshal(rawMutate, &mutate)
	assert.NilError(t, err)
	checker := NewMutateFactory(mutate)
	if _, err := checker.Validate(); err != nil {
		assert.NilError(t, err)
	}
}

func Test_Validate_Mutate_PlusAnchor(t *testing.T) {
	rawMutate := []byte(`
	{
		"overlay": {
		   "spec": {
			  "+(serviceAccountName)": "*",
			  "automountServiceAccountToken": false
		   }
		}
	 }`)

	var mutate kyverno.Mutation
	err := json.Unmarshal(rawMutate, &mutate)
	assert.NilError(t, err)

	checker := NewMutateFactory(mutate)
	if _, err := checker.Validate(); err != nil {
		assert.NilError(t, err)
	}
}

func Test_Validate_Mutate_Mismatched(t *testing.T) {
	rawMutate := []byte(`
	{
		"overlay": {
		   "spec": {
			  "^(serviceAccountName)": "*",
			  "automountServiceAccountToken": false
		   }
		}
	 }`)

	var mutateExistence kyverno.Mutation
	err := json.Unmarshal(rawMutate, &mutateExistence)
	assert.NilError(t, err)

	checker := NewMutateFactory(mutateExistence)
	if _, err := checker.Validate(); err != nil {
		assert.Assert(t, err != nil)
	}

	var mutateEqual kyverno.Mutation
	rawMutate = []byte(`
	{
		"overlay": {
		   "spec": {
			  "=(serviceAccountName)": "*",
			  "automountServiceAccountToken": false
		   }
		}
	 }`)

	err = json.Unmarshal(rawMutate, &mutateEqual)
	assert.NilError(t, err)

	checker = NewMutateFactory(mutateEqual)
	if _, err := checker.Validate(); err != nil {
		assert.Assert(t, err != nil)
	}

	var mutateNegation kyverno.Mutation
	rawMutate = []byte(`
	{
		"overlay": {
		   "spec": {
			  "X(serviceAccountName)": "*",
			  "automountServiceAccountToken": false
		   }
		}
	 }`)

	err = json.Unmarshal(rawMutate, &mutateNegation)
	assert.NilError(t, err)

	checker = NewMutateFactory(mutateEqual)
	if _, err := checker.Validate(); err != nil {
		assert.Assert(t, err != nil)
	}
}

func Test_Validate_Mutate_Unsupported(t *testing.T) {
	var err error
	var mutate kyverno.Mutation
	// case 1
	rawMutate := []byte(`
	{
		"overlay": {
		   "spec": {
			  "!(serviceAccountName)": "*",
			  "automountServiceAccountToken": false
		   }
		}
	 }`)

	err = json.Unmarshal(rawMutate, &mutate)
	assert.NilError(t, err)

	checker := NewMutateFactory(mutate)
	if _, err := checker.Validate(); err != nil {
		assert.Assert(t, err != nil)
	}

	// case 2
	rawMutate = []byte(`
	{
		"overlay": {
		   "spec": {
			  "~(serviceAccountName)": "*",
			  "automountServiceAccountToken": false
		   }
		}
	 }`)

	err = json.Unmarshal(rawMutate, &mutate)
	assert.NilError(t, err)

	checker = NewMutateFactory(mutate)
	if _, err := checker.Validate(); err != nil {
		assert.Assert(t, err != nil)
	}
}
