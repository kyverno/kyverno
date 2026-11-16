package admissionpolicy

import (
	"encoding/json"
	"testing"

	"gotest.tools/v3/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"

	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	utils "github.com/kyverno/kyverno/pkg/utils/restmapper"
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

			var policy admissionregistrationv1beta1.MutatingAdmissionPolicy
			err = json.Unmarshal(tt.rawPolicy, &policy)
			assert.NilError(t, err)

			resource, err := kubeutils.BytesToUnstructured(tt.rawResource)
			assert.NilError(t, err)

			gvk := resource.GroupVersionKind()

			restMapper, err := utils.GetRESTMapper(nil)
			assert.NilError(t, err)

			mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			assert.NilError(t, err)

			gvr := mapping.Resource
			a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, gvk, resource.GetNamespace(), resource.GetName(), gvr, "", admission.Create, nil, false, nil)
			response, err := mutateResource(&policy, nil, *resource, nil, gvr, nil, a, false)
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
			var policy admissionregistrationv1beta1.MutatingAdmissionPolicy
			err := json.Unmarshal(tt.rawPolicy, &policy)
			assert.NilError(t, err)

			resource, err := kubeutils.BytesToUnstructured(tt.rawResource)
			assert.NilError(t, err)

			gvk := resource.GroupVersionKind()

			restMapper, err := utils.GetRESTMapper(nil)
			assert.NilError(t, err)

			mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			assert.NilError(t, err)

			gvr := mapping.Resource
			a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, gvk, resource.GetNamespace(), resource.GetName(), gvr, "", admission.Create, nil, false, nil)
			response, err := mutateResource(&policy, nil, *resource, nil, gvr, nil, a, true)
			assert.NilError(t, err)

			assert.Equal(t, len(response.PolicyResponse.Rules), 1)
			assert.Equal(t, response.PolicyResponse.Rules[0].Status(), tt.result)
		})
	}
}

func Test_MutateResourceNilMatchConstraints(t *testing.T) {
	rawPolicy := []byte(`{
    "apiVersion": "admissionregistration.k8s.io/v1alpha1",
    "kind": "MutatingAdmissionPolicy",
    "metadata": {
        "name": "mutate-policy"
    },
    "spec": {
        "failurePolicy": "Fail",
        "mutations": [
            {
                "patchType": "ApplyConfiguration",
                "applyConfiguration": {
                    "expression": "object"
                }
            }
        ]
    }
}`)
	rawResource := []byte(`{
    "apiVersion": "v1",
    "kind": "ConfigMap",
    "metadata": {
        "name": "game-demo",
        "namespace": "default"
    },
    "data": {
        "player_initial_lives": "3"
    }
}`)

	var policy admissionregistrationv1beta1.MutatingAdmissionPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resource, err := kubeutils.BytesToUnstructured(rawResource)
	assert.NilError(t, err)

	gvk := resource.GroupVersionKind()
	restMapper, err := utils.GetRESTMapper(nil)
	assert.NilError(t, err)

	mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	assert.NilError(t, err)

	gvr := mapping.Resource
	a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, gvk, resource.GetNamespace(), resource.GetName(), gvr, "", admission.Create, nil, false, nil)
	_, err = mutateResource(&policy, nil, *resource, nil, gvr, nil, a, false)
	assert.NilError(t, err)
}

func Test_MatchesSelector_ConjunctionScoping(t *testing.T) {
	tests := []struct {
		name            string
		obj             *unstructured.Unstructured
		paramRef        *admissionregistrationv1.ParamRef
		targetNamespace string
		expected        bool
	}{
		{
			name: "exact namespace and name match",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "config-a",
						"namespace": "tenant-1",
					},
				},
			},
			paramRef: &admissionregistrationv1.ParamRef{
				Name:      "config-a",
				Namespace: "tenant-1",
			},
			expected: true,
		},
		{
			name: "cross-namespace name match rejected",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "config-a",
						"namespace": "tenant-2",
					},
				},
			},
			paramRef: &admissionregistrationv1.ParamRef{
				Name:      "config-a",
				Namespace: "tenant-1",
			},
			expected: false,
		},
		{
			name: "cross-namespace selector match rejected",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "config-b",
						"namespace": "tenant-2",
						"labels": map[string]interface{}{
							"tier": "gold",
						},
					},
				},
			},
			paramRef: &admissionregistrationv1.ParamRef{
				Namespace: "tenant-1",
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"tier": "gold",
					},
				},
			},
			expected: false,
		},
		{
			name: "same namespace selector match accepted",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "config-b",
						"namespace": "tenant-1",
						"labels": map[string]interface{}{
							"tier": "gold",
						},
					},
				},
			},
			paramRef: &admissionregistrationv1.ParamRef{
				Namespace: "tenant-1",
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"tier": "gold",
					},
				},
			},
			expected: true,
		},
		{
			name:     "nil ref rejected",
			obj:      &unstructured.Unstructured{},
			paramRef: nil,
			expected: false,
		},
		{
			name: "cluster-scoped target rejects namespaced candidate with empty ref namespace",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "config-a",
						"namespace": "tenant-1",
					},
				},
			},
			paramRef: &admissionregistrationv1.ParamRef{
				Name: "config-a",
			},
			targetNamespace: "",
			expected:        false,
		},
		{
			name: "namespaced target accepts namespaced candidate with empty ref namespace",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "config-a",
						"namespace": "tenant-1",
					},
				},
			},
			paramRef: &admissionregistrationv1.ParamRef{
				Name: "config-a",
			},
			targetNamespace: "tenant-1",
			expected:        true,
		},
		{
			name: "namespaced target rejects candidate from different namespace with empty ref namespace",
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "config-a",
						"namespace": "tenant-2",
					},
				},
			},
			paramRef: &admissionregistrationv1.ParamRef{
				Name: "config-a",
			},
			targetNamespace: "tenant-1",
			expected:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesSelector(tt.obj, tt.paramRef, tt.targetNamespace)
			assert.Equal(t, got, tt.expected)
		})
	}
}

