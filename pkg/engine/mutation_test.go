package engine

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
	client "github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Test_VariableSubstitutionPatchStrategicMerge(t *testing.T) {
	policyRaw := []byte(`{
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
            "patchStrategicMerge": {
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
  }`)

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
	expectedPatch := []byte(`{"op":"add","path":"/metadata/labels","value":{"appname":"check-root-user"}}`)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(policyRaw, &policy)
	if err != nil {
		t.Error(err)
	}
	resourceUnstructured, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)
	ctx := context.NewContext()
	err = context.AddResource(ctx, resourceRaw)
	if err != nil {
		t.Error(err)
	}
	value, err := ctx.Query("request.object.metadata.name")

	t.Log(value)
	if err != nil {
		t.Error(err)
	}
	policyContext := &PolicyContext{
		Policy:      &policy,
		JSONContext: ctx,
		NewResource: *resourceUnstructured}
	er := Mutate(policyContext)
	t.Log(string(expectedPatch))

	assert.Equal(t, len(er.PolicyResponse.Rules), 1)
	assert.Equal(t, len(er.PolicyResponse.Rules[0].Patches), 1)
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
	policyRaw := []byte(`{
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
          "patchStrategicMerge": {
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
	err := json.Unmarshal(policyRaw, &policy)
	assert.NilError(t, err)
	resourceUnstructured, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = context.AddResource(ctx, resourceRaw)
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      &policy,
		JSONContext: ctx,
		NewResource: *resourceUnstructured}
	er := Mutate(policyContext)
	assert.Equal(t, len(er.PolicyResponse.Rules), 1)
	assert.Assert(t, strings.Contains(er.PolicyResponse.Rules[0].Message, "Unknown key \"name1\" in path"))
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
	policyRaw := []byte(`{
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
						Values: map[string]interface{}{
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
	err := json.Unmarshal(policyRaw, &policy)
	assert.NilError(t, err)
	resourceUnstructured, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = context.AddResource(ctx, resourceRaw)
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      &policy,
		JSONContext: ctx,
		NewResource: *resourceUnstructured,
	}

	er := Mutate(policyContext)
	assert.Equal(t, len(er.PolicyResponse.Rules), 1)
	assert.Equal(t, len(er.PolicyResponse.Rules[0].Patches), 1)
	t.Log(string(expectedPatch))
	t.Log(string(er.PolicyResponse.Rules[0].Patches[0]))
	if !reflect.DeepEqual(expectedPatch, er.PolicyResponse.Rules[0].Patches[0]) {
		t.Error("patches don't match")
	}
}

// https://github.com/kyverno/kyverno/issues/2022
func Test_chained_rules(t *testing.T) {
	policyRaw := []byte(`{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "replace-image-registry",
    "annotations": {
      "policies.kyverno.io/minversion": "1.4.2"
    }
  },
  "spec": {
    "background": false,
    "rules": [
      {
        "name": "replace-image-registry",
        "match": {
          "resources": {
            "kinds": [
              "Pod"
            ]
          }
        },
        "mutate": {
          "patchStrategicMerge": {
            "spec": {
              "containers": [
                {
                  "(name)": "*",
                  "image": "{{regex_replace_all('^[^/]+','{{@}}','myregistry.corp.com')}}"
                }
              ]
            }
          }
        }
      },
      {
        "name": "replace-image-registry-chained",
        "match": {
          "resources": {
            "kinds": [
              "Pod"
            ]
          }
        },
        "mutate": {
          "patchStrategicMerge": {
            "spec": {
              "containers": [
                {
                  "(name)": "*",
                  "image": "{{regex_replace_all('\\b(myregistry.corp.com)\\b','{{@}}','otherregistry.corp.com')}}"
                }
              ]
            }
          }
        }
      }
    ]
  }
}`)
	resourceRaw := []byte(`{
  "apiVersion": "v1",
  "kind": "Pod",
  "metadata": {
    "name": "test"
  },
  "spec": {
    "containers": [
      {
        "name": "test",
        "image": "foo/bash:5.0"
      }
    ]
  }
}`)
	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(policyRaw, &policy)
	assert.NilError(t, err)

	resource, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = ctx.AddResource(resource.Object)
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      &policy,
		JSONContext: ctx,
		NewResource: *resource,
	}

	err = ctx.AddImageInfos(resource)
	assert.NilError(t, err)

	err = context.MutateResourceWithImageInfo(resourceRaw, ctx)
	assert.NilError(t, err)

	er := Mutate(policyContext)
	containers, _, err := unstructured.NestedSlice(er.PatchedResource.Object, "spec", "containers")
	assert.NilError(t, err)
	assert.Equal(t, containers[0].(map[string]interface{})["image"], "otherregistry.corp.com/foo/bash:5.0")

	assert.Equal(t, len(er.PolicyResponse.Rules), 2)
	assert.Equal(t, len(er.PolicyResponse.Rules[0].Patches), 1)
	assert.Equal(t, len(er.PolicyResponse.Rules[1].Patches), 1)

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
	policyRaw := []byte(`{
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
	expectedPatch := []byte(`{"op":"add","path":"/metadata/labels/my-added-label","value":"test"}`)

	store.SetMock(true)
	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(policyRaw, &policy)
	assert.NilError(t, err)
	resourceUnstructured, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = context.AddResource(ctx, resourceRaw)
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      &policy,
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

func Test_nonZeroIndexNumberPatchesJson6902(t *testing.T) {
	resourceRaw := []byte(`{
  "apiVersion": "v1",
  "kind": "Endpoints",
  "metadata": {
    "name": "my-service"
  },
  "subsets": [
    {
      "addresses": [
        {
          "ip": "127.0.0.1"
        }
      ]
    }
  ]
}`)

	policyraw := []byte(`{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "policy-endpoints"
  },
  "spec": {
    "rules": [
      {
        "name": "Add IP to subset",
        "match": {
          "resources": {
            "kinds": [
              "Endpoints"
            ]
          }
        },
        "preconditions": [
          {
            "key": "{{ request.object.subsets[] | length(@) }}",
            "operator": "Equals",
            "value": "1"
          }
        ],
        "mutate": {
          "patchesJson6902": "- path: \"/subsets/0/addresses/-\"\n  op: add\n  value: {\"ip\":\"192.168.42.172\"}"
        }
      },
      {
        "name": "Add IP to subsets",
        "match": {
          "resources": {
            "kinds": [
              "Endpoints"
            ]
          }
        },
        "preconditions": [
          {
            "key": "{{ request.object.subsets[] | length(@) }}",
            "operator": "Equals",
            "value": "2"
          }
        ],
        "mutate": {
          "patchesJson6902": "- path: \"/subsets/0/addresses/-\"\n  op: add\n  value: {\"ip\":\"192.168.42.172\"}\n- path: \"/subsets/1/addresses/-\"\n  op: add\n  value: {\"ip\":\"192.168.42.173\"}"
        }
      }
    ]
  }
}`)

	expectedPatch := []byte(`{"op":"add","path":"/subsets/0/addresses/1","value":{"ip":"192.168.42.172"}}`)

	store.SetMock(true)
	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(policyraw, &policy)
	assert.NilError(t, err)
	resourceUnstructured, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = context.AddResource(ctx, resourceRaw)
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      &policy,
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

func Test_foreach(t *testing.T) {
	policyRaw := []byte(`{
    "apiVersion": "kyverno.io/v1",
    "kind": "ClusterPolicy",
    "metadata": {
      "name": "replace-image-registry"
    },
    "spec": {
      "background": false,
      "rules": [
        {
          "name": "replace-image-registry",
          "match": {
            "resources": {
              "kinds": [
                "Pod"
              ]
            }
          },
          "mutate": {
            "foreach": [
              {
                "list": "request.object.spec.containers",
                "patchStrategicMerge": {
                  "spec": {
                    "containers": [
                      {
                        "name": "{{ element.name }}",
                        "image": "registry.io/{{images.containers.{{element.name}}.path}}:{{images.containers.{{element.name}}.tag}}"
                      }
                    ]
                  }
                }
              }
            ]
          }
        }
      ]
    }
  }`)
	resourceRaw := []byte(`{
  "apiVersion": "v1",
  "kind": "Pod",
  "metadata": {
    "name": "test"
  },
  "spec": {
    "containers": [
      {
        "name": "test1",
        "image": "foo1/bash1:5.0"
      },
      {
        "name": "test2",
        "image": "foo2/bash2:5.0"
      },
      {
        "name": "test3",
        "image": "foo3/bash3:5.0"
      }
    ]
  }
}`)
	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(policyRaw, &policy)
	assert.NilError(t, err)

	resource, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = ctx.AddResource(resource.Object)
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      &policy,
		JSONContext: ctx,
		NewResource: *resource,
	}

	err = ctx.AddImageInfos(resource)
	assert.NilError(t, err)

	err = context.MutateResourceWithImageInfo(resourceRaw, ctx)
	assert.NilError(t, err)

	er := Mutate(policyContext)

	assert.Equal(t, len(er.PolicyResponse.Rules), 1)
	assert.Equal(t, er.PolicyResponse.Rules[0].Status, response.RuleStatusPass)

	containers, _, err := unstructured.NestedSlice(er.PatchedResource.Object, "spec", "containers")
	assert.NilError(t, err)
	for _, c := range containers {
		ctnr := c.(map[string]interface{})
		switch ctnr["name"] {
		case "test1":
			assert.Equal(t, ctnr["image"], "registry.io/foo1/bash1:5.0")
		case "test2":
			assert.Equal(t, ctnr["image"], "registry.io/foo2/bash2:5.0")
		case "test3":
			assert.Equal(t, ctnr["image"], "registry.io/foo3/bash3:5.0")
		}
	}
}

func Test_foreach_element_mutation(t *testing.T) {
	policyRaw := []byte(`{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "mutate-privileged"
  },
  "spec": {
	"validationFailureAction": "audit",
    "background": false,
	"webhookTimeoutSeconds": 10,
	"failurePolicy": "Fail",
    "rules": [
      {
        "name": "set-privileged",
        "match": {
          "resources": {
            "kinds": [
              "Pod"
            ]
          }
        },
        "mutate": {
          "foreach": [
			  {
				"list": "request.object.spec.containers",
				"patchStrategicMerge": {
				  "spec": {
					"containers": [
					  {
						"(name)": "{{ element.name }}",
						"securityContext": {
						  "privileged": false
						}
					  }
					]
				  }
				}
			  }
		  ]
        }
      }
    ]
  }
}`)
	resourceRaw := []byte(`{
  "apiVersion": "v1",
  "kind": "Pod",
  "metadata": {
    "name": "nginx"
  },
  "spec": {
    "containers": [
      {
        "name": "nginx1",
        "image": "nginx"
      },
      {
        "name": "nginx2",
        "image": "nginx"
      }
    ]
  }
}`)
	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(policyRaw, &policy)
	assert.NilError(t, err)

	resource, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = ctx.AddResource(resource.Object)
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      &policy,
		JSONContext: ctx,
		NewResource: *resource,
	}

	err = ctx.AddImageInfos(resource)
	assert.NilError(t, err)

	err = context.MutateResourceWithImageInfo(resourceRaw, ctx)
	assert.NilError(t, err)

	er := Mutate(policyContext)

	assert.Equal(t, len(er.PolicyResponse.Rules), 1)
	assert.Equal(t, er.PolicyResponse.Rules[0].Status, response.RuleStatusPass)

	containers, _, err := unstructured.NestedSlice(er.PatchedResource.Object, "spec", "containers")
	assert.NilError(t, err)
	for _, c := range containers {
		ctnr := c.(map[string]interface{})
		_securityContext, ok := ctnr["securityContext"]
		assert.Assert(t, ok)

		securityContext := _securityContext.(map[string]interface{})
		assert.Equal(t, securityContext["privileged"], false)
	}
}

func Test_Container_InitContainer_foreach(t *testing.T) {
	policyRaw := []byte(`{
    "apiVersion": "kyverno.io/v1",
    "kind": "ClusterPolicy",
    "metadata": {
       "name": "prepend-registry",
       "annotations": {
          "pod-policies.kyverno.io/autogen-controllers": "none"
       }
    },
    "spec": {
       "background": false,
       "rules": [
          {
             "name": "prepend-registry-containers",
             "match": {
                "resources": {
                   "kinds": [
                      "Pod"
                   ]
                }
             },
             "mutate": {
                "foreach": [
                   {
                      "list": "request.object.spec.containers",
                      "patchStrategicMerge": {
                         "spec": {
                            "containers": [
                               {
                                  "name": "{{ element.name }}",
                                  "image": "registry.io/{{ images.containers.\"{{element.name}}\".path}}:{{images.containers.\"{{element.name}}\".tag}}"
                               }
                            ]
                         }
                      }
                   },
                   {
                      "list": "request.object.spec.initContainers",
                      "patchStrategicMerge": {
                         "spec": {
                            "initContainers": [
                               {
                                  "name": "{{ element.name }}",
                                  "image": "registry.io/{{ images.initContainers.\"{{element.name}}\".name}}:{{images.initContainers.\"{{element.name}}\".tag}}"
                               }
                            ]
                         }
                      }
                   }
                ]
             }
          }
       ]
    }
 }`)
	resourceRaw := []byte(`{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
       "name": "mypod"
    },
    "spec": {
       "automountServiceAccountToken": false,
       "initContainers": [
          {
             "name": "alpine",
             "image": "alpine:latest"
          },
          {
             "name": "busybox",
             "image": "busybox:1.28"
          }
       ],
       "containers": [
          {
             "name": "nginx",
             "image": "nginx:1.2.3"
          },
          {
             "name": "redis",
             "image": "redis:latest"
          }
       ]
    }
 }`)
	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(policyRaw, &policy)
	assert.NilError(t, err)

	resource, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = ctx.AddResource(resource.Object)
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      &policy,
		JSONContext: ctx,
		NewResource: *resource,
	}

	err = ctx.AddImageInfos(resource)
	assert.NilError(t, err)

	err = context.MutateResourceWithImageInfo(resourceRaw, ctx)
	assert.NilError(t, err)

	er := Mutate(policyContext)

	assert.Equal(t, len(er.PolicyResponse.Rules), 1)
	assert.Equal(t, er.PolicyResponse.Rules[0].Status, response.RuleStatusPass)

	containers, _, err := unstructured.NestedSlice(er.PatchedResource.Object, "spec", "containers")
	assert.NilError(t, err)
	for _, c := range containers {
		ctnr := c.(map[string]interface{})
		switch ctnr["name"] {
		case "alpine":
			assert.Equal(t, ctnr["image"], "registry.io/alpine:latest")
		case "busybox":
			assert.Equal(t, ctnr["image"], "registry.io/busybox:1.28")
		}
	}

	initContainers, _, err := unstructured.NestedSlice(er.PatchedResource.Object, "spec", "initContainers")
	assert.NilError(t, err)
	for _, c := range initContainers {
		ctnr := c.(map[string]interface{})
		switch ctnr["name"] {
		case "nginx":
			assert.Equal(t, ctnr["image"], "registry.io/nginx:1.2.3")
		case "redis":
			assert.Equal(t, ctnr["image"], "registry.io/redis:latest")
		}
	}
}

func Test_foreach_order_mutation_(t *testing.T) {
	policyRaw := []byte(`{
    "apiVersion": "kyverno.io/v1",
    "kind": "ClusterPolicy",
    "metadata": {
      "name": "replace-image"
    },
    "spec": {
      "background": false,
      "rules": [
        {
          "name": "replace-image",
          "match": {
            "all": [
              {
                "resources": {
                  "kinds": [
                    "Pod"
                  ]
                }
              }
            ]
          },
          "mutate": {
            "foreach": [
              {
                "list": "request.object.spec.containers",
                "patchStrategicMerge": {
                  "spec": {
                    "containers": [
                      {
                        "(name)": "{{ element.name }}",
                        "image": "replaced"
                      }
                    ]
                  }
                }
              }
            ]
          }
        }
      ]
    }
  }`)
	resourceRaw := []byte(`{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
      "name": "mongodb",
      "labels": {
        "app": "mongodb"
      }
    },
    "spec": {
      "containers": [
        {
          "image": "docker.io/mongo:5.0.3",
          "name": "mongod"
        },
        {
          "image": "nginx",
          "name": "nginx"
        },
        {
          "image": "nginx",
          "name": "nginx3"
        },
        {
          "image": "quay.io/mongodb/mongodb-agent:11.0.5.6963-1",
          "name": "mongodb-agent"
        }
      ]
    }
  }`)
	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(policyRaw, &policy)
	assert.NilError(t, err)

	resource, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = ctx.AddResource(resource.Object)
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      &policy,
		JSONContext: ctx,
		NewResource: *resource,
	}

	err = ctx.AddImageInfos(resource)
	assert.NilError(t, err)

	err = context.MutateResourceWithImageInfo(resourceRaw, ctx)
	assert.NilError(t, err)

	er := Mutate(policyContext)

	assert.Equal(t, len(er.PolicyResponse.Rules), 1)
	assert.Equal(t, er.PolicyResponse.Rules[0].Status, response.RuleStatusPass)

	containers, _, err := unstructured.NestedSlice(er.PatchedResource.Object, "spec", "containers")
	assert.NilError(t, err)

	for i, c := range containers {
		ctnr := c.(map[string]interface{})
		switch i {
		case 0:
			assert.Equal(t, ctnr["name"], "mongod")
		case 1:
			assert.Equal(t, ctnr["name"], "nginx")
		case 3:
			assert.Equal(t, ctnr["name"], "mongodb-agent")
		}
	}
}

func Test_mutate_existing_resources(t *testing.T) {
	tests := []struct {
		name       string
		policy     []byte
		trigger    []byte
		targets    [][]byte
		targetList string
		patches    []string
	}{
		{
			name: "test-different-trigger-target",
			policy: []byte(`{
		        "apiVersion": "kyverno.io/v1",
		        "kind": "ClusterPolicy",
		        "metadata": {
		            "name": "test-post-mutation"
		        },
		        "spec": {
		            "rules": [
		                {
		                    "name": "mutate-deploy-on-configmap-update",
		                    "match": {
		                        "any": [
		                            {
		                                "resources": {
		                                    "kinds": [
		                                        "ConfigMap"
		                                    ],
		                                    "names": [
		                                        "dictionary"
		                                    ],
		                                    "namespaces": [
		                                        "staging"
		                                    ]
		                                }
		                            }
		                        ]
		                    },
		                    "preconditions": {
		                        "any": [
		                            {
		                                "key": "{{ request.object.data.foo }}",
		                                "operator": "Equals",
		                                "value": "bar"
		                            }
		                        ]
		                    },
		                    "mutate": {
		                        "targets": [
		                            {
		                                "apiVersion": "v1",
		                                "kind": "Deployment",
		                                "name": "example-A",
		                                "namespace": "staging"
		                            }
		                        ],
		                        "patchStrategicMerge": {
		                            "metadata": {
		                                "labels": {
		                                    "foo": "bar"
		                                }
		                            }
		                        }
		                    }
		                }
		            ]
		        }
		    }`),
			trigger: []byte(`{
		    "apiVersion": "v1",
		    "data": {
		        "foo": "bar"
		    },
		    "kind": "ConfigMap",
		    "metadata": {
		        "name": "dictionary",
		        "namespace": "staging"
		    }
		}`),
			targets: [][]byte{[]byte(`{
		    "apiVersion": "apps/v1",
		    "kind": "Deployment",
		    "metadata": {
		        "name": "example-A",
		        "namespace": "staging",
		        "labels": {
		            "app": "nginx"
		        }
		    },
		    "spec": {
		        "replicas": 1,
		        "selector": {
		            "matchLabels": {
		                "app": "nginx"
		            }
		        },
		        "template": {
		            "metadata": {
		                "labels": {
		                    "app": "nginx"
		                }
		            },
		            "spec": {
		                "containers": [
		                    {
		                        "name": "nginx",
		                        "image": "nginx:1.14.2",
		                        "ports": [
		                            {
		                                "containerPort": 80
		                            }
		                        ]
		                    }
		                ]
		            }
		        }
		    }
		}`)},
			targetList: "DeploymentList",
			patches:    []string{`{"op":"add","path":"/metadata/labels/foo","value":"bar"}`},
		},
		{
			name: "test-same-trigger-target",
			policy: []byte(`{
		        "apiVersion": "kyverno.io/v1",
		        "kind": "ClusterPolicy",
		        "metadata": {
		            "name": "test-post-mutation"
		        },
		        "spec": {
		            "rules": [
		                {
		                    "name": "mutate-deploy-on-configmap-update",
		                    "match": {
		                        "any": [
		                            {
		                                "resources": {
		                                    "kinds": [
		                                        "ConfigMap"
		                                    ],
		                                    "names": [
		                                        "dictionary"
		                                    ],
		                                    "namespaces": [
		                                        "staging"
		                                    ]
		                                }
		                            }
		                        ]
		                    },
		                    "preconditions": {
		                        "any": [
		                            {
		                                "key": "{{ request.object.data.foo }}",
		                                "operator": "Equals",
		                                "value": "bar"
		                            }
		                        ]
		                    },
		                    "mutate": {
		                        "targets": [
		                            {
		                                "apiVersion": "v1",
		                                "kind": "ConfigMap",
		                                "name": "dictionary",
		                                "namespace": "staging"
		                            }
		                        ],
		                        "patchStrategicMerge": {
		                            "metadata": {
		                                "labels": {
		                                    "foo": "bar"
		                                }
		                            }
		                        }
		                    }
		                }
		            ]
		        }
		    }`),
			trigger: []byte(`{
		    "apiVersion": "v1",
		    "data": {
		        "foo": "bar"
		    },
		    "kind": "ConfigMap",
		    "metadata": {
		        "name": "dictionary",
		        "namespace": "staging"
		    }
		}`),
			targets: [][]byte{[]byte(`{
		    "apiVersion": "v1",
		    "data": {
		        "foo": "bar"
		    },
		    "kind": "ConfigMap",
		    "metadata": {
		        "name": "dictionary",
		        "namespace": "staging"
		    }
		}`)},
			targetList: "ComfigMapList",
			patches:    []string{`{"op":"add","path":"/metadata/labels","value":{"foo":"bar"}}`},
		},
		{
			name: "test-in-place-variable",
			policy: []byte(`
      {
        "apiVersion": "kyverno.io/v1",
        "kind": "ClusterPolicy",
        "metadata": {
            "name": "sync-cms"
        },
        "spec": {
            "mutateExistingOnPolicyUpdate": false,
            "rules": [
                {
                    "name": "concat-cm",
                    "match": {
                        "any": [
                            {
                                "resources": {
                                    "kinds": [
                                        "ConfigMap"
                                    ],
                                    "names": [
                                        "cmone"
                                    ],
                                    "namespaces": [
                                        "foo"
                                    ]
                                }
                            }
                        ]
                    },
                    "mutate": {
                        "targets": [
                            {
                                "apiVersion": "v1",
                                "kind": "ConfigMap",
                                "name": "cmtwo",
                                "namespace": "bar"
                            }
                        ],
                        "patchStrategicMerge": {
                            "data": {
                                "keytwo": "{{@}}-{{request.object.data.keyone}}"
                            }
                        }
                    }
                }
            ]
        }
    }
