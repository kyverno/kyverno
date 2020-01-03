package variables

import (
	"encoding/json"
	"reflect"
	"testing"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	authenticationv1 "k8s.io/api/authentication/v1"

	"github.com/nirmata/kyverno/pkg/engine/context"
)

func Test_variablesub1(t *testing.T) {
	patternMap := []byte(`
	{
		"kind": "ClusterRole",
		"name": "ns-owner-{{request.userInfo.username}}",
		"data": {
			"rules": [
				{
					"apiGroups": [
						""
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
	// userInfo
	userReqInfo := kyverno.RequestInfo{
		AdmissionUserInfo: authenticationv1.UserInfo{
			Username: "user1",
		},
	}

	resultMap := []byte(`{"data":{"rules":[{"apiGroups":[""],"resourceNames":["temp"],"resources":["namespaces"],"verbs":["*"]}]},"kind":"ClusterRole","name":"ns-owner-user1"}`)

	var pattern, resource interface{}
	json.Unmarshal(patternMap, &pattern)
	json.Unmarshal(resourceRaw, &resource)
	// context
	ctx := context.NewContext()
	ctx.AddResource(resourceRaw)
	ctx.AddUserInfo(userReqInfo)
	value := SubstituteVariables(ctx, pattern)
	resultRaw, err := json.Marshal(value)
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(resultMap, resultRaw) {
		t.Log(string(resultMap))
		t.Log(string(resultRaw))
		t.Error("result does not match")
	}
}
func Test_variablesubstitution(t *testing.T) {
	patternMap := []byte(`
	{
		"name": "ns-owner-{{request.userInfo.username}}",
		"data": {
			"rules": [
				{
					"apiGroups": [
						""
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

	resultMap := []byte(`"data":{"rules":[{"apiGroups":[""],"resourceNames":["temp"],"resources":["namespaces"],"verbs":["*"]}]},"name":"ns-owner-user1"}`)
	// userInfo
	userReqInfo := kyverno.RequestInfo{
		AdmissionUserInfo: authenticationv1.UserInfo{
			Username: "user1",
		},
	}
	var pattern, resource interface{}
	json.Unmarshal(patternMap, &pattern)
	json.Unmarshal(resourceRaw, &resource)
	// context
	ctx := context.NewContext()
	ctx.AddResource(resourceRaw)
	ctx.AddUserInfo(userReqInfo)
	value := SubstituteVariables(ctx, pattern)
	resultRaw, err := json.Marshal(value)
	if err != nil {
		t.Error(err)
	}
	if !reflect.DeepEqual(resultMap, resultRaw) {
		t.Log(string(resultMap))
		t.Log(string(resultRaw))
		t.Error("result does not match")
	}
}

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
