package engine

import (
	"encoding/json"
	"reflect"
	"testing"

	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/kyverno/store"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_VariableSubstitutionOverlay(t *testing.T) {
	rawPolicy := []byte(`
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
	rawResource := []byte(`
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
	expectedPatch := []byte(`{"op":"add","path":"/metadata/labels","value":{"appname":"check-root-user"}}`)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	if err != nil {
		t.Error(err)
	}
	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	ctx := context.NewContext()
	err = ctx.AddResource(rawResource)
	if err != nil {
		t.Error(err)
	}
	value, err := ctx.Query("request.object.metadata.name")

	t.Log(value)
	if err != nil {
		t.Error(err)
	}
	policyContext := &PolicyContext{
		Policy:      policy,
		JSONContext: ctx,
		NewResource: *resourceUnstructured}
	er := Mutate(policyContext)
	t.Log(string(expectedPatch))
	t.Log(string(er.PolicyResponse.Rules[0].Patches[0]))
	if !reflect.DeepEqual(expectedPatch, er.PolicyResponse.Rules[0].Patches[0]) {
		t.Error("patches dont match")
	}
}

func Test_variableSubstitutionPathNotExist(t *testing.T) {
	resourceRaw := []byte(`{
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
	}`)

	policyraw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "substitute-variable"
		},
		"spec": {
		  "rules": [
			{
			  "name": "test-path-not-exist",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "mutate": {
				"overlay": {
				  "spec": {
					"name": "{{request.object.metadata.name1}}"
				  }
				}
			  }
			}
		  ]
		}
	  }`)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(policyraw, &policy)
	assert.NilError(t, err)
	resourceUnstructured, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = ctx.AddResource(resourceRaw)
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      policy,
		JSONContext: ctx,
		NewResource: *resourceUnstructured}
	er := Mutate(policyContext)
	expectedErrorStr := "variable substitution failed for rule test-path-not-exist: Unknown key \"name1\" in path"
	assert.Equal(t, er.PolicyResponse.Rules[0].Message, expectedErrorStr)
}

func Test_variableSubstitutionCLI(t *testing.T) {
	resourceRaw := []byte(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
		  "name": "nginx-config-test"
		},
		"spec": {
		  "containers": [
			{
			  "image": "nginx:latest",
			  "name": "test-nginx"
			}
		  ]
		}
	}`)

	policyraw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "cm-variable-example"
		},
		"spec": {
		  "rules": [
			{
			  "name": "example-configmap-lookup",
			  "context": [
				{
				  "name": "dictionary",
				  "configMap": {
					"name": "mycmap",
					"namespace": "default"
				  }
				}
			  ],
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
					  "my-environment-name": "{{dictionary.data.env}}"
					}
				  }
				}
			  }
			}
		  ]
		}
	  }`)

	configMapVariableContext := store.Context{
		Policies: []store.Policy{
			{
				Name: "cm-variable-example",
				Rules: []store.Rule{
					{
						Name: "example-configmap-lookup",
						Values: map[string]string{
							"dictionary.data.env": "dev1",
						},
					},
				},
			},
		},
	}

	expectedPatch := []byte(`{"op":"add","path":"/metadata/labels","value":{"my-environment-name":"dev1"}}`)

	store.SetContext(configMapVariableContext)
	store.SetMock(true)
	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(policyraw, &policy)
	assert.NilError(t, err)
	resourceUnstructured, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = ctx.AddResource(resourceRaw)
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      policy,
		JSONContext: ctx,
		NewResource: *resourceUnstructured,
	}

	er := Mutate(policyContext)
	t.Log(string(expectedPatch))
	t.Log(string(er.PolicyResponse.Rules[0].Patches[0]))
	if !reflect.DeepEqual(expectedPatch, er.PolicyResponse.Rules[0].Patches[0]) {
		t.Error("patches don't match")
	}
}

// https://github.com/kyverno/kyverno/issues/2022
func Test_chained_rules(t *testing.T) {
	policyRaw := []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"replace-image-registry","annotations":{"policies.kyverno.io/minversion":"1.4.2"}},"spec":{"background":false,"rules":[{"name":"replace-image-registry","match":{"resources":{"kinds":["Pod"]}},"mutate":{"patchStrategicMerge":{"spec":{"containers":[{"(name)":"*","image":"{{regex_replace_all('^[^/]+','{{@}}','myregistry.corp.com')}}"}]}}}},{"name":"replace-image-registry-chained","match":{"resources":{"kinds":["Pod"]}},"mutate":{"patchStrategicMerge":{"spec":{"containers":[{"(name)":"*","image":"{{regex_replace_all('\\b(myregistry.corp.com)\\b','{{@}}','otherregistry.corp.com')}}"}]}}}}]}}`)
	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(policyRaw, &policy)
	assert.NilError(t, err)

	resourceRaw := []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"test"},"spec":{"containers":[{"name":"test","image":"foo/bash:5.0"}]}}`)
	resource, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = ctx.AddResourceAsObject(resource.Object)
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      policy,
		JSONContext: ctx,
		NewResource: *resource,
	}

	err = ctx.AddImageInfo(resource)
	assert.NilError(t, err)

	err = context.MutateResourceWithImageInfo(resourceRaw, ctx)
	assert.NilError(t, err)

	er := Mutate(policyContext)
	containers, _, err := unstructured.NestedSlice(er.PatchedResource.Object, "spec", "containers")
	assert.NilError(t, err)
	assert.Equal(t, containers[0].(map[string]interface{})["image"], "otherregistry.corp.com/foo/bash:5.0")

	assert.Equal(t, string(er.PolicyResponse.Rules[0].Patches[0]), `{"op":"replace","path":"/spec/containers/0/image","value":"myregistry.corp.com/foo/bash:5.0"}`)
	assert.Equal(t, string(er.PolicyResponse.Rules[1].Patches[0]), `{"op":"replace","path":"/spec/containers/0/image","value":"otherregistry.corp.com/foo/bash:5.0"}`)
}

func Test_precondition(t *testing.T) {
	resourceRaw := []byte(`{
  "apiVersion": "v1",
  "kind": "Pod",
  "metadata": {
    "name": "nginx-config-test",
    "labels": {
      "app.kubernetes.io/managed-by": "Helm"
    }
  },
  "spec": {
    "containers": [
      {
        "image": "nginx:latest",
        "name": "test-nginx"
      }
    ]
  }
}`)

	policyraw := []byte(`{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "cm-variable-example"
  },
  "spec": {
    "rules": [
      {
        "name": "example-configmap-lookup",
        "match": {
          "resources": {
            "kinds": [
              "Pod"
            ]
          }
        },
        "preconditions": [
          {
            "key": "{{ request.object.metadata.labels.\"app.kubernetes.io/managed-by\"}}",
            "operator": "Equals",
            "value": "Helm"
          }
        ],
        "mutate": {
          "patchStrategicMerge": {
            "metadata": {
              "labels": {
                "my-added-label": "test"
              }
            }
          }
        }
      }
    ]
  }
}`)

	expectedPatch := []byte(`{"op":"add","path":"/metadata/labels","value":{"my-added-label":"test"}}`)

	store.SetMock(true)
	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(policyraw, &policy)
	assert.NilError(t, err)
	resourceUnstructured, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = ctx.AddResource(resourceRaw)
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      policy,
		JSONContext: ctx,
		NewResource: *resourceUnstructured,
	}

	er := Mutate(policyContext)
	t.Log(string(expectedPatch))
	t.Log(string(er.PolicyResponse.Rules[0].Patches[0]))
	if !reflect.DeepEqual(expectedPatch, er.PolicyResponse.Rules[0].Patches[0]) {
		t.Error("patches don't match")
	}
}
