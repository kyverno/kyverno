package engine

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var rawPolicy = []byte(`
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
}
`)

var rawResource = []byte(`
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

func Test_ForceMutateSubstituteVars(t *testing.T) {
	expectedRawResource := []byte(`
	{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "check-root-user",
			"labels": {
				"appname": "check-root-user"
			}
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

	var expectedResource interface{}
	assert.NilError(t, json.Unmarshal(expectedRawResource, &expectedResource))

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := kubeutils.BytesToUnstructured(rawResource)
	assert.NilError(t, err)
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	ctx := context.NewContext(jp)
	err = context.AddResource(ctx, rawResource)
	assert.NilError(t, err)

	mutatedResource, err := ForceMutate(ctx, logr.Discard(), &policy, *resourceUnstructured)
	assert.NilError(t, err)

	assert.DeepEqual(t, expectedResource, mutatedResource.UnstructuredContent())
}

func Test_ApplyForEachMutate(t *testing.T) {
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
                        "forEach": [
                            {
                                "patchStrategicMerge": {
                                    "metadata": {
                                        "labels": {
                                            "appname": "{{request.object.metadata.name}}"
                                        }
                                    }
                                },
                                "forEach": [
                                    {
                                        "patchStrategicMerge": {
                                            "metadata": {
                                                "labels": {
                                                    "nestedLabel": "nestedValue"
                                                }
                                            }
                                        }
                                    }
                                ]
                            }
                        ]
                    }
                }
            ]
        }
    }
    `)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := kubeutils.BytesToUnstructured(rawResource)
	assert.NilError(t, err)
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	ctx := context.NewContext(jp)
	err = context.AddResource(ctx, rawResource)
	assert.NilError(t, err)

	mutatedResource, err := ForceMutate(ctx, logr.Discard(), &policy, *resourceUnstructured)
	assert.NilError(t, err)

	expectedRawResource := []byte(`{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": {
			"labels": {
				"nestedLabel": "nestedValue"
			},
			"name": "check-root-user"
		},
		"spec": {"containers": [{"image": "nginxinc/nginx-unprivileged", "name": "check-root-user", "securityContext": {"runAsNonRoot": true}}]}
	}`)

	var expectedResource interface{}
	assert.NilError(t, json.Unmarshal(expectedRawResource, &expectedResource))

	assert.DeepEqual(t, expectedResource, mutatedResource.UnstructuredContent())
}

func Test_ForceMutateSubstituteVarsWithPatchesJson6902(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "insert-container"
		},
		"spec": {
		  "rules": [
			{
			  "name": "insert-container",
			  "match": {
				"resources": {
				  "kinds": [
					"Deployment"
				  ]
				}
			  },
			  "mutate": {
				"patchesJson6902": "- op: add\n  path: \"/spec/template/spec/containers/0/command/0\"\n  value: ls"
			  }
			}
		  ]
		}
	  }
	`)

	rawResource := []byte(`
		{
			"apiVersion": "apps/v1",
			"kind": "Deployment",
			"metadata": {
				"name": "myDeploy"
			},
			"spec": {
				"replica": 2,
				"template": {
				"metadata": {
					"labels": {
					"old-label": "old-value"
					}
				},
				"spec": {
					"containers": [
					{
						"command": ["ll", "rm"],
						"image": "nginx",
						"name": "nginx"
					}
					]
				}
				}
			}
		}
	`)

	rawExpected := []byte(`
	{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
		  "name": "myDeploy"
		},
		"spec": {
		  "replica": 2,
		  "template": {
			"metadata": {
			  "labels": {
				"old-label": "old-value"
			  }
			},
			"spec": {
			  "containers": [
				{
					"command": ["ls", "ll", "rm"],
				  "image": "nginx",
				  "name": "nginx"
				}
			  ]
			}
		  }
		}
	  }
	`)

	var expectedResource unstructured.Unstructured
	assert.NilError(t, json.Unmarshal(rawExpected, &expectedResource))

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := kubeutils.BytesToUnstructured(rawResource)
	assert.NilError(t, err)
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	ctx := context.NewContext(jp)
	err = context.AddResource(ctx, rawResource)
	assert.NilError(t, err)

	mutatedResource, err := ForceMutate(ctx, logr.Discard(), &policy, *resourceUnstructured)
	assert.NilError(t, err)

	assert.DeepEqual(t, expectedResource.UnstructuredContent(), mutatedResource.UnstructuredContent())
}