`),
			trigger: []byte(`
      {
        "apiVersion": "v1",
        "data": {
            "keyone": "valueone"
        },
        "kind": "ConfigMap",
        "metadata": {
            "name": "cmone",
            "namespace": "foo"
        }
    }
`),
			targets: [][]byte{[]byte(`
      {
        "apiVersion": "v1",
        "data": {
            "keytwo": "valuetwo"
        },
        "kind": "ConfigMap",
        "metadata": {
            "name": "cmtwo",
            "namespace": "bar"
        }
    }
`)},
			targetList: "ComfigMapList",
			patches:    []string{`{"op":"replace","path":"/data/keytwo","value":"valuetwo-valueone"}`},
		},
		{
			name: "test-in-place-variable",
			policy: []byte(`
      {
        "apiVersion": "kyverno.io/v1",
        "kind": "ClusterPolicy",
        "metadata": {
            "name": "sync-cms"
        },
        "spec": {
            "mutateExistingOnPolicyUpdate": false,
            "rules": [
                {
                    "name": "concat-cm",
                    "match": {
                        "any": [
                            {
                                "resources": {
                                    "kinds": [
                                        "ConfigMap"
                                    ],
                                    "names": [
                                        "cmone"
                                    ],
                                    "namespaces": [
                                        "foo"
                                    ]
                                }
                            }
                        ]
                    },
                    "mutate": {
                        "targets": [
                            {
                                "apiVersion": "v1",
                                "kind": "ConfigMap",
                                "name": "cmtwo",
                                "namespace": "bar"
                            },
                            {
                                "apiVersion": "v1",
                                "kind": "ConfigMap",
                                "name": "cmthree",
                                "namespace": "bar"
                            }
                        ],
                        "patchStrategicMerge": {
                            "data": {
                                "key": "{{@}}-{{request.object.data.keyone}}"
                            }
                        }
                    }
                }
            ]
        }
    }
