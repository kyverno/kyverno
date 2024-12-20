package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	client "github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	"github.com/kyverno/kyverno/pkg/imageverifycache"
	"github.com/kyverno/kyverno/pkg/registryclient"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func testMutate(
	ctx context.Context,
	client client.Interface,
	rclient registryclient.Client,
	pContext *PolicyContext,
	contextLoader engineapi.ContextLoaderFactory,
) engineapi.EngineResponse {
	if contextLoader == nil {
		contextLoader = factories.DefaultContextLoaderFactory(nil)
	}
	e := NewEngine(
		cfg,
		config.NewDefaultMetricsConfiguration(),
		jp,
		adapters.Client(client),
		factories.DefaultRegistryClientFactory(adapters.RegistryClient(rclient), nil),
		imageverifycache.DisabledImageVerifyCache(),
		contextLoader,
		nil,
		nil,
	)
	return e.Mutate(
		ctx,
		pContext,
	)
}

func loadResource[T any](t *testing.T, bytes []byte) T {
	var result T
	require.NoError(t, json.Unmarshal(bytes, &result))
	return result
}

func loadUnstructured(t *testing.T, bytes []byte) unstructured.Unstructured {
	var resource unstructured.Unstructured
	require.NoError(t, resource.UnmarshalJSON(bytes))
	return resource
}

