package admissionpolicy

import (
	"encoding/json"
	"testing"

	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	utils "github.com/kyverno/kyverno/pkg/utils/restmapper"
	"gotest.tools/assert"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apiserver/pkg/admission"
)

func Test_MutateResource(t *testing.T) {
	tests := []struct {
		name                string
		rawPolicy           []byte
		rawResource         []byte
		expectedRawResource []byte
	}{
		{
			name: "MAP ApplyConfiguration",
			rawPolicy: []byte(`{
    "apiVersion": "admissionregistration.k8s.io/v1alpha1",
    "kind": "MutatingAdmissionPolicy",
    "metadata": {
        "name": "mutate-policy"
    },
    "spec": {
        "matchConstraints": {
            "resourceRules": [
                {
                    "apiGroups": [
                        ""
                    ],
                    "apiVersions": [
                        "v1"
                    ],
                    "operations": [
                        "CREATE"
                    ],
                    "resources": [
                        "configmaps"
                    ]
                }
            ]
        },
        "failurePolicy": "Fail",
        "mutations": [
            {
                "patchType": "ApplyConfiguration",
                "applyConfiguration": {
                    "expression": "object.metadata.?labels[\"lfx-mentorship\"].hasValue() ? \n    Object{} :\n    Object{ metadata: Object.metadata{ labels: {\"lfx-mentorship\": \"kyverno\"}}}\n"
                }
            }
        ]
    }
}`),
			rawResource: []byte(`{
    "apiVersion": "v1",
    "kind": "ConfigMap",
    "metadata": {
        "name": "game-demo",
        "labels": {
            "app": "game"
        }
    },
    "data": {
        "player_initial_lives": "3"
    }
}`),
			expectedRawResource: []byte(`{
    "apiVersion": "v1",
    "kind": "ConfigMap",
    "metadata": {
        "name": "game-demo",
        "labels": {
            "app": "game",
            "lfx-mentorship": "kyverno"
        }
    },
    "data": {
        "player_initial_lives": "3"
    }
}`),
		},
		{
			name: "MAP JSONPatch",
			rawPolicy: []byte(`{
    "apiVersion": "admissionregistration.k8s.io/v1alpha1",
    "kind": "MutatingAdmissionPolicy",
    "metadata": {
        "name": "mutate-policy"
    },
    "spec": {
        "matchConstraints": {
            "resourceRules": [
                {
                    "apiGroups": [
                        "discovery.k8s.io"
                    ],
                    "apiVersions": [
                        "v1"
                    ],
                    "operations": [
                        "CREATE"
                    ],
                    "resources": [
                        "endpointslices"
                    ]
                }
            ]
        },
        "failurePolicy": "Fail",
        "reinvocationPolicy": "Never",
        "mutations": [
            {
                "patchType": "JSONPatch",
                "jsonPatch": {
                    "expression": "[\n  JSONPatch{\n    op: \"add\", path: \"/ports\",\n    value: object.ports.map(\n      p, \n      {\n        \"name\": p.name,\n        \"port\": dyn(p.name.contains(\"secure\") ? 6443 : p.port)\n      }\n    )\n  }\n]\n"
                }
            }
        ]
    }
}`),
			rawResource: []byte(`{
    "apiVersion": "discovery.k8s.io/v1",
    "kind": "EndpointSlice",
    "metadata": {
        "name": "example-abc",
        "labels": {
            "kubernetes.io/service-name": "example"
        }
    },
    "addressType": "IPv4",
    "ports": [
        {
            "name": "http",
            "protocol": "TCP",
            "port": 80
        },
        {
            "name": "secure",
            "protocol": "TCP"
        }
    ],
    "endpoints": [
        {
            "addresses": [
                "10.1.2.3"
            ],
            "conditions": {
                "ready": true
            },
            "hostname": "pod-1",
            "nodeName": "node-1",
            "zone": "us-west2-a"
        }
    ]
}`),
			expectedRawResource: []byte(`{
    "apiVersion": "discovery.k8s.io/v1",
    "kind": "EndpointSlice",
    "metadata": {
        "name": "example-abc",
        "labels": {
            "kubernetes.io/service-name": "example"
        }
    },
    "addressType": "IPv4",
    "ports": [
        {
            "name": "http",
            "port": 80
        },
        {
            "name": "secure",
            "port": 6443
        }
    ],
    "endpoints": [
        {
            "addresses": [
                "10.1.2.3"
            ],
            "conditions": {
                "ready": true
            },
            "hostname": "pod-1",
            "nodeName": "node-1",
            "zone": "us-west2-a"
        }
    ]
}`),
		},
		{
			name: "MAP JSONPatch and ApplyConfiguration",
			rawPolicy: []byte(`{
    "apiVersion": "admissionregistration.k8s.io/v1alpha1",
    "kind": "MutatingAdmissionPolicy",
    "metadata": {
        "name": "sample-policy"
    },
    "spec": {
        "matchConstraints": {
            "resourceRules": [
                {
                    "apiGroups": [
                        "apps"
                    ],
                    "apiVersions": [
                        "v1"
                    ],
                    "operations": [
                        "CREATE"
                    ],
                    "resources": [
                        "deployments"
                    ]
                }
            ]
        },
        "failurePolicy": "Fail",
        "mutations": [
            {
                "patchType": "ApplyConfiguration",
                "applyConfiguration": {
                    "expression": "Object{\n  spec: Object.spec{\n    replicas: object.spec.replicas + 100\n  }\n}\n"
                }
            },
            {
                "patchType": "JSONPatch",
                "jsonPatch": {
                    "expression": "[\n  JSONPatch{\n      op: \"replace\", \n      path: \"/spec/replicas\", \n      value: object.spec.replicas + 10\n  }\n]\n"
                }
            }
        ]
    }
}`),
			rawResource: []byte(`{
    "apiVersion": "apps/v1",
    "kind": "Deployment",
    "metadata": {
        "name": "nginx-deployment",
        "labels": {
            "app": "nginx"
        }
    },
    "spec": {
        "replicas": 3,
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
}`),
			expectedRawResource: []byte(`{
    "apiVersion": "apps/v1",
    "kind": "Deployment",
    "metadata": {
        "name": "nginx-deployment",
        "labels": {
            "app": "nginx"
        }
    },
    "spec": {
        "replicas": 113,
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
}`),
		},
		{
			name: "Two mutations of type ApplyConfigurations",
			rawPolicy: []byte(`{
    "apiVersion": "admissionregistration.k8s.io/v1alpha1",
    "kind": "MutatingAdmissionPolicy",
    "metadata": {
        "name": "sample-policy"
    },
    "spec": {
        "matchConstraints": {
            "resourceRules": [
                {
                    "apiGroups": [
                        "apps"
                    ],
                    "apiVersions": [
                        "v1"
                    ],
                    "operations": [
                        "CREATE"
                    ],
                    "resources": [
                        "deployments"
                    ]
                }
            ]
        },
        "failurePolicy": "Fail",
        "mutations": [
            {
                "patchType": "ApplyConfiguration",
                "applyConfiguration": {
                    "expression": "Object{\n  spec: Object.spec{\n    replicas: object.spec.replicas + 100\n  }\n}\n"
                }
            },
            {
                "patchType": "ApplyConfiguration",
                "applyConfiguration": {
                    "expression": "Object{\n  spec: Object.spec{\n    replicas: object.spec.replicas + 100\n  }\n}\n"
                }
            }
        ]
    }
}`),
			rawResource: []byte(`{
    "apiVersion": "apps/v1",
    "kind": "Deployment",
    "metadata": {
        "name": "nginx-deployment",
        "labels": {
            "app": "nginx"
        }
    },
    "spec": {
        "replicas": 3,
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
}`),
			expectedRawResource: []byte(`{
    "apiVersion": "apps/v1",
    "kind": "Deployment",
    "metadata": {
        "name": "nginx-deployment",
        "labels": {
            "app": "nginx"
        }
    },
    "spec": {
        "replicas": 203,
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
}`),
		},
		{
			name: "Two mutations of type JSONPatch",
			rawPolicy: []byte(`{
    "apiVersion": "admissionregistration.k8s.io/v1alpha1",
    "kind": "MutatingAdmissionPolicy",
    "metadata": {
        "name": "sample-policy"
    },
    "spec": {
        "matchConstraints": {
            "resourceRules": [
                {
                    "apiGroups": [
                        "apps"
                    ],
                    "apiVersions": [
                        "v1"
                    ],
                    "operations": [
                        "CREATE"
                    ],
                    "resources": [
                        "deployments"
                    ]
                }
            ]
        },
        "failurePolicy": "Fail",
        "mutations": [
            {
                "patchType": "JSONPatch",
                "jsonPatch": {
                    "expression": "[\n  JSONPatch{\n      op: \"replace\", \n      path: \"/spec/replicas\", \n      value: object.spec.replicas + 10\n  }\n]\n"
                }
            },
            {
                "patchType": "JSONPatch",
                "jsonPatch": {
                    "expression": "[\n  JSONPatch{\n      op: \"replace\", \n      path: \"/spec/replicas\", \n      value: object.spec.replicas + 10\n  }\n]\n"
                }
            }
        ]
    }
}`),
			rawResource: []byte(`{
    "apiVersion": "apps/v1",
    "kind": "Deployment",
    "metadata": {
        "name": "nginx-deployment",
        "labels": {
            "app": "nginx"
        }
    },
    "spec": {
        "replicas": 3,
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
}`),
			expectedRawResource: []byte(`{
    "apiVersion": "apps/v1",
    "kind": "Deployment",
    "metadata": {
        "name": "nginx-deployment",
        "labels": {
            "app": "nginx"
        }
    },
    "spec": {
        "replicas": 23,
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
}`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedResource, err := kubeutils.BytesToUnstructured(tt.expectedRawResource)
			assert.NilError(t, err)

			var policy admissionregistrationv1alpha1.MutatingAdmissionPolicy
			err = json.Unmarshal(tt.rawPolicy, &policy)
			assert.NilError(t, err)

			resource, err := kubeutils.BytesToUnstructured(tt.rawResource)
			assert.NilError(t, err)

			gvk := resource.GroupVersionKind()

			restMapper, err := utils.GetRESTMapper(nil, false)
			assert.NilError(t, err)

			mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			assert.NilError(t, err)

			gvr := mapping.Resource
			a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, gvk, resource.GetNamespace(), resource.GetName(), gvr, "", admission.Create, nil, false, nil)
			response, err := mutateResource(&policy, nil, *resource, gvr, nil, a, false)
			assert.NilError(t, err)

			assert.DeepEqual(t, expectedResource.Object, response.PatchedResource.Object)
		})
	}
}

