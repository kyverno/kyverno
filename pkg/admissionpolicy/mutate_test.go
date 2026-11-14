package admissionpolicy

import (
	"encoding/json"
	"testing"

	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	utils "github.com/kyverno/kyverno/pkg/utils/restmapper"
	"gotest.tools/v3/assert"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func TestMutate_ProcessMAPWithoutClient_MatchConstraints(t *testing.T) {
	cmResource, err := kubeutils.BytesToUnstructured([]byte(`{
		"apiVersion": "v1",
		"kind": "ConfigMap",
		"metadata": {
			"name": "my-config",
			"namespace": "default"
		},
		"data": {
			"key": "value"
		}
	}`))
	assert.NilError(t, err)

	deployResource, err := kubeutils.BytesToUnstructured([]byte(`{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
			"name": "my-deploy",
			"namespace": "default"
		},
		"spec": {
			"replicas": 3
		}
	}`))
	assert.NilError(t, err)

	validPolicyRaw := []byte(`{
		"apiVersion": "admissionregistration.k8s.io/v1alpha1",
		"kind": "MutatingAdmissionPolicy",
		"metadata": {"name": "sample-mutate-policy"},
		"spec": {
			"matchConstraints": {
				"resourceRules": [
					{
						"apiGroups": ["apps"],
						"apiVersions": ["v1"],
						"operations": ["CREATE"],
						"resources": ["deployments"]
					}
				]
			},
			"failurePolicy": "Fail",
			"mutations": [
				{
					"patchType": "ApplyConfiguration",
					"applyConfiguration": {
						"expression": "Object{ metadata: Object.metadata{ labels: {\"injected\": \"true\"}}}"
					}
				}
			]
		}
	}`)
	var validPolicy admissionregistrationv1beta1.MutatingAdmissionPolicy
	assert.NilError(t, json.Unmarshal(validPolicyRaw, &validPolicy))

	invalidSelectorPolicyRaw := []byte(`{
		"apiVersion": "admissionregistration.k8s.io/v1alpha1",
		"kind": "MutatingAdmissionPolicy",
		"metadata": {"name": "invalid-selector-mutate-policy"},
		"spec": {
			"matchConstraints": {
				"resourceRules": [
					{
						"apiGroups": ["apps"],
						"apiVersions": ["v1"],
						"operations": ["CREATE"],
						"resources": ["deployments"]
					}
				]
			},
			"failurePolicy": "Fail",
			"mutations": [
				{
					"patchType": "ApplyConfiguration",
					"applyConfiguration": {
						"expression": "Object{ metadata: Object.metadata{ labels: {\"injected\": \"true\"}}}"
					}
				}
			]
		}
	}`)
	var invalidSelectorPolicy admissionregistrationv1beta1.MutatingAdmissionPolicy
	assert.NilError(t, json.Unmarshal(invalidSelectorPolicyRaw, &invalidSelectorPolicy))
	invalidSelectorPolicy.Spec.MatchConstraints.ObjectSelector = &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "tier",
				Operator: "InvalidOperator",
				Values:   []string{"frontend"},
			},
		},
	}

	tests := []struct {
		name           string
		policy         *admissionregistrationv1beta1.MutatingAdmissionPolicy
		bindingName    string
		resource       *unstructured.Unstructured
		gvr            schema.GroupVersionResource
		wantErr        bool
		expectedRules  int
		expectedStatus engineapi.RuleStatus
	}{
		{
			name:          "non-matching resource kind is ignored",
			policy:        &validPolicy,
			bindingName:   "sample-mutate-binding",
			resource:      cmResource,
			gvr:           schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
			wantErr:       false,
			expectedRules: 0,
		},
		{
			name:           "matching resource kind is evaluated",
			policy:         &validPolicy,
			bindingName:    "sample-mutate-binding",
			resource:       deployResource,
			gvr:            schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			wantErr:        false,
			expectedRules:  1,
			expectedStatus: engineapi.RuleStatusPass,
		},
		{
			name:          "invalid label selector in matchConstraints returns matcher error",
			policy:        &invalidSelectorPolicy,
			bindingName:   "invalid-selector-binding",
			resource:      deployResource,
			gvr:           schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
			wantErr:       true,
			expectedRules: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binding := admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{Name: tt.bindingName},
				Spec: admissionregistrationv1beta1.MutatingAdmissionPolicyBindingSpec{
					PolicyName: tt.policy.Name,
				},
			}
			policyData := engineapi.NewMutatingAdmissionPolicyData(tt.policy)
			policyData.AddBinding(binding)

			resp, err := Mutate(policyData, *tt.resource, tt.resource.GroupVersionKind(), tt.gvr, map[string]map[string]string{}, nil, nil, true, false)
			if tt.wantErr {
				assert.Assert(t, err != nil, "expected non-nil error from matcher failure")
				assert.Equal(t, len(resp.PolicyResponse.Rules), 0, "expected 0 evaluated rules on matcher error")
			} else {
				assert.NilError(t, err)
				assert.Equal(t, len(resp.PolicyResponse.Rules), tt.expectedRules)
				if tt.expectedRules > 0 {
					assert.Equal(t, resp.PolicyResponse.Rules[0].Status(), tt.expectedStatus)
				}
			}
		})
	}
}
