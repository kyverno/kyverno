package variables

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/nirmata/kyverno/pkg/engine/context"
)

func Test_variableSubstitutionValue(t *testing.T) {

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
	patternMap := []byte(`
	{
		"spec": {
			"name": "{{request.object.metadata.name}}"
		}
	}
	`)

	resultMap := []byte(`{"spec":{"name":"temp"}}`)

	var pattern, resource interface{}
	json.Unmarshal(patternMap, &pattern)
	json.Unmarshal(resourceRaw, &resource)

	// context
	ctx := context.NewContext()
	ctx.AddResource(resourceRaw)
	value := SubstituteVariables(ctx, pattern)
	resultRaw, err := json.Marshal(value)
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(resultMap, resultRaw) {
		t.Error("result does not match")
	}
}

func Test_variableSubstitutionValueOperatorNotEqual(t *testing.T) {

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
	patternMap := []byte(`
	{
		"spec": {
			"name": "!{{request.object.metadata.name}}"
		}
	}
	`)
	resultMap := []byte(`{"spec":{"name":"!temp"}}`)

	var pattern, resource interface{}
	json.Unmarshal(patternMap, &pattern)
	json.Unmarshal(resourceRaw, &resource)

	// context
	ctx := context.NewContext()
	ctx.AddResource(resourceRaw)
	value := SubstituteVariables(ctx, pattern)
	resultRaw, err := json.Marshal(value)
	if err != nil {
		t.Error(err)
	}
	t.Log(string(resultRaw))
	t.Log(string(resultMap))
	if !reflect.DeepEqual(resultMap, resultRaw) {
		t.Error("result does not match")
	}
}

func Test_variableSubstitutionValueFail(t *testing.T) {

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
	patternMap := []byte(`
	{
		"spec": {
			"name": "{{request.object.metadata.name1}}"
		}
	}
	`)
	resultMap := []byte(`{"spec":{"name":null}}`)

	var pattern, resource interface{}
	json.Unmarshal(patternMap, &pattern)
	json.Unmarshal(resourceRaw, &resource)

	// context
	ctx := context.NewContext()
	ctx.AddResource(resourceRaw)
	value := SubstituteVariables(ctx, pattern)
	resultRaw, err := json.Marshal(value)
	if err != nil {
		t.Error(err)
	}
	t.Log(string(resultRaw))
	t.Log(string(resultMap))
	if !reflect.DeepEqual(resultMap, resultRaw) {
		t.Error("result does not match")
	}
}

func Test_variableSubstitutionObject(t *testing.T) {
	resourceRaw := []byte(`
	{
		"metadata": {
			"name": "temp",
			"namespace": "n1"
		},
		"spec": {
			"namespace": "n1",
			"variable": {
				"var1": "temp1",
				"var2": "temp2",
				"varNested": {
					"var1": "temp1"
				}
			}
		}
	}
	`)
	patternMap := []byte(`
	{
		"spec": {
			"variable": "{{request.object.spec.variable}}"
		}
	}
	`)
	resultMap := []byte(`{"spec":{"variable":{"var1":"temp1","var2":"temp2","varNested":{"var1":"temp1"}}}}`)

	var pattern, resource interface{}
	json.Unmarshal(patternMap, &pattern)
	json.Unmarshal(resourceRaw, &resource)

	// context
	ctx := context.NewContext()
	ctx.AddResource(resourceRaw)
	value := SubstituteVariables(ctx, pattern)
	resultRaw, err := json.Marshal(value)
	if err != nil {
		t.Error(err)
	}
	t.Log(string(resultRaw))
	t.Log(string(resultMap))
	if !reflect.DeepEqual(resultMap, resultRaw) {
		t.Error("result does not match")
	}
}

func Test_variableSubstitutionObjectOperatorNotEqualFail(t *testing.T) {
	resourceRaw := []byte(`
	{
		"metadata": {
			"name": "temp",
			"namespace": "n1"
		},
		"spec": {
			"namespace": "n1",
			"variable": {
				"var1": "temp1",
				"var2": "temp2",
				"varNested": {
					"var1": "temp1"
				}
			}
		}
	}
	`)
	patternMap := []byte(`
	{
		"spec": {
			"variable": "!{{request.object.spec.variable}}"
		}
	}
	`)

	resultMap := []byte(`{"spec":{"variable":null}}`)

	var pattern, resource interface{}
	json.Unmarshal(patternMap, &pattern)
	json.Unmarshal(resourceRaw, &resource)

	// context
	ctx := context.NewContext()
	ctx.AddResource(resourceRaw)
	value := SubstituteVariables(ctx, pattern)
	resultRaw, err := json.Marshal(value)
	if err != nil {
		t.Error(err)
	}
	t.Log(string(resultRaw))
	t.Log(string(resultMap))
	if !reflect.DeepEqual(resultMap, resultRaw) {
		t.Error("result does not match")
	}
}

func Test_variableSubstitutionMultipleObject(t *testing.T) {
	resourceRaw := []byte(`
	{
		"metadata": {
			"name": "temp",
			"namespace": "n1"
		},
		"spec": {
			"namespace": "n1",
			"variable": {
				"var1": "temp1",
				"var2": "temp2",
				"varNested": {
					"var1": "temp1"
				}
			}
		}
	}
	`)
	patternMap := []byte(`
	{
		"spec": {
			"var": "{{request.object.spec.variable.varNested.var1}}",
			"variable": "{{request.object.spec.variable}}"
		}
	}
	`)

	resultMap := []byte(`{"spec":{"var":"temp1","variable":{"var1":"temp1","var2":"temp2","varNested":{"var1":"temp1"}}}}`)

	var pattern, resource interface{}
	json.Unmarshal(patternMap, &pattern)
	json.Unmarshal(resourceRaw, &resource)

	// context
	ctx := context.NewContext()
	ctx.AddResource(resourceRaw)
	value := SubstituteVariables(ctx, pattern)
	resultRaw, err := json.Marshal(value)
	if err != nil {
		t.Error(err)
	}
	t.Log(string(resultRaw))
	t.Log(string(resultMap))
	if !reflect.DeepEqual(resultMap, resultRaw) {
		t.Error("result does not match")
	}
}