func Test_MutateClusterScopedResource_NilNamespace(t *testing.T) {
	rawPolicy := []byte(`{
    "apiVersion": "admissionregistration.k8s.io/v1alpha1",
    "kind": "MutatingAdmissionPolicy",
    "metadata": {
        "name": "mutate-ns"
    },
    "spec": {
        "failurePolicy": "Fail",
        "matchConstraints": {
            "resourceRules": [
                {
                    "apiGroups": [""],
                    "apiVersions": ["v1"],
                    "operations": ["CREATE"],
                    "resources": ["namespaces"]
                }
            ]
        },
        "mutations": [
            {
                "patchType": "ApplyConfiguration",
                "applyConfiguration": {
                    "expression": "object"
                }
            }
        ]
    }
}`)
	rawResource := []byte(`{
    "apiVersion": "v1",
    "kind": "Namespace",
    "metadata": {
        "name": "prod"
    }
}`)

	var policy admissionregistrationv1beta1.MutatingAdmissionPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resource, err := kubeutils.BytesToUnstructured(rawResource)
	assert.NilError(t, err)

	data := engineapi.NewMutatingAdmissionPolicyData(&policy)
	gvk := resource.GroupVersionKind()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}

	res, err := Mutate(data, *resource, gvk, gvr, nil, nil, nil, true, false)
	assert.NilError(t, err)
	assert.Equal(t, res.Resource.GetName(), "prod")
}

func Test_ProcessMAPWithoutClient_MultipleParams_Rejects(t *testing.T) {
	rawPolicy := []byte(`{
    "apiVersion": "admissionregistration.k8s.io/v1alpha1",
    "kind": "MutatingAdmissionPolicy",
    "metadata": {
        "name": "mutate-with-params"
    },
    "spec": {
        "failurePolicy": "Fail",
        "paramKind": {
            "apiVersion": "v1",
            "kind": "ConfigMap"
        },
        "matchConstraints": {
            "resourceRules": [
                {
                    "apiGroups": [""],
                    "apiVersions": ["v1"],
                    "operations": ["CREATE"],
                    "resources": ["configmaps"]
                }
            ]
        },
        "mutations": [
            {
                "patchType": "ApplyConfiguration",
                "applyConfiguration": {
                    "expression": "object"
                }
            }
        ]
    }
}`)
	rawResource := []byte(`{
    "apiVersion": "v1",
    "kind": "ConfigMap",
    "metadata": {
        "name": "target-cm",
        "namespace": "default"
    }
}`)

	var policy admissionregistrationv1beta1.MutatingAdmissionPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resource, err := kubeutils.BytesToUnstructured(rawResource)
	assert.NilError(t, err)

	gvk := resource.GroupVersionKind()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, gvk, resource.GetNamespace(), resource.GetName(), gvr, "", admission.Create, nil, false, nil)
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}

	bindings := []admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "binding-named"},
			Spec: admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{
				PolicyName: "mutate-with-params",
				ParamRef: &admissionregistrationv1beta1.ParamRef{
					Namespace: "default",
					Name:      "cm-1",
				},
			},
		},
	}

	cm1, err := kubeutils.BytesToUnstructured([]byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"cm-1","namespace":"default"}}`))
	assert.NilError(t, err)
	cm2, err := kubeutils.BytesToUnstructured([]byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"cm-1","namespace":"default"}}`))
	assert.NilError(t, err)

	// MAP evaluations reject multiple parameter matches without a selector
	_, err = processMAPWithoutClient(&policy, bindings, *resource, ns, []runtime.Object{cm1, cm2}, gvr, a, "default/ConfigMap/target-cm", false)
	assert.ErrorContains(t, err, "multiple parameter resources match paramRef without selector")
}

