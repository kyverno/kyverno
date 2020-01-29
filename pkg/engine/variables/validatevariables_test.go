package variables

import (
	"encoding/json"
	"fmt"
	"testing"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"gotest.tools/assert"
	authenticationv1 "k8s.io/api/authentication/v1"
)

func Test_ExtractVariables(t *testing.T) {
	patternRaw := []byte(`
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

	var pattern interface{}
	if err := json.Unmarshal(patternRaw, &pattern); err != nil {
		t.Error(err)
	}

	vars := extractVariables(pattern)

	result := [][]string{[]string{"{{request.userInfo.username}}", "request.userInfo.username"},
		[]string{"{{request.object.metadata.name}}", "request.object.metadata.name"}}

	assert.Assert(t, len(vars) == len(result), fmt.Sprintf("result does not match, var: %s", vars))
}

func Test_ValidateVariables_NoVariable(t *testing.T) {
	patternRaw := []byte(`
{
	"name": "ns-owner",
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
					"Pod"
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
	var pattern, resource interface{}
	assert.NilError(t, json.Unmarshal(patternRaw, &pattern))
	assert.NilError(t, json.Unmarshal(resourceRaw, &resource))

	var err error
	ctx := context.NewContext()
	err = ctx.AddResource(resourceRaw)
	if err != nil {
		t.Error(err)
	}
	err = ctx.AddUserInfo(userReqInfo)
	if err != nil {
		t.Error(err)
	}
	invalidPaths := ValidateVariables(ctx, pattern)
	assert.Assert(t, len(invalidPaths) == 0)
}

func Test_ValidateVariables(t *testing.T) {
	patternRaw := []byte(`
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

	// userInfo
	userReqInfo := kyverno.RequestInfo{
		AdmissionUserInfo: authenticationv1.UserInfo{
			Username: "user1",
		},
	}
	var pattern, resource interface{}
	assert.NilError(t, json.Unmarshal(patternRaw, &pattern))
	assert.NilError(t, json.Unmarshal(resourceRaw, &resource))

	ctx := context.NewContext()
	var err error
	err = ctx.AddResource(resourceRaw)
	if err != nil {
		t.Error(err)
	}
	err = ctx.AddUserInfo(userReqInfo)
	if err != nil {
		t.Error(err)
	}

	invalidPaths := ValidateVariables(ctx, pattern)
	assert.Assert(t, len(invalidPaths) > 0)
}