func Test_ForceMutateSubstituteVarsWithPatchStrategicMerge(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "strategic-merge-patch"
		},
		"spec": {
		  "rules": [
			{
			  "name": "set-image-pull-policy-add-command",
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
						"volumes": [
						  {
							"emptyDir": {
							  "medium": "Memory"
							},
							"name": "cache-volume"
						  }
						]
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
		"volumes": [
			{
				"name": "cache-volume",
				"emptyDir": { }
			  },
			  {
				"name": "cache-volume2",
				"emptyDir": {
				  "medium": "Memory"
				}
			  }
		]
	}
}
`)

	expectedRawResource := []byte(`
	{"apiVersion":"v1","kind":"Pod","metadata":{"name":"check-root-user"},"spec":{"volumes":[{"emptyDir":{"medium":"Memory"},"name":"cache-volume"},{"emptyDir":{"medium":"Memory"},"name":"cache-volume2"}]}}
	  `)

	var expectedResource interface{}
	assert.NilError(t, json.Unmarshal(expectedRawResource, &expectedResource))

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := kubeutils.BytesToUnstructured(rawResource)
	assert.NilError(t, err)
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	ctx := context.NewContext(jp)
	err = context.AddResource(ctx, rawResource)
	assert.NilError(t, err)

	mutatedResource, err := ForceMutate(ctx, logr.Discard(), &policy, *resourceUnstructured)
	assert.NilError(t, err)

	assert.DeepEqual(t, expectedResource, mutatedResource.UnstructuredContent())
}

func Test_ForceMutateAutogenRules(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		anchor  string
		mutates bool
	}{
		{name: "matching anchor", anchor: "*", mutates: true},
		{name: "nonmatching anchor", anchor: "missing", mutates: false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			var policy kyverno.ClusterPolicy
			assert.NilError(t, json.Unmarshal([]byte(fmt.Sprintf(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
			"name": "require-non-root",
			"annotations": {"pod-policies.kyverno.io/autogen-controllers": "Deployment,CronJob"}
		},
		"spec": {"rules": [{
			"name": "set-non-root",
			"match": {"resources": {"kinds": ["Pod"]}},
			"mutate": {"patchStrategicMerge": {
				"spec": {"containers": [{"(name)": %q, "securityContext": {"runAsNonRoot": true}}]}
			}}
		}]}
	}`, scenario.anchor)), &policy))
			originalPolicy := policy.DeepCopy()
			rules := autogen.Default.ComputeRules(&policy, "")
			assert.Equal(t, len(rules), 3)

			for _, tc := range []struct {
				name           string
				ruleName       string
				resource       string
				containersPath []string
			}{
				{
					name:           "Pod",
					ruleName:       "set-non-root",
					resource:       `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"test"},"spec":{"containers":[{"name":"app","image":"nginx"}]}}`,
					containersPath: []string{"spec", "containers"},
				},
				{
					name:           "Deployment",
					ruleName:       "autogen-set-non-root",
					resource:       `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"test"},"spec":{"template":{"spec":{"containers":[{"name":"app","image":"nginx"}]}}}}`,
					containersPath: []string{"spec", "template", "spec", "containers"},
				},
				{
					name:           "CronJob",
					ruleName:       "autogen-cronjob-set-non-root",
					resource:       `{"apiVersion":"batch/v1","kind":"CronJob","metadata":{"name":"test"},"spec":{"schedule":"* * * * *","jobTemplate":{"spec":{"template":{"spec":{"containers":[{"name":"app","image":"nginx"}]}}}}}}`,
					containersPath: []string{"spec", "jobTemplate", "spec", "template", "spec", "containers"},
				},
			} {
				t.Run(tc.name, func(t *testing.T) {
					selectedPolicy := policy.DeepCopy()
					selectedPolicy.Spec.Rules = nil
					for _, rule := range rules {
						if rule.Name == tc.ruleName {
							selectedPolicy.Spec.Rules = append(selectedPolicy.Spec.Rules, *rule.DeepCopy())
						}
					}
					assert.Equal(t, len(selectedPolicy.Spec.Rules), 1)
					originalSelectedPolicy := selectedPolicy.DeepCopy()

					resource, err := kubeutils.BytesToUnstructured([]byte(tc.resource))
					assert.NilError(t, err)
					originalResource := resource.DeepCopy()
					ctx := context.NewContext(jmespath.New(config.NewDefaultConfiguration(false)))
					assert.NilError(t, context.AddResource(ctx, []byte(tc.resource)))

					// ForceMutate deliberately bypasses matching. Give it only the rule
					// intended for this resource, rather than every generated rule.
					mutatedResource, err := ForceMutate(ctx, logr.Discard(), selectedPolicy, *resource)
					assert.NilError(t, err)
					expectedResource := originalResource.DeepCopy()
					if scenario.mutates {
						assert.NilError(t, unstructured.SetNestedSlice(expectedResource.Object, []interface{}{
							map[string]interface{}{
								"name": "app", "image": "nginx",
								"securityContext": map[string]interface{}{"runAsNonRoot": true},
							},
						}, tc.containersPath...))
					}
					assert.DeepEqual(t, mutatedResource.Object, expectedResource.Object)
					if tc.name != "Pod" {
						_, found, err := unstructured.NestedSlice(mutatedResource.Object, "spec", "containers")
						assert.NilError(t, err)
						assert.Equal(t, found, false)
					}
					assert.DeepEqual(t, resource.Object, originalResource.Object)
					assert.DeepEqual(t, selectedPolicy, originalSelectedPolicy)
					assert.DeepEqual(t, &policy, originalPolicy)
				})
			}
		})
	}
}
