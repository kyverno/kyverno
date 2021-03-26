package engine

import (
	"encoding/json"
	"regexp"
	"strings"

	"reflect"
	"testing"

	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	assertnew "github.com/stretchr/testify/assert"
	"gotest.tools/assert"
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
	expectedErrorStr := "variable request.object.metadata.name1 not resolved at path /spec/name"
	t.Log(er.PolicyResponse.Rules[0].Message)
	assert.Equal(t, er.PolicyResponse.Rules[0].Message, expectedErrorStr)
}

func Test_patchJson6902WithJMESPath(t *testing.T) {

	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "mutate-ingress-host"
		},
		"spec": {
		  "rules": [
			{
			  "name": "mutate-ingress-host",
			  "match": {
				"resources": {
				  "kinds": [
					"Ingress"
				  ]
				}
			  },
			  "exclude": {
				"resources": {
				  "namespaces": [
					"kube-system",
					"kube-public",
					"kyverno"
				  ]
				}
			  },
			  "mutate": {
				"patchesJson6902": "- op: replace\n  path: \"/spec/rules/0/host\"\n  value: \"{{request.object.spec.rules[0].host}}.mycompany.com\"\n- op: replace\n  path: \"/spec/tls/0/hosts/0\"\n  value: \"{{request.object.spec.tls[0].hosts[0]}}.mycompany.com\""
			  }
			}
		  ]
		}
	  }
	`)

	rawResource := []byte(`
	{
		"apiVersion": "networking.k8s.io/v1",
		"kind": "Ingress",
		"metadata": {
		  "name": "kuard",
		  "labels": {
			"app": "kuard"
		  }
		},
		"spec": {
		  "rules": [
			{
			  "host": "kuard",
			  "http": {
				"paths": [
				  {
					"backend": {
					  "service": {
						"name": "kuard",
						"port": {
						  "number": 8080
						}
					  }
					},
					"path": "/",
					"pathType": "ImplementationSpecific"
				  }
				]
			  }
			}
		  ],
		  "tls": [
			{
			  "hosts": [
				"kuard"
			  ]
			}
		  ]
		}
	  }
	`)

	expected := []byte(`
		{
			"apiVersion": "networking.k8s.io/v1",
			"kind": "Ingress",
			"metadata": {
			  "labels": {
				"app": "kuard"
			  },
			  "name": "kuard"
			},
			"spec": {
			  "rules": [
				{
				  "host": "kuard.mycompany.com",
				  "http": {
					"paths": [
					  {
						"backend": {
						  "service": {
							"name": "kuard",
							"port": {
							  "number": 8080
							}
						  }
						},
						"path": "/",
						"pathType": "ImplementationSpecific"
					  }
					]
				  }
				}
			  ],
			  "tls": [
				{
				  "hosts": [
					"kuard.mycompany.com"
				  ]
				}
			  ]
			}
		  }
		`)
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

	policyContext := &PolicyContext{
		Policy:      policy,
		JSONContext: ctx,
		NewResource: *resourceUnstructured}
	resp := Mutate(policyContext)
	result, _ := resp.PatchedResource.DeepCopy().MarshalJSON()

	re := regexp.MustCompile(`[[:space:]]`)
	exp := strings.TrimSpace(re.ReplaceAllString(string(expected), ""))
	res := strings.TrimSpace(string(result))

	if !assertnew.Equal(t, string(res), exp) {
		t.FailNow()
	}
}