`),
			trigger: []byte(`
      {
        "apiVersion": "v1",
        "data": {
            "keyone": "valueone"
        },
        "kind": "ConfigMap",
        "metadata": {
            "name": "cmone",
            "namespace": "foo"
        }
    }
`),
			targets: [][]byte{
				[]byte(`
      {
        "apiVersion": "v1",
        "data": {
            "key": "valuetwo"
        },
        "kind": "ConfigMap",
        "metadata": {
            "name": "cmtwo",
            "namespace": "bar"
        }
    }
`),
				[]byte(`
				      {
				        "apiVersion": "v1",
				        "data": {
				            "key": "valuethree"
				        },
				        "kind": "ConfigMap",
				        "metadata": {
				            "name": "cmthree",
				            "namespace": "bar"
				        }
				    }
				`),
			},
			targetList: "ComfigMapList",
			patches:    []string{`{"op":"replace","path":"/data/key","value":"valuetwo-valueone"}`, `{"op":"replace","path":"/data/key","value":"valuethree-valueone"}`},
		},
	}

	var policyContext *PolicyContext
	for _, test := range tests {
		var policy kyverno.ClusterPolicy
		err := json.Unmarshal(test.policy, &policy)
		assert.NilError(t, err)

		trigger, err := utils.ConvertToUnstructured(test.trigger)
		assert.NilError(t, err)

		for _, target := range test.targets {
			target, err := utils.ConvertToUnstructured(target)
			assert.NilError(t, err)

			ctx := context.NewContext()
			err = ctx.AddResource(trigger.Object)
			assert.NilError(t, err)

			gvrToListKind := map[schema.GroupVersionResource]string{
				{Group: target.GroupVersionKind().Group, Version: target.GroupVersionKind().Version, Resource: target.GroupVersionKind().Kind}: test.targetList,
			}

			objects := []runtime.Object{target}
			scheme := runtime.NewScheme()
			dclient, err := client.NewFakeClient(scheme, gvrToListKind, objects...)
			assert.NilError(t, err)
			dclient.SetDiscovery(client.NewFakeDiscoveryClient(nil))

			_, err = dclient.GetResource(target.GetAPIVersion(), target.GetKind(), target.GetNamespace(), target.GetName())
			assert.NilError(t, err)

			policyContext = &PolicyContext{
				Client:      dclient,
				Policy:      &policy,
				JSONContext: ctx,
				NewResource: *trigger,
			}
		}
		er := Mutate(policyContext)

		for _, rr := range er.PolicyResponse.Rules {
			for i, p := range rr.Patches {
				assert.Equal(t, test.patches[i], string(p), "test %s failed:\nGot %s\nExpected: %s", test.name, rr.Patches[i], test.patches[i])
				assert.Equal(t, rr.Status, response.RuleStatusPass, rr.Status)
			}
		}
	}
}

func Test_RuleSelectorMutate(t *testing.T) {
	policyRaw := []byte(`{
    "apiVersion": "kyverno.io/v1",
    "kind": "ClusterPolicy",
    "metadata": {
      "name": "add-label"
    },
    "spec": {
      "rules": [
        {
          "name": "add-app-label",
          "match": {
            "resources": {
              "name": "check-root-user",
              "kinds": [
                "Pod"
              ]
            }
          },
          "mutate": {
            "patchStrategicMerge": {
              "metadata": {
                "labels": {
                  "app": "root"
                }
              }
            }
          }
        },
        {
          "name": "add-appname-label",
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
                  "appname": "{{request.object.metadata.name}}"
                }
              }
            }
          }
        }        
      ]
    }
  }`)

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

	expectedPatch1 := []byte(`{"op":"add","path":"/metadata/labels","value":{"app":"root"}}`)
	expectedPatch2 := []byte(`{"op":"add","path":"/metadata/labels/appname","value":"check-root-user"}`)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(policyRaw, &policy)
	if err != nil {
		t.Error(err)
	}

	resourceUnstructured, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)
	ctx := context.NewContext()
	err = context.AddResource(ctx, resourceRaw)
	if err != nil {
		t.Error(err)
	}

	_, err = ctx.Query("request.object.metadata.name")
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      &policy,
		JSONContext: ctx,
		NewResource: *resourceUnstructured,
	}

	er := Mutate(policyContext)
	assert.Equal(t, len(er.PolicyResponse.Rules), 2)
	assert.Equal(t, len(er.PolicyResponse.Rules[0].Patches), 1)
	assert.Equal(t, len(er.PolicyResponse.Rules[1].Patches), 1)

	if !reflect.DeepEqual(expectedPatch1, er.PolicyResponse.Rules[0].Patches[0]) {
		t.Error("rule 1 patches dont match")
	}
	if !reflect.DeepEqual(expectedPatch2, er.PolicyResponse.Rules[1].Patches[0]) {
		t.Errorf("rule 2 patches dont match")
	}

	applyOne := kyverno.ApplyOne
	policyContext.Policy.GetSpec().ApplyRules = &applyOne

	er = Mutate(policyContext)
	assert.Equal(t, len(er.PolicyResponse.Rules), 1)
	assert.Equal(t, len(er.PolicyResponse.Rules[0].Patches), 1)

	if !reflect.DeepEqual(expectedPatch1, er.PolicyResponse.Rules[0].Patches[0]) {
		t.Error("rule 1 patches dont match")
	}
}

func Test_SpecialCharacters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		policyRaw   []byte
		documentRaw []byte
		want        [][]byte
	}{
		{
			name: "regex_replace",
			policyRaw: []byte(`{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "regex-replace-all-demo"
  },
  "spec": {
    "background": false,
    "rules": [
      {
        "name": "retention-adjust",
        "match": {
          "any": [
            {
              "resources": {
                "kinds": [
                  "Deployment"
                ]
              }
            }
          ]
        },
        "mutate": {
          "patchStrategicMerge": {
            "metadata": {
              "labels": {
                "retention": "{{ regex_replace_all('([0-9])([0-9])', '{{ @ }}', '${1}0') }}"
              }
            }
          }
        }
      }
    ]
  }
}`),
			documentRaw: []byte(`{
  "apiVersion": "apps/v1",
  "kind": "Deployment",
  "metadata": {
    "name": "busybox",
    "labels": {
      "app": "busybox",
      "retention": "days_37"
    }
  },
  "spec": {
    "replicas": 3,
    "selector": {
      "matchLabels": {
        "app": "busybox"
      }
    },
    "template": {
      "metadata": {
        "labels": {
          "app": "busybox"
        }
      },
      "spec": {
        "containers": [
          {
            "image": "busybox:1.28",
            "name": "busybox",
            "command": [
              "sleep",
              "9999"
            ]
          }
        ]
      }
    }
  }
}`),
			want: [][]byte{
				[]byte(`{"op":"replace","path":"/metadata/labels/retention","value":"days_30"}`),
			},
		},
		{
			name: "regex_replace_with_slash",
			policyRaw: []byte(`{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "regex-replace-all-demo"
  },
  "spec": {
    "background": false,
    "rules": [
      {
        "name": "retention-adjust",
        "match": {
          "any": [
            {
              "resources": {
                "kinds": [
                  "Deployment"
                ]
              }
            }
          ]
        },
        "mutate": {
          "patchStrategicMerge": {
            "metadata": {
              "labels": {
                "corp.com/retention": "{{ regex_replace_all('([0-9])([0-9])', '{{ @ }}', '${1}0') }}"
              }
            }
          }
        }
      }
    ]
  }
}`),
			documentRaw: []byte(`{
  "apiVersion": "apps/v1",
  "kind": "Deployment",
  "metadata": {
    "name": "busybox",
    "labels": {
      "app": "busybox",
      "corp.com/retention": "days_37"
    }
  },
  "spec": {
    "replicas": 3,
    "selector": {
      "matchLabels": {
        "app": "busybox"
      }
    },
    "template": {
      "metadata": {
        "labels": {
          "app": "busybox"
        }
      },
      "spec": {
        "containers": [
          {
            "image": "busybox:1.28",
            "name": "busybox",
            "command": [
              "sleep",
              "9999"
            ]
          }
        ]
      }
    }
  }
}`),
			want: [][]byte{
				[]byte(`{"op":"replace","path":"/metadata/labels/corp.com~1retention","value":"days_30"}`),
			},
		},
		{
			name: "regex_replace_with_hyphen",
			policyRaw: []byte(`{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "regex-replace-all-demo"
  },
  "spec": {
    "background": false,
    "rules": [
      {
        "name": "retention-adjust",
        "match": {
          "any": [
            {
              "resources": {
                "kinds": [
                  "Deployment"
                ]
              }
            }
          ]
        },
        "mutate": {
          "patchStrategicMerge": {
            "metadata": {
              "labels": {
                "corp-retention": "{{ regex_replace_all('([0-9])([0-9])', '{{ @ }}', '${1}0') }}"
              }
            }
          }
        }
      }
    ]
  }
}`),
			documentRaw: []byte(`{
  "apiVersion": "apps/v1",
  "kind": "Deployment",
  "metadata": {
    "name": "busybox",
    "labels": {
      "app": "busybox",
      "corp-retention": "days_37"
    }
  },
  "spec": {
    "replicas": 3,
    "selector": {
      "matchLabels": {
        "app": "busybox"
      }
    },
    "template": {
      "metadata": {
        "labels": {
          "app": "busybox"
        }
      },
      "spec": {
        "containers": [
          {
            "image": "busybox:1.28",
            "name": "busybox",
            "command": [
              "sleep",
              "9999"
            ]
          }
        ]
      }
    }
  }
}`),
			want: [][]byte{
				[]byte(`{"op":"replace","path":"/metadata/labels/corp-retention","value":"days_30"}`),
			},
		},
		{
			name: "to_upper_with_hyphen",
			policyRaw: []byte(`{
  "apiVersion": "kyverno.io/v1",
  "kind": "ClusterPolicy",
  "metadata": {
    "name": "to-upper-demo"
  },
  "spec": {
    "rules": [
      {
        "name": "format-deploy-zone",
        "match": {
          "any": [
            {
              "resources": {
                "kinds": [
                  "Deployment"
                ]
              }
            }
          ]
        },
        "mutate": {
          "patchStrategicMerge": {
            "metadata": {
              "labels": {
                "deploy-zone": "{{ to_upper('{{@}}') }}"
              }
            }
          }
        }
      }
    ]
  }
}`),
			documentRaw: []byte(`{
  "apiVersion": "apps/v1",
  "kind": "Deployment",
  "metadata": {
    "name": "busybox",
    "labels": {
      "app": "busybox",
      "deploy-zone": "eu-central-1"
    }
  },
  "spec": {
    "replicas": 3,
    "selector": {
      "matchLabels": {
        "app": "busybox"
      }
    },
    "template": {
      "metadata": {
        "labels": {
          "app": "busybox"
        }
      },
      "spec": {
        "containers": [
          {
            "image": "busybox:1.28",
            "name": "busybox",
            "command": [
              "sleep",
              "9999"
            ]
          }
        ]
      }
    }
  }
}`),
			want: [][]byte{
				[]byte(`{"op":"replace","path":"/metadata/labels/deploy-zone","value":"EU-CENTRAL-1"}`),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Parse policy document.
			var policy kyverno.ClusterPolicy
			if err := json.Unmarshal(tt.policyRaw, &policy); err != nil {
				t.Error(err)
			}

			// Parse resource document.
			resource, err := utils.ConvertToUnstructured(tt.documentRaw)
			if err != nil {
				t.Fatalf("ConvertToUnstructured() error = %v", err)
			}

			// Create JSON context and add the resource.
			ctx := context.NewContext()
			err = ctx.AddResource(resource.Object)
			if err != nil {
				t.Fatalf("ctx.AddResource() error = %v", err)
			}

			// Create policy context.
			policyContext := &PolicyContext{
				Policy:      &policy,
				JSONContext: ctx,
				NewResource: *resource,
			}

			// Mutate and make sure that we got the expected amount of rules.
			patches := Mutate(policyContext).GetPatches()
			if !reflect.DeepEqual(patches, tt.want) {
				t.Errorf("Mutate() got patches %s, expected %s", patches, tt.want)
			}
		})
	}
}
