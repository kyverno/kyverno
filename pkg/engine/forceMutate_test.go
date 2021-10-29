package engine

import (
	"encoding/json"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"gotest.tools/assert"
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

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	ctx := context.NewContext()
	err = ctx.AddResource(rawResource)
	assert.NilError(t, err)

	mutatedResource, err := ForceMutate(ctx, policy, *resourceUnstructured)
	assert.NilError(t, err)

	assert.DeepEqual(t, expectedResource, mutatedResource.UnstructuredContent())
}

func Test_ForceMutateSubstituteVarsWithNilContext(t *testing.T) {
	expectedRawResource := []byte(`
	{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "check-root-user",
			"labels": {
				"appname": "placeholderValue"
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

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)

	mutatedResource, err := ForceMutate(nil, policy, *resourceUnstructured)
	assert.NilError(t, err)

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
					"Pod"
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

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	ctx := context.NewContext()
	err = ctx.AddResource(rawResource)
	assert.NilError(t, err)

	mutatedResource, err := ForceMutate(ctx, policy, *resourceUnstructured)
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

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	ctx := context.NewContext()
	err = ctx.AddResource(rawResource)
	assert.NilError(t, err)

	mutatedResource, err := ForceMutate(ctx, policy, *resourceUnstructured)
	assert.NilError(t, err)

	assert.DeepEqual(t, expectedResource, mutatedResource.UnstructuredContent())
}