func createContext(t *testing.T, policy kyverno.PolicyInterface, resource unstructured.Unstructured) *PolicyContext {
	ctx, err := NewPolicyContext(
		jp,
		resource,
		kyverno.Create,
		nil,
		cfg,
	)
	require.NoError(t, err)
	return ctx.WithPolicy(policy)
}

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
	policy := loadResource[kyverno.ClusterPolicy](t, policyRaw)
	resource := loadUnstructured(t, resourceRaw)
	policyContext := createContext(t, &policy, resource)

	er := testMutate(context.TODO(), nil, nil, policyContext, nil)
	require.Equal(t, 1, len(er.PolicyResponse.Rules))

	patched := er.PatchedResource
	require.NotEqual(t, resource, patched)
	unstructured.SetNestedField(resource.UnstructuredContent(), "check-root-user", "metadata", "labels", "appname")
	require.Equal(t, resource, patched)
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

	policy := loadResource[kyverno.ClusterPolicy](t, policyRaw)
	resource := loadUnstructured(t, resourceRaw)
	policyContext := createContext(t, &policy, resource)

	er := testMutate(context.TODO(), nil, nil, policyContext, nil)
	assert.Equal(t, len(er.PolicyResponse.Rules), 1)

	assert.Assert(t, strings.Contains(er.PolicyResponse.Rules[0].Message(), "Unknown key \"name1\" in path"))
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

	policy := loadResource[kyverno.ClusterPolicy](t, policyRaw)
	resource := loadUnstructured(t, resourceRaw)
	policyContext := createContext(t, &policy, resource)
	ctxLoaderFactory := factories.DefaultContextLoaderFactory(
		nil,
		factories.WithInitializer(func(jsonContext enginecontext.Interface) error {
			if err := jsonContext.AddVariable("dictionary.data.env", "dev1"); err != nil {
				return err
			}
			return nil
		}),
	)

	er := testMutate(
		context.TODO(),
		nil,
		nil,
		policyContext,
		ctxLoaderFactory,
	)

	require.Equal(t, 1, len(er.PolicyResponse.Rules))

	patched := er.PatchedResource
	require.NotEqual(t, resource, patched)
	unstructured.SetNestedField(resource.UnstructuredContent(), "dev1", "metadata", "labels", "my-environment-name")
	require.Equal(t, resource, patched)
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
                    "image": "{{regex_replace_all('^([^/]+\\.[^/]+/)?(.*)$','{{@}}','myregistry.corp.com/$2')}}"
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
	policy := loadResource[kyverno.ClusterPolicy](t, policyRaw)
	resource := loadUnstructured(t, resourceRaw)
	policyContext := createContext(t, &policy, resource)

	er := testMutate(context.TODO(), nil, nil, policyContext, nil)
	require.Equal(t, 2, len(er.PolicyResponse.Rules))

	patched := er.PatchedResource
	require.NotEqual(t, resource, patched)

	containers, found, err := unstructured.NestedSlice(resource.UnstructuredContent(), "spec", "containers")
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, containers)
	unstructured.SetNestedField(containers[0].(map[string]interface{}), "otherregistry.corp.com/foo/bash:5.0", "image")
	unstructured.SetNestedSlice(resource.UnstructuredContent(), containers, "spec", "containers")
	require.Equal(t, resource, patched)
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
	policy := loadResource[kyverno.ClusterPolicy](t, policyRaw)
	resource := loadUnstructured(t, resourceRaw)
	policyContext := createContext(t, &policy, resource)

	er := testMutate(context.TODO(), nil, nil, policyContext, nil)
	require.Equal(t, 1, len(er.PolicyResponse.Rules))

	patched := er.PatchedResource
	require.NotEqual(t, resource, patched)
	unstructured.SetNestedField(resource.UnstructuredContent(), "test", "metadata", "labels", "my-added-label")
	require.Equal(t, resource, patched)
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

	policyRaw := []byte(`{
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

	policy := loadResource[kyverno.ClusterPolicy](t, []byte(policyRaw))
	resource := loadUnstructured(t, []byte(resourceRaw))
	policyContext := createContext(t, &policy, resource)

	er := testMutate(context.TODO(), nil, nil, policyContext, nil)
	require.Equal(t, 2, len(er.PolicyResponse.Rules))

	patched := er.PatchedResource
	require.NotEqual(t, resource, patched)

	subsetsField, found, err := unstructured.NestedFieldNoCopy(resource.UnstructuredContent(), "subsets")
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, subsetsField)

	subsets, ok := subsetsField.([]interface{})
	require.True(t, ok)
	require.NotNil(t, subsets)

	addressesField, found, err := unstructured.NestedFieldNoCopy(subsets[0].(map[string]interface{}), "addresses")
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, addressesField)

	addresses, ok := addressesField.([]interface{})
	require.True(t, ok)
	require.NotNil(t, addresses)

	addresses = append(addresses, map[string]interface{}{"ip": "192.168.42.172"})
	unstructured.SetNestedSlice(subsets[0].(map[string]interface{}), addresses, "addresses")

	require.Equal(t, resource, patched)
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

	resource, err := kubeutils.BytesToUnstructured(resourceRaw)
	assert.NilError(t, err)

	policyContext, err := NewPolicyContext(
		jp,
		*resource,
		kyverno.Create,
		nil,
		cfg,
	)
	assert.NilError(t, err)
	policyContext = policyContext.WithPolicy(&policy)

	er := testMutate(context.TODO(), nil, nil, policyContext, nil)

	assert.Equal(t, len(er.PolicyResponse.Rules), 1)
	assert.Equal(t, er.PolicyResponse.Rules[0].Status(), engineapi.RuleStatusPass)

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

	resource, err := kubeutils.BytesToUnstructured(resourceRaw)
	assert.NilError(t, err)

	policyContext, err := NewPolicyContext(
		jp,
		*resource,
		kyverno.Create,
		nil,
		cfg,
	)
	assert.NilError(t, err)
	policyContext = policyContext.WithPolicy(&policy)

	er := testMutate(context.TODO(), nil, nil, policyContext, nil)

	assert.Equal(t, len(er.PolicyResponse.Rules), 1)
	assert.Equal(t, er.PolicyResponse.Rules[0].Status(), engineapi.RuleStatusPass)

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

	resource, err := kubeutils.BytesToUnstructured(resourceRaw)
	assert.NilError(t, err)

	policyContext, err := NewPolicyContext(
		jp,
		*resource,
		kyverno.Create,
		nil,
		cfg,
	)
	assert.NilError(t, err)
	policyContext = policyContext.WithPolicy(&policy)

	er := testMutate(context.TODO(), nil, nil, policyContext, nil)

	assert.Equal(t, len(er.PolicyResponse.Rules), 1)
	assert.Equal(t, er.PolicyResponse.Rules[0].Status(), engineapi.RuleStatusPass)

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
	policy := loadResource[kyverno.ClusterPolicy](t, policyRaw)
	resource := loadUnstructured(t, resourceRaw)
	policyContext := createContext(t, &policy, resource)

	er := testMutate(context.TODO(), nil, nil, policyContext, nil)

	assert.Equal(t, len(er.PolicyResponse.Rules), 1)
	assert.Equal(t, er.PolicyResponse.Rules[0].Status(), engineapi.RuleStatusPass)

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

func Test_patchStrategicMerge_descending(t *testing.T) {
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
                "order": "Descending",
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
	policy := loadResource[kyverno.ClusterPolicy](t, policyRaw)
	resource := loadUnstructured(t, resourceRaw)
	policyContext := createContext(t, &policy, resource)

	er := testMutate(context.TODO(), nil, nil, policyContext, nil)

	assert.Equal(t, len(er.PolicyResponse.Rules), 1)
	assert.Equal(t, er.PolicyResponse.Rules[0].Status(), engineapi.RuleStatusPass)

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

func Test_patchStrategicMerge_ascending(t *testing.T) {
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
                "order": "Ascending",
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
	policy := loadResource[kyverno.ClusterPolicy](t, policyRaw)
	resource := loadUnstructured(t, resourceRaw)
	policyContext := createContext(t, &policy, resource)

	er := testMutate(context.TODO(), nil, nil, policyContext, nil)

	assert.Equal(t, len(er.PolicyResponse.Rules), 1)
	assert.Equal(t, er.PolicyResponse.Rules[0].Status(), engineapi.RuleStatusPass)

	containers, _, err := unstructured.NestedSlice(er.PatchedResource.Object, "spec", "containers")
	assert.NilError(t, err)

	for i, c := range containers {
		ctnr := c.(map[string]interface{})
		switch i {
		case 0:
			assert.Equal(t, ctnr["name"], "mongodb-agent")
		case 1:
			assert.Equal(t, ctnr["name"], "nginx3")
		case 3:
			assert.Equal(t, ctnr["name"], "mongod")
		}
	}
}

func Test_mutate_nested_foreach(t *testing.T) {
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
          "name": "replace-dns-suffix",
          "match": {
            "any": [
              {
                "resources": {
                  "kinds": [
                    "Ingress"
                  ]
                }
              }
            ]
          },
          "mutate": {
            "foreach": [
              {
                "list": "request.object.spec.tls",
                "foreach": [
                  {
                    "list": "element.hosts",
                    "patchesJson6902": "- path: /spec/tls/{{elementIndex0}}/hosts/{{elementIndex1}}\n  op: replace\n  value: {{replace_all('{{element}}', '.foo.com', '.newfoo.com')}}"
                  }
                ]
              }
            ]
          }
        }
      ]
    }
  }`)

	resourceRaw := []byte(`{
    "apiVersion": "networking.k8s.io/v1",
    "kind": "Ingress",
    "metadata": {
      "name": "tls-example-ingress"
    },
    "spec": {
      "tls": [
        {
          "hosts": [
            "https-example.foo.com"
          ],
          "secretName": "testsecret-tls"
        },
        {
          "hosts": [
            "https-example2.foo.com"
          ],
          "secretName": "testsecret-tls-2"
        }
      ],
      "rules": [
        {
          "host": "https-example.foo.com",
          "http": {
            "paths": [
              {
                "path": "/",
                "pathType": "Prefix",
                "backend": {
                  "service": {
                    "name": "service1",
                    "port": {
                      "number": 80
                    }
                  }
                }
              }
            ]
          }
        },
        {
          "host": "https-example2.foo.com",
          "http": {
            "paths": [
              {
                "path": "/",
                "pathType": "Prefix",
                "backend": {
                  "service": {
                    "name": "service2",
                    "port": {
                      "number": 80
                    }
                  }
                }
              }
            ]
          }
        }
      ]
    }
  }`)

	expectedRaw := []byte(`{
  "apiVersion": "networking.k8s.io/v1",
  "kind": "Ingress",
  "metadata": {
    "name": "tls-example-ingress"
  },
  "spec": {
    "rules": [
      {
        "host": "https-example.foo.com",
        "http": {
          "paths": [
            {
              "backend": {
                "service": {
                  "name": "service1",
                  "port": {
                    "number": 80
                  }
                }
              },
              "path": "/",
              "pathType": "Prefix"
            }
          ]
        }
      },
      {
        "host": "https-example2.foo.com",
        "http": {
          "paths": [
            {
              "backend": {
                "service": {
                  "name": "service2",
                  "port": {
                    "number": 80
                  }
                }
              },
              "path": "/",
              "pathType": "Prefix"
            }
          ]
        }
      }
    ],
    "tls": [
      {
        "hosts": [
          "https-example.newfoo.com"
        ],
        "secretName": "testsecret-tls"
      },
      {
        "hosts": [
          "https-example2.newfoo.com"
        ],
        "secretName": "testsecret-tls-2"
      }
    ]
  }
}`)
	policy := loadResource[kyverno.ClusterPolicy](t, policyRaw)
	resource := loadUnstructured(t, resourceRaw)
	expected := loadUnstructured(t, expectedRaw)
	policyContext := createContext(t, &policy, resource)

	er := testMutate(context.TODO(), nil, nil, policyContext, nil)
	require.Equal(t, 1, len(er.PolicyResponse.Rules))
	require.Equal(t, engineapi.RuleStatusPass, er.PolicyResponse.Rules[0].Status())

	patched := er.PatchedResource
	require.Equal(t, expected, patched)
}