func Test_MutateResourceWithBackgroundScanEnabled(t *testing.T) {
	tests := []struct {
		name        string
		rawPolicy   []byte
		rawResource []byte
		result      engineapi.RuleStatus
	}{
		{
			name: "Existing resource that was not mutated",
			rawPolicy: []byte(`{
    "apiVersion": "admissionregistration.k8s.io/v1alpha1",
    "kind": "MutatingAdmissionPolicy",
    "metadata": {
        "name": "mutate-policy"
    },
    "spec": {
        "matchConstraints": {
            "resourceRules": [
                {
                    "apiGroups": [
                        ""
                    ],
                    "apiVersions": [
                        "v1"
                    ],
                    "operations": [
                        "CREATE"
                    ],
                    "resources": [
                        "configmaps"
                    ]
                }
            ]
        },
        "failurePolicy": "Fail",
        "mutations": [
            {
                "patchType": "ApplyConfiguration",
                "applyConfiguration": {
                    "expression": "object.metadata.?labels[\"lfx-mentorship\"].hasValue() ? \n    Object{} :\n    Object{ metadata: Object.metadata{ labels: {\"lfx-mentorship\": \"kyverno\"}}}\n"
                }
            }
        ]
    }
}`),
			rawResource: []byte(`{
    "apiVersion": "v1",
    "kind": "ConfigMap",
    "metadata": {
        "name": "game-demo",
        "labels": {
            "app": "game"
        }
    },
    "data": {
        "player_initial_lives": "3"
    }
}`),
			result: engineapi.RuleStatusFail,
		},
		{
			name: "resource that was mutated",
			rawPolicy: []byte(`{
                "apiVersion": "admissionregistration.k8s.io/v1alpha1",
                "kind": "MutatingAdmissionPolicy",
                "metadata": {
                    "name": "mutate-policy"
                },
                "spec": {
                    "matchConstraints": {
                        "resourceRules": [
                            {
                                "apiGroups": [
                                    "discovery.k8s.io"
                                ],
                                "apiVersions": [
                                    "v1"
                                ],
                                "operations": [
                                    "CREATE"
                                ],
                                "resources": [
                                    "endpointslices"
                                ]
                            }
                        ]
                    },
                    "failurePolicy": "Fail",
                    "reinvocationPolicy": "Never",
                    "mutations": [
                        {
                            "patchType": "JSONPatch",
                            "jsonPatch": {
                                "expression": "[\n  JSONPatch{\n    op: \"add\", path: \"/ports\",\n    value: object.ports.map(\n      p, \n      {\n        \"name\": p.name,\n        \"port\": dyn(p.name.contains(\"secure\") ? 6443 : p.port)\n      }\n    )\n  }\n]\n"
                            }
                        }
                    ]
                }
            }`),
			rawResource: []byte(`{
                "apiVersion": "discovery.k8s.io/v1",
                "kind": "EndpointSlice",
                "metadata": {
                    "name": "example-abc",
                    "labels": {
                        "kubernetes.io/service-name": "example"
                    }
                },
                "addressType": "IPv4",
                "ports": [
                    {
                        "name": "http",
                        "port": 80
                    },
                    {
                        "name": "secure",
                        "port": 6443
                    }
                ],
                "endpoints": [
                    {
                        "addresses": [
                            "10.1.2.3"
                        ],
                        "conditions": {
                            "ready": true
                        },
                        "hostname": "pod-1",
                        "nodeName": "node-1",
                        "zone": "us-west2-a"
                    }
                ]
            }`),
			result: engineapi.RuleStatusPass,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var policy admissionregistrationv1alpha1.MutatingAdmissionPolicy
			err := json.Unmarshal(tt.rawPolicy, &policy)
			assert.NilError(t, err)

			resource, err := kubeutils.BytesToUnstructured(tt.rawResource)
			assert.NilError(t, err)

			gvk := resource.GroupVersionKind()

			restMapper, err := utils.GetRESTMapper(nil, false)
			assert.NilError(t, err)

			mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			assert.NilError(t, err)

			gvr := mapping.Resource
			a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, gvk, resource.GetNamespace(), resource.GetName(), gvr, "", admission.Create, nil, false, nil)
			response, err := mutateResource(&policy, nil, *resource, gvr, nil, a, true)
			assert.NilError(t, err)

			assert.Equal(t, len(response.PolicyResponse.Rules), 1)
			assert.Equal(t, response.PolicyResponse.Rules[0].Status(), tt.result)
		})
	}
}