func Test_ProcessMAPWithoutClient_ChainedMutation(t *testing.T) {
	rawPolicy := []byte(`{
    "apiVersion": "admissionregistration.k8s.io/v1alpha1",
    "kind": "MutatingAdmissionPolicy",
    "metadata": {
        "name": "chained-mutation-policy"
    },
    "spec": {
        "failurePolicy": "Fail",
        "matchConstraints": {
            "resourceRules": [
                {
                    "apiGroups": [""],
                    "apiVersions": ["v1"],
                    "operations": ["CREATE"],
                    "resources": ["configmaps"]
                }
            ]
        },
        "mutations": [
            {
                "patchType": "ApplyConfiguration",
                "applyConfiguration": {
                    "expression": "has(object.metadata.labels) && 'step1' in object.metadata.labels ? Object{metadata: Object.metadata{labels: {'step2': 'applied'}}} : Object{metadata: Object.metadata{labels: {'step1': 'applied'}}}"
                }
            }
        ]
    }
}`)
	var policy admissionregistrationv1beta1.MutatingAdmissionPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	rawResource := []byte(`{
    "apiVersion": "v1",
    "kind": "ConfigMap",
    "metadata": {
        "name": "target-cm",
        "namespace": "default"
    }
}`)
	resource, err := kubeutils.BytesToUnstructured(rawResource)
	assert.NilError(t, err)

	gvk := resource.GroupVersionKind()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, gvk, resource.GetNamespace(), resource.GetName(), gvr, "", admission.Create, nil, false, nil)
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}

	bindings := []admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "binding-1"},
			Spec: admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{
				PolicyName: "chained-mutation-policy",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "binding-2"},
			Spec: admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{
				PolicyName: "chained-mutation-policy",
			},
		},
	}

	er, err := processMAPWithoutClient(&policy, bindings, *resource, ns, nil, gvr, a, "default/ConfigMap/target-cm", false)
	assert.NilError(t, err)
	assert.Equal(t, len(er.PolicyResponse.Rules), 2)
	assert.Equal(t, er.PolicyResponse.Rules[0].MutatingAdmissionPolicyBinding().GetName(), "binding-1")
	assert.Equal(t, er.PolicyResponse.Rules[1].MutatingAdmissionPolicyBinding().GetName(), "binding-2")
	assert.Assert(t, er.PatchedResource.Object != nil)
	assert.Equal(t, er.PatchedResource.GetLabels()["step1"], "applied")
	assert.Equal(t, er.PatchedResource.GetLabels()["step2"], "applied")
	patches := er.GetPatches()
	assert.Assert(t, len(patches) > 0)
}

func Test_ProcessMAPWithoutClient_ClusterScopedTarget_NamespacedParamWithoutNamespace_Fails(t *testing.T) {
	rawPolicy := []byte(`{
    "apiVersion": "admissionregistration.k8s.io/v1beta1",
    "kind": "MutatingAdmissionPolicy",
    "metadata": {
        "name": "cluster-target-map-policy"
    },
    "spec": {
        "failurePolicy": "Fail",
        "paramKind": {
            "apiVersion": "v1",
            "kind": "ConfigMap"
        },
        "matchConstraints": {
            "resourceRules": [
                {
                    "apiGroups": [""],
                    "apiVersions": ["v1"],
                    "operations": ["CREATE"],
                    "resources": ["namespaces"]
                }
            ]
        },
        "mutations": [
            {
                "patchType": "ApplyConfiguration",
                "applyConfiguration": {
                    "expression": "Object{metadata: Object.metadata{labels: {'mutated': 'true'}}}"
                }
            }
        ]
    }
}`)
	rawResource := []byte(`{
    "apiVersion": "v1",
    "kind": "Namespace",
    "metadata": {
        "name": "target-ns"
    }
}`)

	var policy admissionregistrationv1beta1.MutatingAdmissionPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resource, err := kubeutils.BytesToUnstructured(rawResource)
	assert.NilError(t, err)

	gvk := resource.GroupVersionKind()
	gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	a := admission.NewAttributesRecord(resource.DeepCopyObject(), nil, gvk, "", resource.GetName(), gvr, "", admission.Create, nil, false, nil)

	bindings := []admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "binding-cluster-target"},
			Spec: admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{
				PolicyName: "cluster-target-map-policy",
				ParamRef: &admissionregistrationv1beta1.ParamRef{
					Name: "config-a",
				},
			},
		},
	}

	cm, err := kubeutils.BytesToUnstructured([]byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"config-a","namespace":"tenant-1"}}`))
	assert.NilError(t, err)

	_, err = processMAPWithoutClient(&policy, bindings, *resource, nil, []runtime.Object{cm}, gvr, a, "target-ns", false)
	assert.ErrorContains(t, err, "can't use namespaced paramRef to match cluster-scoped resources")
}