func Test_mutate_existing_resources(t *testing.T) {
	tests := []struct {
		name           string
		policy         []byte
		trigger        []byte
		targets        [][]byte
		patchedTargets [][]byte
		targetList     string
	}{
		{
			name: "test-labelselector",
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
				                                "namespace": "staging",
                                        "selector": {
                                          "matchLabels": {
                                            "app":"nginx"
                                        }
                                      }
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
			patchedTargets: [][]byte{[]byte(`{
		      "apiVersion": "apps/v1",
		      "kind": "Deployment",
		      "metadata": {
		          "name": "example-A",
		          "namespace": "staging",
		          "labels": {
		              "app": "nginx",
		              "foo": "bar"
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
		},
		{
			name: "test-labelselector-variables",
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
				                                "namespace": "staging",
                                        "selector": {
                                          "matchLabels": {
                                            "parent": "{{ request.object.metadata.name }}"
                                        }
                                      }
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
                    "parent": "dictionary",
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
			patchedTargets: [][]byte{[]byte(`{
		      "apiVersion": "apps/v1",
		      "kind": "Deployment",
		      "metadata": {
		          "name": "example-A",
		          "namespace": "staging",
		          "labels": {
		              "app": "nginx",
                  "parent": "dictionary",
		              "foo": "bar"
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
		},
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
			patchedTargets: [][]byte{[]byte(`{
		      "apiVersion": "apps/v1",
		      "kind": "Deployment",
		      "metadata": {
		          "name": "example-A",
		          "namespace": "staging",
		          "labels": {
		              "app": "nginx",
		              "foo": "bar"
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
			patchedTargets: [][]byte{[]byte(`{
		        "apiVersion": "v1",
		        "data": {
		            "foo": "bar"
		        },
		        "kind": "ConfigMap",
		        "metadata": {
		            "name": "dictionary",
		            "namespace": "staging",
		            "labels": {
		              "foo": "bar"
		          }
		        }
		    }`)},
			targetList: "ComfigMapList",
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
                                "mutateExistingOnPolicyUpdate": false,
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
			patchedTargets: [][]byte{[]byte(`
		{
		  "apiVersion": "v1",
		  "data": {
		      "keytwo": "valuetwo-valueone"
		  },
		  "kind": "ConfigMap",
		  "metadata": {
		      "name": "cmtwo",
		      "namespace": "bar"
		  }
		}
		`)},
			targetList: "ComfigMapList",
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
                            "mutateExistingOnPolicyUpdate": false,
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
			patchedTargets: [][]byte{
				[]byte(`
		      {
		        "apiVersion": "v1",
		        "data": {
		            "key": "valuetwo-valueone"
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
						            "key": "valuethree-valueone"
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
		},
	}

	for _, test := range tests {
		policy := loadResource[kyverno.ClusterPolicy](t, test.policy)
		trigger := loadUnstructured(t, test.trigger)

		var targets []runtime.Object
		var patchedTargets []unstructured.Unstructured
		for i := range test.targets {
			target := loadUnstructured(t, test.targets[i])
			targets = append(targets, &target)
			patchedTargets = append(patchedTargets, loadUnstructured(t, test.patchedTargets[i]))
		}
		policyContext := createContext(t, &policy, trigger)

		scheme := runtime.NewScheme()
		dclient, err := client.NewFakeClient(scheme, map[schema.GroupVersionResource]string{}, targets...)
		require.NoError(t, err)
		dclient.SetDiscovery(client.NewFakeDiscoveryClient(nil))

		er := testMutate(context.TODO(), dclient, registryclient.NewOrDie(), policyContext, nil)

		var actualPatchedTargets []unstructured.Unstructured
		for i := range er.PolicyResponse.Rules {
			rr := er.PolicyResponse.Rules[i]
			require.Equal(t, engineapi.RuleStatusPass, rr.Status())
			p, _, _ := rr.PatchedTarget()
			require.NotNil(t, p)
			actualPatchedTargets = append(actualPatchedTargets, *p)
		}
		require.Equal(t, patchedTargets, actualPatchedTargets)
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

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(policyRaw, &policy)
	if err != nil {
		t.Error(err)
	}

	resourceUnstructured, err := kubeutils.BytesToUnstructured(resourceRaw)
	assert.NilError(t, err)

	policyContext, err := NewPolicyContext(
		jp,
		*resourceUnstructured,
		kyverno.Create,
		nil,
		cfg,
	)
	assert.NilError(t, err)
	policyContext = policyContext.WithPolicy(&policy)

	er := testMutate(context.TODO(), nil, nil, policyContext, nil)
	assert.Equal(t, len(er.PolicyResponse.Rules), 2)

	{
		expectedRaw := []byte(`
    {
      "apiVersion": "v1",
      "kind": "Pod",
      "metadata": {
        "labels": {
          "app": "root",
          "appname": "check-root-user"
        },
        "name": "check-root-user"
      },
      "spec": {
        "containers": [
          {
            "image": "nginxinc/nginx-unprivileged",
            "name": "check-root-user",
            "securityContext": {
              "runAsNonRoot": true
            }
          }
        ]
      }
    }`)

		expected := loadUnstructured(t, expectedRaw)
		require.Equal(t, expected, er.PatchedResource)
	}

	applyOne := kyverno.ApplyOne
	policyContext.Policy().GetSpec().ApplyRules = &applyOne

	er = testMutate(context.TODO(), nil, nil, policyContext, nil)
	assert.Equal(t, len(er.PolicyResponse.Rules), 1)

	{
		expectedRaw := []byte(`
    {
      "apiVersion": "v1",
      "kind": "Pod",
      "metadata": {
        "labels": {
          "app": "root"
        },
        "name": "check-root-user"
      },
      "spec": {
        "containers": [
          {
            "image": "nginxinc/nginx-unprivileged",
            "name": "check-root-user",
            "securityContext": {
              "runAsNonRoot": true
            }
          }
        ]
      }
    }`)

		expected := loadUnstructured(t, expectedRaw)
		require.Equal(t, expected, er.PatchedResource)
	}
}

func Test_SpecialCharacters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		policyRaw   []byte
		documentRaw []byte
		want        []string
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
			want: []string{
				`{"op":"replace","path":"/metadata/labels/retention","value":"days_30"}`,
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
			want: []string{
				`{"op":"replace","path":"/metadata/labels/corp.com~1retention","value":"days_30"}`,
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
			want: []string{
				`{"op":"replace","path":"/metadata/labels/corp-retention","value":"days_30"}`,
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
			want: []string{
				`{"op":"replace","path":"/metadata/labels/deploy-zone","value":"EU-CENTRAL-1"}`,
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
			resource, err := kubeutils.BytesToUnstructured(tt.documentRaw)
			if err != nil {
				t.Fatalf("ConvertToUnstructured() error = %v", err)
			}

			// Create policy context.
			policyContext, err := NewPolicyContext(
				jp,
				*resource,
				kyverno.Create,
				nil,
				cfg,
			)
			assert.NilError(t, err)
			policyContext = policyContext.WithPolicy(&policy)

			// Mutate and make sure that we got the expected amount of rules.
			er := testMutate(context.TODO(), nil, nil, policyContext, nil)
			patches := er.GetPatches()
			assert.Equal(t, len(patches), len(tt.want))
			for i := range patches {
				assert.Equal(t, patches[i].Json(), tt.want[i])
			}
		})
	}
}
