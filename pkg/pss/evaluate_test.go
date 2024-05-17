package pss

import (
	"encoding/json"
	"fmt"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
)

func Test_EvaluatePod(t *testing.T) {
	testCases := []testCase{}
	tests := [][]testCase{
		baseline_hostProcess,
		baseline_host_namespaces,
		baseline_privileged,
		baseline_capabilities,
		baseline_hostPath_volumes,
		baseline_host_ports,
		baseline_appArmor,
		baseline_seLinux,
		baseline_procMount,
		baseline_seccompProfile,
		baseline_sysctls,
		restricted_volume_types,
		restricted_privilege_escalation,
		restricted_runAsNonRoot,
		restricted_runAsUser,
		restricted_seccompProfile,
		restricted_capabilities,
		wildcard_images,
	}

	for _, test := range tests {
		testCases = append(testCases, test...)
	}

	for _, test := range testCases {
		var pod corev1.Pod
		err := json.Unmarshal(test.rawPod, &pod)
		assert.NilError(t, err)

		var rule kyvernov1.PodSecurity
		err = json.Unmarshal(test.rawRule, &rule)
		assert.NilError(t, err)

		levelVersion, err := ParseVersion(rule.Level, rule.Version)
		assert.Assert(t, err == nil)

		allowed, checkResults := EvaluatePod(levelVersion, rule.Exclude, &pod)
		if allowed != test.allowed {
			for _, result := range checkResults {
				fmt.Printf("failed check result: %v\n", result)
			}
		}
		assert.Assert(t, allowed == test.allowed, fmt.Sprintf("test \"%s\" fails", test.name))
	}
}

var baseline_hostProcess = []testCase{
	{
		name: "baseline_hostProcess_defines_all_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "HostProcess",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"windowsOptions": {
						"hostProcess": false
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"windowsOptions": {
								"hostProcess": true
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_hostProcess_defines_all_violate_false",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "HostProcess",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"windowsOptions": {
						"hostProcess": false
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"windowsOptions": {
								"hostProcess": false
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_hostProcess_defines_container_only_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "HostProcess",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"windowsOptions": {
								"hostProcess": true
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_hostProcess_defines_initcontainer_only_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "HostProcess",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"initContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"windowsOptions": {
								"hostProcess": true
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_hostProcess_defines_ephemeralcontainer_only_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "HostProcess",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"ephemeralContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"windowsOptions": {
								"hostProcess": true
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_hostProcess_defines_container_only_violate_false",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "HostProcess",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"windowsOptions": {
								"hostProcess": false
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_hostProcess_defines_initContainer_&_ephemeralContainer_allowed_positive",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "HostProcess",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.windowsOptions.hostProcess",
					"values": [
						"true"
					]
				},
				{
					"controlName": "HostProcess",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.initContainers[*].securityContext.windowsOptions.hostProcess",
					"values": [
						"true"
					]
				},
				{
					"controlName": "HostProcess",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.ephemeralContainers[*].securityContext.windowsOptions.hostProcess",
					"values": [
						"true"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"windowsOptions": {
								"hostProcess": true
							}
						}
					}
				],
				"initContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"windowsOptions": {
								"hostProcess": true
							}
						}
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"windowsOptions": {
								"hostProcess": true
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_hostProcess_defines_initContainer_&_ephemeralContainer_allowed_negative",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "HostProcess",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.windowsOptions.hostProcess",
					"values": ["true"]
				},
				{
					"controlName": "HostProcess",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.initContainers[*].securityContext.windowsOptions.hostProcess",
					"values": ["true"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"windowsOptions": {
								"hostProcess": true
							}
						}
					}
				],
				"initContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"windowsOptions": {
								"hostProcess": true
							}
						}
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"windowsOptions": {
								"hostProcess": true
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_hostProcess_defines_spec_only_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "HostProcess"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"windowsOptions": {
						"hostProcess": true
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx:1.2.3"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_hostProcess_defines_spec_blocked_with_no_exclusion",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24"
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"windowsOptions": {
						"hostProcess": true
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx:1.2.3"
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_hostProcess_defines_spec_only_violate_false",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "HostProcess"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"windowsOptions": {
						"hostProcess": false
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx:1.2.3"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_hostProcess_defines_none",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "HostProcess"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx:1.2.3"
					}
				]
			}
		}`),
		allowed: true,
	},
}

var baseline_host_namespaces = []testCase{
	{
		name: "baseline_host_namespaces_hostNetwork_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Host Namespaces"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"hostNetwork": true,
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_host_namespaces_hostNetwork_violate_false",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Host Namespaces"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"hostNetwork": false,
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_host_namespaces_hostNetwork_undefined",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Host Namespaces"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_host_namespaces_hostPID_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Host Namespaces"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"hostPID": true,
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_host_namespaces_hostPID_violate_false",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Host Namespaces"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"hostPID": false,
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_host_namespaces_hostPID_undefined",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Host Namespaces"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_host_namespaces_hostIPC_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Host Namespaces"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"hostIPC": true,
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_host_namespaces_hostIPC_violate_false",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Host Namespaces",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"hostIPC": false,
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_host_namespaces_hostIPC_undefined",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Host Namespaces",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
}

var baseline_privileged = []testCase{
	{
		name: "baseline_privileged_defines_container_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privileged Containers",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"privileged": true
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_privileged_defines_container_violate_false",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privileged Containers",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"privileged": false
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_privileged_defines_container_none",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privileged Containers",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_privileged_defines_container_violate_true_skip",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privileged Containers",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx:1.2.3",
						"securityContext": {
							"privileged": true
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_privileged_defines_initContainer_&_ephemeralContainer_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privileged Containers",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"privileged": false
						}
					}
				],
				"initContainers": [
					{
						"name": "nginx-init",
						"image": "nginx",
						"securityContext": {
							"privileged": true
						}
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx-ephemeral",
						"image": "nginx",
						"securityContext": {
							"privileged": true
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_privileged_defines_initContainer_&_ephemeralContainer_violate_true_allowed_positive",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privileged Containers",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.initContainers[*].securityContext.privileged",
					"values": [
						"true"
					]
				},
				{
					"controlName": "Privileged Containers",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.ephemeralContainers[*].securityContext.privileged",
					"values": [
						"true"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"privileged": false
						}
					}
				],
				"initContainers": [
					{
						"name": "nginx-init",
						"image": "nginx",
						"securityContext": {
							"privileged": true
						}
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx-ephemeral",
						"image": "nginx",
						"securityContext": {
							"privileged": true
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_privileged_defines_initContainer_&_ephemeralContainer_violate_true_allowed_negative",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privileged Containers",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.initContainers[*].securityContext.privileged",
					"values": [
						"true"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"privileged": false
						}
					}
				],
				"initContainers": [
					{
						"name": "nginx-init",
						"image": "nginx",
						"securityContext": {
							"privileged": true
						}
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx-ephemeral",
						"image": "nginx",
						"securityContext": {
							"privileged": true
						}
					}
				]
			}
		}`),
		allowed: false,
	},
}

var baseline_capabilities = []testCase{
	{
		name: "baseline_capabilities_defines_container_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"capabilities": {
								"add": [
									"FAKE_VALUE"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_capabilities_defines_container_violate_false",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"capabilities": {
								"add": [
									"KILL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_capabilities_foo_defines_container_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"capabilities": {
								"add": [
									"FOO", "BAR"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_capabilities_foo_defines_container_allow_positive",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.capabilities.add",
					"values": ["FOO", "BAR"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"capabilities": {
								"add": [
									"FOO", "BAR"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_capabilities_foo_defines_initContainer_&_ephemeralContainer_allow_positive",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.initContainers[*].securityContext.capabilities.add",
					"values": ["FOO", "BAR"]
				},
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.ephemeralContainers[*].securityContext.capabilities.add",
					"values": ["FOO", "BAZ"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"initContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"capabilities": {
								"add": [
									"FOO", "BAR"
								]
							}
						}
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"capabilities": {
								"add": [
									"FOO", "BAZ"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_capabilities_foo_defines_initContainer_&_ephemeralContainer_allow_negative",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.initContainers[*].securityContext.capabilities.add",
					"values": ["FOO", "BAR"]
				},
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.ephemeralContainers[*].securityContext.capabilities.add",
					"values": ["FOO", "BAR"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"initContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"capabilities": {
								"add": [
									"FOO", "BAR"
								]
							}
						}
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"capabilities": {
								"add": [
									"FOO", "BAZ"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_capabilities_foo_defines_container_allow_negative",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.capabilities.add",
					"values": ["FOO"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"capabilities": {
								"add": [
									"FOO", "BAR"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_capabilities_defines_container_none",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_capabilities_defines_ephemeralContainers_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx-ephemeral",
						"image": "nginx",
						"securityContext": {
							"capabilities": {
								"add": [
									"FAKE_VALUE"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_capabilities_defines_ephemeralContainers_violate_false",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx-ephemeral",
						"image": "nginx",
						"securityContext": {
							"capabilities": {
								"add": [
									"KILL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_capabilities_defines_ephemeralContainers_none",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx-ephemeral",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_capabilities_not_match",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx-ephemeral",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: false,
	},
}

var baseline_hostPath_volumes = []testCase{
	{
		name: "baseline_hostPath_volumes_exclude_path_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "HostPath Volumes",
					"restrictedField": "spec.volumes[*].hostPath",
					"values": [
						"/etc/nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				],
				"volumes": [
					{
						"hostPath": {
							"path": "/etc/nginx"
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_hostPath_volumes_exclude_path_false",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "HostPath Volumes",
					"restrictedField": "spec.volumes[*].hostPath",
					"values": [
						"/etc/nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				],
				"volumes": [
					{
						"hostPath": {
							"path": "/var/lib1"
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_hostPath_volumes_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "HostPath Volumes"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				],
				"volumes": [
					{
						"hostPath": {
							"path": "/var/lib1"
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_hostPath_volumes_violate_false",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "HostPath Volumes"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_hostPath_volumes_not_match",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privileged Containers",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				],
				"volumes": [
					{
						"hostPath": {
							"path": "/var/lib1"
						}
					}
				]
			}
		}`),
		allowed: false,
	},
}

var baseline_host_ports = []testCase{
	{
		name: "baseline_host_ports_defines_0",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Host Ports"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"ports": [
							{
								"hostPort": 0
							}
						]
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_host_ports_defines_non_zero",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Host Ports",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"ports": [
							{
								"hostPort": 1000
							}
						]
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_host_ports_define_different_values",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Host Ports",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"ports": [
							{
								"hostPort": 10,
								"hostPort": 20
							}
						]
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_host_ports_initContainer_&_ephemeralContainer_define_different_values_allowed_positive",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Host Ports",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.initContainers[*].ports[*].hostPort",
					"values": [
						"10", "20"
					]
				},
				{
					"controlName": "Host Ports",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.ephemeralContainers[*].ports[*].hostPort",
					"values": [
						"10", "20"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"initContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"ports": [
							{
								"hostPort": 10,
								"hostPort": 20
							}
						]
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"ports": [
							{
								"hostPort": 10,
								"hostPort": 20
							}
						]
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_host_ports_initContainer_&_ephemeralContainer_define_different_values_allowed_negative",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Host Ports",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.initContainers[*].ports[*].hostPort",
					"values": [
						"10", "20"
					]
				},
				{
					"controlName": "Host Ports",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.ephemeralContainers[*].ports[*].hostPort",
					"values": [
						"10"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"initContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"ports": [
							{
								"hostPort": 10,
								"hostPort": 20
							}
						]
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"ports": [
							{
								"hostPort": 20
							}
						]
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_host_ports_define_different_values_allow_negative",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Host Ports",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].ports.hostPort",
					"values": ["-1"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"ports": [
							{
								"hostPort": 10,
								"hostPort": 20
							}
						]
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_host_ports_undefined",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Host Ports",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
}

var baseline_appArmor = []testCase{
	{
		name: "baseline_appArmor_undefined",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "AppArmor"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_appArmor_defines_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "AppArmor"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test",
				"annotations": {
					"container.apparmor.security.beta.kubernetes.io/kyverno.test": "fake_value"
				}
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_appArmor_defines_violate_false",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "AppArmor"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test",
				"annotations": {
					"container.apparmor.security.beta.kubernetes.io/kyverno.test": "runtime/default"
				}
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_appArmor_defines_multiple_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "AppArmor"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test",
				"annotations": {
					"container.apparmor.security.beta.kubernetes.io/": "bogus",
					"container.apparmor.security.beta.kubernetes.io/a": "",
					"container.apparmor.security.beta.kubernetes.io/b": "runtime/default",
					"container.apparmor.security.beta.kubernetes.io/c": "localhost/",
					"container.apparmor.security.beta.kubernetes.io/d": "localhost/foo",
					"container.apparmor.security.beta.kubernetes.io/e": "unconfined",
					"container.apparmor.security.beta.kubernetes.io/f": "unknown"
				}
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_appArmor_defines_multiple_allow_positive",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "AppArmor",
					"restrictedField": "metadata.annotations[container.apparmor.security.beta.kubernetes.io/]",
					"values": ["bogus"]
				},
				{
					"controlName": "AppArmor",
					"restrictedField": "metadata.annotations[container.apparmor.security.beta.kubernetes.io/a]",
					"values": ["bogus"]
				},
				{
					"controlName": "AppArmor",
					"restrictedField": "metadata.annotations[container.apparmor.security.beta.kubernetes.io/e]",
					"values": ["unconfined"]
				},
				{
					"controlName": "AppArmor",
					"restrictedField": "metadata.annotations[container.apparmor.security.beta.kubernetes.io/f]",
					"values": ["unknown"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test",
				"annotations": {
					"container.apparmor.security.beta.kubernetes.io/": "bogus",
					"container.apparmor.security.beta.kubernetes.io/a": "",
					"container.apparmor.security.beta.kubernetes.io/b": "runtime/default",
					"container.apparmor.security.beta.kubernetes.io/c": "localhost/",
					"container.apparmor.security.beta.kubernetes.io/d": "localhost/foo",
					"container.apparmor.security.beta.kubernetes.io/e": "unconfined",
					"container.apparmor.security.beta.kubernetes.io/f": "unknown"
				}
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_appArmor_defines_multiple_allow_negative",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "AppArmor",
					"restrictedField": "metadata.annotations[container.apparmor.security.beta.kubernetes.io/]",
					"values": ["bogus"]
				},
				{
					"controlName": "AppArmor",
					"restrictedField": "metadata.annotations[container.apparmor.security.beta.kubernetes.io/a]",
					"values": ["bogus"]
				},
				{
					"controlName": "AppArmor",
					"restrictedField": "metadata.annotations[container.apparmor.security.beta.kubernetes.io/e]",
					"values": ["unconfined"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test",
				"annotations": {
					"container.apparmor.security.beta.kubernetes.io/": "bogus",
					"container.apparmor.security.beta.kubernetes.io/a": "",
					"container.apparmor.security.beta.kubernetes.io/b": "runtime/default",
					"container.apparmor.security.beta.kubernetes.io/c": "localhost/",
					"container.apparmor.security.beta.kubernetes.io/d": "localhost/foo",
					"container.apparmor.security.beta.kubernetes.io/e": "unconfined",
					"container.apparmor.security.beta.kubernetes.io/f": "unknown"
				}
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_appArmor_not_match_block",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "HostPath Volumes"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test",
				"annotations": {
					"container.apparmor.security.beta.kubernetes.io/kyverno.test": "fake_value"
				}
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_appArmor_not_match_pass",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "HostPath Volumes"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test",
				"annotations": {
					"container.apparmor.security.beta.kubernetes.io/kyverno.test": "localhost/default"
				}
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
}

var baseline_seLinux = []testCase{
	{
		name: "baseline_seLinux_type_defines_all_violate_true_1",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seLinuxOptions": {
						"type": "fake_value"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"type": "fake_value"
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_seLinux_type_defines_all_violate_true_2",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux"
				},
				{
					"controlName": "SELinux",
					"images": [
						"nginx"
					]
				}
			]
		}`),

		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seLinuxOptions": {
						"type": "fake_value"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"type": "fake_value"
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seLinux_type_defines_all_violate_false",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seLinuxOptions": {
						"type": "container_t"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"type": "container_t"
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seLinux_type_defines_container_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"type": "fake_value"
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seLinux_type_defines_container_violate_false",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"type": "container_t"
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seLinux_type_defines_spec_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seLinuxOptions": {
						"type": "fake_value"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seLinux_type_defines_spec",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux",
					"restrictedField": "spec.securityContext.seLinuxOptions.type",
					"values": [
						"fake_value"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seLinuxOptions": {
						"type": "fake_value"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seLinux_type_defines_spec_violate_false",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seLinuxOptions": {
						"type": "container_t"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seLinux_type_defines_bad_spec_violate_false",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seLinuxOptions": {
						"type": "bad"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seLinux_type_defines_bad_spec_allow_positive",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux",
					"restrictedField": "spec.securityContext.seLinuxOptions.type",
					"values": ["bad"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seLinuxOptions": {
						"type": "bad"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seLinux_type_defines_bad_spec_allow_negative",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux",
					"restrictedField": "spec.securityContext.seLinuxOptions.type",
					"values": ["good"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seLinuxOptions": {
						"type": "bad"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_seLinux_type_securityContext_nil_violate_false",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "a",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"type": "container_t"
							}
						}
					},
					{
						"name": "b",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"type": "container_init_t"
							}
						}
					},
					{
						"name": "c",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"type": "container_kvm_t"
							}
						}
					},
					{
						"name": "d",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"type": "bar"
							}
						}
					},
					{
						"name": "e",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"user": "bar"
							}
						}
					},
					{
						"name": "f",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"role": "baz"
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seLinux_type_securityContext_nil_allow_positive",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.seLinuxOptions.type",
					"values": ["bar"]
				},
				{
					"controlName": "SELinux",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.seLinuxOptions.user",
					"values": ["bar"]
				},
				{
					"controlName": "SELinux",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.seLinuxOptions.role",
					"values": ["baz"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "a",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"type": "container_t"
							}
						}
					},
					{
						"name": "b",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"type": "container_init_t"
							}
						}
					},
					{
						"name": "c",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"type": "container_kvm_t"
							}
						}
					},
					{
						"name": "d",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"type": "bar"
							}
						}
					},
					{
						"name": "e",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"user": "bar"
							}
						}
					},
					{
						"name": "f",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"role": "baz"
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seLinux_type_securityContext_nil_allow_negative",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.seLinuxOptions.type",
					"values": ["bar"]
				},
				{
					"controlName": "SELinux",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.seLinuxOptions.user",
					"values": ["bar"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "a",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"type": "container_t"
							}
						}
					},
					{
						"name": "b",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"type": "container_init_t"
							}
						}
					},
					{
						"name": "c",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"type": "container_kvm_t"
							}
						}
					},
					{
						"name": "d",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"type": "bar"
							}
						}
					},
					{
						"name": "e",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"user": "bar"
							}
						}
					},
					{
						"name": "f",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"role": "baz"
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_seLinux_type_securityContext_initContainer_&_ephemeralContainer_nil_allow_positive",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.seLinuxOptions.type",
					"values": ["bar"]
				},
				{
					"controlName": "SELinux",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.initContainers[*].securityContext.seLinuxOptions.user",
					"values": ["bar"]
				},
				{
					"controlName": "SELinux",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.ephemeralContainers[*].securityContext.seLinuxOptions.role",
					"values": ["bar"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "a",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"type": "bar"
							}
						}
					}
				],
				"initContainers": [
					{
						"name": "a",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"user": "bar"
							}
						}
					}
				],
				"ephemeralContainers": [
					{
						"name": "a",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"role": "bar"
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seLinux_type_securityContext_initContainer_&_ephemeralContainer_nil_allow_negative",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.seLinuxOptions.type",
					"values": ["bar"]
				},
				{
					"controlName": "SELinux",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.initContainers[*].securityContext.seLinuxOptions.user",
					"values": ["bar"]
				},
				{
					"controlName": "SELinux",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.ephemeralContainers[*].securityContext.seLinuxOptions.role",
					"values": ["baz"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "a",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"type": "bar"
							}
						}
					}
				],
				"initContainers": [
					{
						"name": "a",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"user": "bar"
							}
						}
					}
				],
				"ephemeralContainers": [
					{
						"name": "a",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"role": "bar"
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_seLinux_type_not_match_pass",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "HostPath Volumes"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seLinuxOptions": {
						"type": "container_t"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seLinux_type_not_match_block",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "HostPath Volumes"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seLinuxOptions": {
						"type": "fake_value"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_seLinux_type_container_not_match_pass",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Host Ports",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"type": "container_t"
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seLinux_type_container_not_match_block",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Host Ports",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"type": "fake_value"
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_seLinux_type_defines_none",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seLinux_user_defines_spec_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seLinuxOptions": {
						"user": "fake_value"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seLinux_user_defines_bad_spec_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seLinuxOptions": {
						"user": "bad"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seLinux_user_defines_bad_spec_allowed_positive",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux",
					"restrictedField": "spec.securityContext.seLinuxOptions.user",
					"values": ["bad"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seLinuxOptions": {
						"user": "bad"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seLinux_user_defines_bad_spec_allowed_negative",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux",
					"restrictedField": "spec.securityContext.seLinuxOptions.user",
					"values": ["good"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seLinuxOptions": {
						"user": "bad"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_seLinux_role_defines_bad_spec_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seLinuxOptions": {
						"role": "bad"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seLinux_role_defines_bad_spec_allowed_positive",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux",
					"restrictedField": "spec.securityContext.seLinuxOptions.role",
					"values": ["bad"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seLinuxOptions": {
						"role": "bad"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seLinux_role_defines_bad_spec_allowed_negative",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux",
					"restrictedField": "spec.securityContext.seLinuxOptions.role",
					"values": ["good"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seLinuxOptions": {
						"role": "bad"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_seLinux_role_defines_container_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "SELinux",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seLinuxOptions": {
								"role": "fake_value"
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
}

var baseline_procMount = []testCase{
	{
		name: "baseline_procMount_undefined",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "/proc Mount Type",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_procMount_defines_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "/proc Mount Type",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"procMount": "fakeValue"
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_procMount_defines_violate_false",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "/proc Mount Type",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"procMount": "Default"
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_procMount_defines_multiple_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "/proc Mount Type",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": null
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"procMount": "Default"
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"procMount": "Unmasked"
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"procMount": "other"
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_procMount_defines_multiple_allowed_positive",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "/proc Mount Type",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.procMount",
					"values": ["Unmasked", "other"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": null
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"procMount": "Default"
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"procMount": "Unmasked"
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"procMount": "other"
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_procMount_defines_multiple_allowed_negative",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "/proc Mount Type",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.procMount",
					"values": ["Unmasked"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": null
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"procMount": "Default"
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"procMount": "Unmasked"
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"procMount": "other"
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_procMount_defines_multiple_initContainer_&_ephemeralContainer_allowed_positive",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "/proc Mount Type",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.initContainers[*].securityContext.procMount",
					"values": ["Unmasked"]
				},
				{
					"controlName": "/proc Mount Type",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.ephemeralContainers[*].securityContext.procMount",
					"values": ["other"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"initContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"procMount": "Unmasked"
						}
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"procMount": "other"
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_procMount_defines_multiple_initContainer_&_ephemeralContainer_allowed_negative",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "/proc Mount Type",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.initContainers[*].securityContext.procMount",
					"values": ["Unmasked"]
				},
				{
					"controlName": "/proc Mount Type",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.ephemeralContainers[*].securityContext.procMount",
					"values": ["others"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"initContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"procMount": "Unmasked"
						}
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"procMount": "other"
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_procMount_not_match_pass",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privileged Containers",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"procMount": "Default"
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_procMount_not_match_block",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privileged Containers",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"procMount": "fakeValue"
						}
					}
				]
			}
		}`),
		allowed: false,
	},
}

var baseline_seccompProfile = []testCase{
	{
		name: "baseline_seccompProfile_no_exclusion",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "latest"
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "Unconfined"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "Unconfined"
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_seccompProfile_defines_all_violate_true_1",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "fake"
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seccompProfile_defines_all_violate_true_2",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "fake"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "RuntimeDefault"
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seccompProfile_defines_all_violate_false_1",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "RuntimeDefault"
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seccompProfile_defines_all_violate_false_2",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "RuntimeDefault"
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seccompProfile_metadata_annotations_allow_unconfined",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.0"
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "RuntimeDefault"
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seccompProfile_defines_multiple_all_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.19",
			"exclude": [
				{
					"controlName": "Seccomp"
				},
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "Unconfined"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": null
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "Unconfined"
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "RuntimeDefault"
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "Localhost"
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seccompProfile_defines_multiple_all_allowed_positive",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.19",
			"exclude": [
				{
					"controlName": "Seccomp",
					"restrictedField": "spec.securityContext.seccompProfile.type",
					"values": ["Unconfined"]
				},
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.seccompProfile.type",
					"values": ["Unconfined"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "Unconfined"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": null
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "Unconfined"
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "RuntimeDefault"
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "Localhost"
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seccompProfile_defines_multiple_all_allowed_negative",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.19",
			"exclude": [
				{
					"controlName": "Seccomp",
					"restrictedField": "spec.securityContext.seccompProfile.type",
					"values": ["unknown"]
				},
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.seccompProfile.type",
					"values": ["Unconfined"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "Unconfined"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": null
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "Unconfined"
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "RuntimeDefault"
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "Localhost"
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_seccompProfile_defines_multiple_initContainer_&_ephemeralContainer_all_allowed_positive",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.19",
			"exclude": [
				{
					"controlName": "Seccomp",
					"restrictedField": "spec.securityContext.seccompProfile.type",
					"values": ["Unconfined"]
				},
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.initContainers[*].securityContext.seccompProfile.type",
					"values": ["Unconfined"]
				},
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.ephemeralContainers[*].securityContext.seccompProfile.type",
					"values": ["Unconfined"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "Unconfined"
					}
				},
				"initContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "Unconfined"
							}
						}
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "Unconfined"
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seccompProfile_defines_multiple_initContainer_&_ephemeralContainer_all_allowed_negative",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.19",
			"exclude": [
				{
					"controlName": "Seccomp",
					"restrictedField": "spec.securityContext.seccompProfile.type",
					"values": ["Unconfined"]
				},
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.initContainers[*].securityContext.seccompProfile.type",
					"values": ["Unconfined"]
				},
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.ephemeralContainers[*].securityContext.seccompProfile.type",
					"values": ["unknown"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "Unconfined"
					}
				},
				"initContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "Unconfined"
							}
						}
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "Unconfined"
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_seccompProfile_defines_container_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "fake"
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seccompProfile_defines_container_allowed_positive",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.seccompProfile.type",
					"values": ["fake"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "fake"
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seccompProfile_defines_container_allowed_negative",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.seccompProfile.type",
					"values": ["real"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "fake"
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_seccompProfile_defines_container_violate_false",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [

			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "RuntimeDefault"
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seccompProfile_defines_spec_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "fake"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seccompProfile_defines_spec_allowed_positive",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp",
					"restrictedField": "spec.securityContext.seccompProfile.type",
					"values": ["fake"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "fake"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_seccompProfile_defines_spec_allowed_negative",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp",
					"restrictedField": "spec.securityContext.seccompProfile.type",
					"values": ["true"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "fake"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_seccompProfile_defines_spec_violate_false",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24"
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
}

var baseline_sysctls = []testCase{
	{
		name: "baseline_sysctls_undefined",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Sysctls"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_sysctls_defines_violate_true",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Sysctls"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"sysctls": [
                		{
                    		"name": "fake.value"
                		}
            		]
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_sysctls_defines_allowed_positive",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Sysctls",
					"restrictedField": "spec.securityContext.sysctls[*].name",
					"values": ["fake.value"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"sysctls": [
                		{
                    		"name": "fake.value"
                		}
            		]
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_sysctls_defines_violate_false",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Sysctls"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"sysctls": [
                		{
                    		"name": "kernel.shm_rmid_forced"
                		}
            		]
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_sysctls_multiple_sysctls_pass",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.0",
			"exclude": [
				{
					"controlName": "Sysctls"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"sysctls": [
                		{
                    		"name": "a"
                		},
						{
							"name": "b"
						}
            		]
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_sysctls_multiple_sysctls_pass_allowed_positive",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.0",
			"exclude": [
				{
					"controlName": "Sysctls",
					"restrictedField": "spec.securityContext.sysctls[*].name",
					"values": ["a", "b"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"sysctls": [
                		{
                    		"name": "a"
                		},
						{
							"name": "b"
						}
            		]
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_sysctls_multiple_sysctls_pass_allowed_negative",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.0",
			"exclude": [
				{
					"controlName": "Sysctls",
					"restrictedField": "spec.securityContext.sysctls[*].name",
					"values": ["a"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"sysctls": [
                		{
                    		"name": "a"
                		},
						{
							"name": "b"
						}
            		]
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_sysctls_new_sysctls_pass",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.0",
			"exclude": [
				{
					"controlName": "Sysctls"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"sysctls": [
                		{
                    		"name": "net.ipv4.ip_local_reserved_ports"
                		}
            		]
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_sysctls_new_sysctls_pass_allowed_positive",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.0",
			"exclude": [
				{
					"controlName": "Sysctls",
					"restrictedField": "spec.securityContext.sysctls[*].name",
					"values": ["net.ipv4.ip_local_reserved_ports"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"sysctls": [
                		{
                    		"name": "net.ipv4.ip_local_reserved_ports"
                		}
            		]
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_sysctls_multiple_sysctls_pass_v1.24",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Sysctls"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"sysctls": [
                		{
                    		"name": "a"
                		},
						{
							"name": "b"
						}
            		]
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_sysctls_multiple_sysctls_pass_v1.24_allowed_positive",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Sysctls",
					"restrictedField": "spec.securityContext.sysctls[*].name",
					"values": ["a", "b"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"sysctls": [
                		{
                    		"name": "a"
                		},
						{
							"name": "b"
						}
            		]
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_sysctls_not_match_pass",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"sysctls": [
                		{
                    		"name": "kernel.shm_rmid_forced"
                		}
            		]
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_sysctls_not_match_pass_allowed_positive",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp",
					"restrictedField": "spec.securityContext.sysctls[*].name",
					"values": ["kernel.shm_rmid_forced"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"sysctls": [
                		{
                    		"name": "kernel.shm_rmid_forced"
                		}
            		]
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "baseline_sysctls_not_match_block",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"sysctls": [
                		{
                    		"name": "fake.value"
                		}
            		]
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "baseline_sysctls_not_match_block_allowed_positive",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp",
					"restrictedField": "spec.securityContext.sysctls[*].name",
					"values": ["fake.value"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"sysctls": [
                		{
                    		"name": "fake.value"
                		}
            		]
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: false,
	},
}

var restricted_volume_types = []testCase{
	{
		name: "restricted_volume_types_undefined",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Volume Types"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": true,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_volume_types_not_match_block",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": true,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				],
				"volumes": [
					{
						"name": "test-volume",
						"awsElasticBlockStore": null,
						"volumeID": "<volume id>",
						"fsType": "ext4"
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "restricted_volume_types_defines_violate_true",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Volume Types"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": true,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				],
				"volumes": [
					{
						"name": "test-volume",
						"awsElasticBlockStore": null,
						"volumeID": "<volume id>",
						"fsType": "ext4"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_volume_types_defines_violate_false",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Volume Types"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": true,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				],
				"volumes": [
					{
						"emptyDir": {}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_volume_types_defines_violate_false_not_match_pass",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "HostPath Volumes"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": true,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				],
				"volumes": [
					{
						"emptyDir": {}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_volume_types_defines_violate_true_not_match_block",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "HostPath Volumes"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": true,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				],
				"volumes": [
					{
						"hostPath": {
							"path": "/var/lib1"
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "restricted_volume_types_defines_violate_true_not_match_block",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Volume Types"
				},
				{
					"controlName": "HostPath Volumes"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": true,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				],
				"volumes": [
					{
						"emptyDir": {}
					},
					{
						"secret": {}
					},
					{
						"persistentVolumeClaim": {}
					},
					{
						"downwardAPI": {}
					},
					{
						"configMap": {}
					},
					{
						"projected": {}
					},
					{
						"csi": {}
					},
					{
						"ephemeral": {}
					},
					{
						"hostPath": {}
					},
					{
						"awsElasticBlockStore": {}
					},
					{
						"gitRepo": {}
					},
					{
						"nfs": {}
					},
					{
						"iscsi": {}
					},
					{
						"glusterfs": {}
					},
					{
						"rbd": {}
					},
					{
						"flexVolume": {}
					},
					{
						"cinder": {}
					},
					{
						"cephfs": {}
					},
					{
						"flocker": {}
					},
					{
						"fc": {}
					},
					{
						"azureFile": {}
					},
					{
						"vsphereVolume": {}
					},
					{
						"quobyte": {}
					},
					{
						"azureDisk": {}
					},
					{
						"photonPersistentDisk": {}
					},
					{
						"portworxVolume": {}
					},
					{
						"scaleIO": {}
					},
					{
						"storageos": {}
					},
					{
						"unknown": {}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_volume_types_defines_allow_positive",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Volume Types",
					"restrictedField": "spec.volumes[*].awsElasticBlockStore",
					"values": [""]
				},
				{
					"controlName": "Volume Types",
					"restrictedField": "spec.volumes[*].azureDisk",
					"values": [""]
				},
				{
					"controlName": "Volume Types",
					"restrictedField": "spec.volumes[*].azureFile",
					"values": [""]
				},
				{
					"controlName": "Volume Types",
					"restrictedField": "spec.volumes[*].cephfs",
					"values": [""]
				},
				{
					"controlName": "Volume Types",
					"restrictedField": "spec.volumes[*].cinder",
					"values": [""]
				},
				{
					"controlName": "Volume Types",
					"restrictedField": "spec.volumes[*].fc",
					"values": [""]
				},
				{
					"controlName": "Volume Types",
					"restrictedField": "spec.volumes[*].flexVolume",
					"values": [""]
				},
				{
					"controlName": "Volume Types",
					"restrictedField": "spec.volumes[*].flocker",
					"values": [""]
				},
				{
					"controlName": "Volume Types",
					"restrictedField": "spec.volumes[*].gitRepo",
					"values": [""]
				},
				{
					"controlName": "Volume Types",
					"restrictedField": "spec.volumes[*].glusterfs",
					"values": [""]
				},
				{
					"controlName": "Volume Types",
					"restrictedField": "spec.volumes[*].hostPath",
					"values": [""]
				},
				{
					"controlName": "Volume Types",
					"restrictedField": "spec.volumes[*].iscsi",
					"values": [""]
				},
				{
					"controlName": "Volume Types",
					"restrictedField": "spec.volumes[*].nfs",
					"values": [""]
				},
				{
					"controlName": "Volume Types",
					"restrictedField": "spec.volumes[*].photonPersistentDisk",
					"values": [""]
				},
				{
					"controlName": "Volume Types",
					"restrictedField": "spec.volumes[*].portworxVolume",
					"values": [""]
				},
				{
					"controlName": "Volume Types",
					"restrictedField": "spec.volumes[*].quobyte",
					"values": [""]
				},
				{
					"controlName": "Volume Types",
					"restrictedField": "spec.volumes[*].rbd",
					"values": [""]
				},
				{
					"controlName": "Volume Types",
					"restrictedField": "spec.volumes[*].scaleIO",
					"values": [""]
				},
				{
					"controlName": "Volume Types",
					"restrictedField": "spec.volumes[*].storageos",
					"values": [""]
				},
				{
					"controlName": "Volume Types",
					"restrictedField": "spec.volumes[*].unknown",
					"values": [""]
				},
				{
					"controlName": "Volume Types",
					"restrictedField": "spec.volumes[*].vsphereVolume",
					"values": [""]
				},
				{
					"controlName": "HostPath Volumes",
					"restrictedField": "spec.volumes[*].hostPath",
					"values": [""]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": true,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				],
				"volumes": [
					{
						"emptyDir": {}
					},
					{
						"secret": {}
					},
					{
						"persistentVolumeClaim": {}
					},
					{
						"downwardAPI": {}
					},
					{
						"configMap": {}
					},
					{
						"projected": {}
					},
					{
						"csi": {}
					},
					{
						"ephemeral": {}
					},
					{
						"hostPath": {}
					},
					{
						"awsElasticBlockStore": {}
					},
					{
						"gitRepo": {}
					},
					{
						"nfs": {}
					},
					{
						"iscsi": {}
					},
					{
						"glusterfs": {}
					},
					{
						"rbd": {}
					},
					{
						"flexVolume": {}
					},
					{
						"cinder": {}
					},
					{
						"cephfs": {}
					},
					{
						"flocker": {}
					},
					{
						"fc": {}
					},
					{
						"azureFile": {}
					},
					{
						"vsphereVolume": {}
					},
					{
						"quobyte": {}
					},
					{
						"azureDisk": {}
					},
					{
						"photonPersistentDisk": {}
					},
					{
						"portworxVolume": {}
					},
					{
						"scaleIO": {}
					},
					{
						"storageos": {}
					},
					{
						"unknown": {}
					}
				]
			}
		}`),
		allowed: true,
	},
}

var restricted_privilege_escalation = []testCase{
	{
		name: "restricted_privilege_escalation_undefined",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privilege Escalation",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": true,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_privilege_escalation_undefined_not_match_block",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": true,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "restricted_privilege_escalation_defines_container_violate_true",
		rawRule: []byte(`
			{
				"level": "restricted",
				"version": "v1.24",
				"exclude": [
					{
						"controlName": "Privilege Escalation",
						"images": [
							"nginx"
						]
					}
				]
			}`),
		rawPod: []byte(`
			{
				"kind": "Pod",
				"metadata": {
					"name": "test"
				},
				"spec": {
					"securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						}
					},
					"containers": [
						{
							"name": "nginx",
							"image": "nginx",
							"securityContext": {
								"allowPrivilegeEscalation": true,
								"runAsNonRoot": true,
								"capabilities": {
									"drop": [
										"ALL"
									]
								}
							}
						}
					]
				}
			}`),
		allowed: true,
	},

	{
		name: "restricted_privilege_escalation_defines_container_violate_false",
		rawRule: []byte(`
			{
				"level": "restricted",
				"version": "v1.24",
				"exclude": [
					{
						"controlName": "Privilege Escalation",
						"images": [
							"nginx"
						]
					}
				]
			}`),
		rawPod: []byte(`
			{
				"kind": "Pod",
				"metadata": {
					"name": "test"
				},
				"spec": {
					"securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						}
					},
					"containers": [
						{
							"name": "nginx",
							"image": "nginx",
							"securityContext": {
								"allowPrivilegeEscalation": false,
								"runAsNonRoot": true,
								"capabilities": {
									"drop": [
										"ALL"
									]
								}
							}
						}
					]
				}
			}`),
		allowed: true,
	},

	{
		name: "restricted_privilege_escalation_defines_container_violate_none",
		rawRule: []byte(`
			{
				"level": "restricted",
				"version": "v1.24",
				"exclude": [
					{
						"controlName": "Privilege Escalation",
						"images": [
							"nginx"
						]
					}
				]
			}`),
		rawPod: []byte(`
			{
				"kind": "Pod",
				"metadata": {
					"name": "test"
				},
				"spec": {
					"securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						}
					},
					"containers": [
						{
							"name": "nginx",
							"image": "nginx",
							"securityContext": {
								"allowPrivilegeEscalation": null,
								"runAsNonRoot": true,
								"capabilities": {
									"drop": [
										"ALL"
									]
								}
							}
						},
						{
							"name": "nginx",
							"image": "nginx",
							"securityContext": {
								"allowPrivilegeEscalation": false,
								"runAsNonRoot": true,
								"capabilities": {
									"drop": [
										"ALL"
									]
								}
							}
						},
						{
							"name": "nginx",
							"image": "nginx",
							"securityContext": {
								"allowPrivilegeEscalation": true,
								"runAsNonRoot": true,
								"capabilities": {
									"drop": [
										"ALL"
									]
								}
							}
						}
					]
				}
			}`),
		allowed: true,
	},

	{
		name: "restricted_privilege_escalation_defines_container_allow_negative",
		rawRule: []byte(`
			{
				"level": "restricted",
				"version": "v1.24",
				"exclude": [
					{
						"controlName": "Privilege Escalation",
						"images": [
							"nginx"
						],
						"restrictedField": "spec.containers[*].securityContext.allowPrivilegeEscalation",
						"values": ["falses"]
					}
				]
			}`),
		rawPod: []byte(`
			{
				"kind": "Pod",
				"metadata": {
					"name": "test"
				},
				"spec": {
					"securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						}
					},
					"containers": [
						{
							"name": "nginx",
							"image": "nginx",
							"securityContext": {
								"allowPrivilegeEscalation": null,
								"runAsNonRoot": true,
								"capabilities": {
									"drop": [
										"ALL"
									]
								}
							}
						},
						{
							"name": "nginx",
							"image": "nginx",
							"securityContext": {
								"allowPrivilegeEscalation": false,
								"runAsNonRoot": true,
								"capabilities": {
									"drop": [
										"ALL"
									]
								}
							}
						},
						{
							"name": "nginx",
							"image": "nginx",
							"securityContext": {
								"allowPrivilegeEscalation": true,
								"runAsNonRoot": true,
								"capabilities": {
									"drop": [
										"ALL"
									]
								}
							}
						}
					]
				}
			}`),
		allowed: false,
	},

	{
		name: "restricted_privilege_escalation_defines_initContainer_&_ephemeralContainer_allow_positive",
		rawRule: []byte(`
			{
				"level": "restricted",
				"version": "v1.24",
				"exclude": [
					{
						"controlName": "Privilege Escalation",
						"images": [
							"nginx"
						],
						"restrictedField": "spec.initContainers[*].securityContext.allowPrivilegeEscalation",
						"values": ["true"]
					},
					{
						"controlName": "Privilege Escalation",
						"images": [
							"nginx"
						],
						"restrictedField": "spec.ephemeralContainers[*].securityContext.allowPrivilegeEscalation",
						"values": ["true"]
					}
				]
			}`),
		rawPod: []byte(`
			{
				"kind": "Pod",
				"metadata": {
					"name": "test"
				},
				"spec": {
					"securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						}
					},
					"initContainers": [
						{
							"name": "nginx",
							"image": "nginx",
							"securityContext": {
								"allowPrivilegeEscalation": true,
								"runAsNonRoot": true,
								"capabilities": {
									"drop": [
										"ALL"
									]
								}
							}
						}
					],
					"ephemeralContainers": [
						{
							"name": "nginx",
							"image": "nginx",
							"securityContext": {
								"allowPrivilegeEscalation": true,
								"runAsNonRoot": true,
								"capabilities": {
									"drop": [
										"ALL"
									]
								}
							}
						}
					]
				}
			}`),
		allowed: true,
	},

	{
		name: "restricted_privilege_escalation_defines_initContainer_&_ephemeralContainer_allow_negative",
		rawRule: []byte(`
			{
				"level": "restricted",
				"version": "v1.24",
				"exclude": [
					{
						"controlName": "Privilege Escalation",
						"images": [
							"nginx"
						],
						"restrictedField": "spec.initContainers[*].securityContext.allowPrivilegeEscalation",
						"values": ["true"]
					}
				]
			}`),
		rawPod: []byte(`
			{
				"kind": "Pod",
				"metadata": {
					"name": "test"
				},
				"spec": {
					"securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						}
					},
					"initContainers": [
						{
							"name": "nginx",
							"image": "nginx",
							"securityContext": {
								"allowPrivilegeEscalation": true,
								"runAsNonRoot": true,
								"capabilities": {
									"drop": [
										"ALL"
									]
								}
							}
						}
					],
					"ephemeralContainers": [
						{
							"name": "nginx",
							"image": "nginx",
							"securityContext": {
								"allowPrivilegeEscalation": true,
								"runAsNonRoot": true,
								"capabilities": {
									"drop": [
										"ALL"
									]
								}
							}
						}
					]
				}
			}`),
		allowed: false,
	},

	{
		name: "restricted_privilege_escalation_defines_init_container_violate_true",
		rawRule: []byte(`
			{
				"level": "restricted",
				"version": "v1.24",
				"exclude": [
					{
						"controlName": "Privilege Escalation",
						"images": [
							"nginx"
						]
					}
				]
			}`),
		rawPod: []byte(`
			{
				"kind": "Pod",
				"metadata": {
					"name": "test"
				},
				"spec": {
					"securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						}
					},
					"containers": [
						{
							"name": "nginx",
							"image": "nginx",
							"securityContext": {
								"allowPrivilegeEscalation": false,
								"runAsNonRoot": true,
								"capabilities": {
									"drop": [
										"ALL"
									]
								}
							}
						}
					],
					"initContainers": [
						{
							"name": "nginx-init",
							"image": "nginx",
							"securityContext": {
								"allowPrivilegeEscalation": true,
								"runAsNonRoot": true,
								"capabilities": {
									"drop": [
										"ALL"
									]
								}
							}
						}
					]
				}
			}`),
		allowed: true,
	},

	{
		name: "restricted_privilege_escalation_defines_init_container_violate_false",
		rawRule: []byte(`
			{
				"level": "restricted",
				"version": "v1.24",
				"exclude": [
					{
						"controlName": "Privilege Escalation",
						"images": [
							"nginx"
						]
					}
				]
			}`),
		rawPod: []byte(`
			{
				"kind": "Pod",
				"metadata": {
					"name": "test"
				},
				"spec": {
					"securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						}
					},
					"containers": [
						{
							"name": "nginx",
							"image": "nginx",
							"securityContext": {
								"allowPrivilegeEscalation": false,
								"runAsNonRoot": true,
								"capabilities": {
									"drop": [
										"ALL"
									]
								}
							}
						}
					],
					"initContainers": [
						{
							"name": "nginx-init",
							"image": "nginx",
							"securityContext": {
								"allowPrivilegeEscalation": false,
								"runAsNonRoot": true,
								"capabilities": {
									"drop": [
										"ALL"
									]
								}
							}
						}
					]
				}
			}`),
		allowed: true,
	},

	{
		name: "restricted_privilege_escalation_defines_init_container_violate_true_not_match_block",
		rawRule: []byte(`
			{
				"level": "restricted",
				"version": "v1.24",
				"exclude": [
					{
						"controlName": "Running as Non-root",
						"images": [
							"nginx"
						]
					}
				]
			}`),
		rawPod: []byte(`
			{
				"kind": "Pod",
				"metadata": {
					"name": "test"
				},
				"spec": {
					"securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						}
					},
					"containers": [
						{
							"name": "nginx",
							"image": "nginx",
							"securityContext": {
								"allowPrivilegeEscalation": false,
								"runAsNonRoot": true,
								"capabilities": {
									"drop": [
										"ALL"
									]
								}
							}
						}
					],
					"initContainers": [
						{
							"name": "nginx-init",
							"image": "nginx",
							"securityContext": {
								"allowPrivilegeEscalation": true,
								"runAsNonRoot": true,
								"capabilities": {
									"drop": [
										"ALL"
									]
								}
							}
						}
					]
				}
			}`),
		allowed: false,
	},
}

var restricted_runAsNonRoot = []testCase{
	{
		name: "restricted_runAsNonRoot_defines_all_violate_none",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Privilege Escalation",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx"
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsNonRoot_defines_all_violate_spec_true_container_false",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root"
				}
			]
		}`),
		rawPod: []byte(`{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": false,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": true,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsNonRoot_defines_all_violate_spec_true_container_false_allow_positive",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root",
					"restrictedField": "spec.securityContext.runAsNonRoot",
					"values": ["false"]
				}
			]
		}`),
		rawPod: []byte(`{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": false,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": true,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsNonRoot_defines_all_violate_spec_true_container_false_allow_negative",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root",
					"restrictedField": "spec.securityContext.runAsNonRoot",
					"values": ["true"]
				}
			]
		}`),
		rawPod: []byte(`{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": false,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": true,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "restricted_runAsNonRoot_defines_all_violate_spec_false_container_false",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root"
				}
			]
		}`),
		rawPod: []byte(`{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": true,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsNonRoot_defines_all_violate_pod_nil",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root"
				},
				{
					"controlName": "Privilege Escalation",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Running as Non-root",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": null
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": false,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": true,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsNonRoot_defines_all_violate_multiple_container",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root"
				},
				{
					"controlName": "Privilege Escalation",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": null,
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": null
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": false
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": true
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsNonRoot_defines_all_violate_spec_true_container_true_spec_level_allowed_positive",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.runAsNonRoot",
					"values": ["false"]
				},
				{
					"controlName": "Running as Non-root",
					"restrictedField": "spec.securityContext.runAsNonRoot",
					"values": ["false"]
				}
			]
		}`),
		rawPod: []byte(`{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": false,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": false,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsNonRoot_defines_all_violate_spec_true_container_true_spec_level_allowed_negative",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.runAsNonRoot",
					"values": ["true"]
				},
				{
					"controlName": "Running as Non-root",
					"restrictedField": "spec.securityContext.runAsNonRoot",
					"values": ["false"]
				}
			]
		}`),
		rawPod: []byte(`{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": false,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": false,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "restricted_runAsNonRoot_defines_all_initContainer_&_ephemeralContainer_allowed_positive",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.initContainers[*].securityContext.runAsNonRoot",
					"values": ["false"]
				},
				{
					"controlName": "Running as Non-root",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.ephemeralContainers[*].securityContext.runAsNonRoot",
					"values": ["false"]
				},
				{
					"controlName": "Running as Non-root",
					"restrictedField": "spec.securityContext.runAsNonRoot",
					"values": ["false"]
				}
			]
		}`),
		rawPod: []byte(`{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": false,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"initContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": false,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": false,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsNonRoot_defines_all_initContainer_&_ephemeralContainer_allowed_negative",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.ephemeralContainers[*].securityContext.runAsNonRoot",
					"values": ["false"]
				},
				{
					"controlName": "Running as Non-root",
					"restrictedField": "spec.securityContext.runAsNonRoot",
					"values": ["false"]
				}
			]
		}`),
		rawPod: []byte(`{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": false,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"initContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": false,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": false,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "restricted_runAsNonRoot_defines_all_violate_spec_false_container_true_spec_level",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root"
				}
			]
		}`),
		rawPod: []byte(`{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": false,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "restricted_runAsNonRoot_defines_all_violate_spec_true_container_false_container_level",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": false,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": true,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "restricted_runAsNonRoot_defines_all_violate_spec_true_container_true_container_level",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": false,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsNonRoot_defines_all_violate_spec_false_container_true_container_level",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": false,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsNonRoot_defines_all_violate_spec_false_container_false_container_level",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": true,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsNonRoot_defines_container_only_violate_true",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": false,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsNonRoot_defines_container_only_violate_false",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsNonRoot": true,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsNonRoot_defines_spec_only_violate_true",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": false,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsNonRoot_defines_spec_only_violate_false",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsNonRoot_defines_spec_violate_true_not_match",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privilege Escalation",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": false,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "restricted_runAsNonRoot_defines_none",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsNonRoot_defines_none_not_match",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privilege Escalation",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "restricted_runAsNonRoot_defines_none",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsNonRoot_defines_none_not_match",
		rawRule: []byte(`{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privilege Escalation",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
}

var restricted_runAsUser = []testCase{
	{
		name: "restricted_runAsUser_defines_all_violate_true_spec_level",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root user"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsUser": 0,
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsUser": 1000,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsUser_defines_all_violate_null_spec_level",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root user"
				},
				{
					"controlName": "Privilege Escalation"
				},
				{
					"controlName": "Capabilities"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsUser": 0,
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": null
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsUser_defines_all_violate_null_spec_level_allow_positive",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root user",
					"restrictedField": "spec.securityContext.runAsUser",
					"values": ["0"]
				},
				{
					"controlName": "Privilege Escalation"
				},
				{
					"controlName": "Capabilities"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsUser": 0,
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": null
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsUser_defines_all_violate_null_spec_level_allow_negative",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root user",
					"restrictedField": "spec.securityContext.runAsUser",
					"values": ["1"]
				},
				{
					"controlName": "Privilege Escalation"
				},
				{
					"controlName": "Capabilities"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsUser": 0,
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": null
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "restricted_runAsUser_defines_all_violate_false_multiple_containers",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root user",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Privilege Escalation",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsUser": 1000,
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": null
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsUser": 0,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsUser": 1,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsUser_defines_all_multiple_containers_allow_positive",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root user",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.runAsUser",
					"values": ["0"]
				},
				{
					"controlName": "Privilege Escalation",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsUser": 1000,
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": null
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsUser": 0,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsUser": 1,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsUser_defines_all_multiple_containers_allow_negative",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root user",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.runAsUser",
					"values": ["1"]
				},
				{
					"controlName": "Privilege Escalation",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsUser": 1000,
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": null
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsUser": 0,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsUser": 1,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "restricted_runAsUser_defines_all_multiple_initContainer_&_ephemeralContainer_allow_positive",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root user",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.initContainers[*].securityContext.runAsUser",
					"values": ["0"]
				},
				{
					"controlName": "Running as Non-root user",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.ephemeralContainers[*].securityContext.runAsUser",
					"values": ["0"]
				},
				{
					"controlName": "Privilege Escalation",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsUser": 1000,
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"initContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsUser": 0,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsUser": 0,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsUser_defines_all_multiple_initContainer_&_ephemeralContainer_allow_negative",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root user",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.initContainers[*].securityContext.runAsUser",
					"values": ["0"]
				},
				{
					"controlName": "Running as Non-root user",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.ephemeralContainers[*].securityContext.runAsUser",
					"values": ["-1"]
				},
				{
					"controlName": "Privilege Escalation",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsUser": 1000,
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"initContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsUser": 0,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsUser": 0,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "restricted_runAsUser_defines_all_violate_false_spec_level",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root user"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsUser": 1000,
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsUser": 1000,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsUser_defines_all_violate_true_container_level",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root user",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsUser": 1000,
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsUser": 0,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsUser_defines_all_violate_false_container_level",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root user",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsUser": 1000,
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsUser": 1000,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsUser_defines_container_violate_true",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root user",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsUser": 0,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsUser_defines_container_violate_false",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root user",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"runAsUser": 1000,
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsUser_defines_spec_violate_true",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root user"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsUser": 0,
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsUser_defines_spec_violate_false",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root user"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsUser": 1000,
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsUser_defines_none",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root user"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_runAsUser_defines_spec_violate_true_not_match",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsUser": 0,
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
}

var restricted_seccompProfile = []testCase{
	{
		name: "restricted_seccompProfile_defines_container_no_seccompProfile",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {},
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_seccompProfile_defines_container_violate_true",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "fakeValue"
							},
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_seccompProfile_defines_container_allow_positive",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.seccompProfile.type",
					"values": ["fakeValue"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "fakeValue"
							},
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_seccompProfile_defines_container_allow_negative",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.seccompProfile.type",
					"values": ["fake"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "fakeValue"
							},
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "restricted_seccompProfile_defines_initContainer_&_ephemeralContainer_allow_positive",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.initContainers[*].securityContext.seccompProfile.type",
					"values": ["fake1"]
				},
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.ephemeralContainers[*].securityContext.seccompProfile.type",
					"values": ["fake2"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true
				},
				"initContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "fake1"
							},
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "fake2"
							},
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_seccompProfile_defines_initContainer_&_ephemeralContainer_allow_negative",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.initContainers[*].securityContext.seccompProfile.type",
					"values": ["fake1"]
				},
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.ephemeralContainers[*].securityContext.seccompProfile.type",
					"values": ["fake1"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true
				},
				"initContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "fake1"
							},
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "fake2"
							},
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "restricted_seccompProfile_defines_spec_violate_true",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "fakeValue"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_seccompProfile_defines_container_seccompProfile_type_unconfined",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Seccomp"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "fakeValue"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "Unconfined"
							},
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_seccompProfile_invalid",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Seccomp"
				},
				{
					"controlName": "Privilege Escalation",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": null
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "Unconfined"
							},
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "RuntimeDefault"
							},
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "Localhost"
							},
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_seccompProfile_invalid_multiple_containers",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privilege Escalation",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Running as Non-root",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": null,
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": null
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "Unconfined"
							},
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "RuntimeDefault"
							},
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "Localhost"
							},
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_seccompProfile_invalid_multiple_containers_allow_positive",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privilege Escalation",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Running as Non-root",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.seccompProfile.type",
					"values": ["Unconfined"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": null,
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": null
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "Unconfined"
							},
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "RuntimeDefault"
							},
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "Localhost"
							},
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_seccompProfile_invalid_multiple_containers_allow_negative",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privilege Escalation"
				},
				{
					"controlName": "Capabilities"
				},
				{
					"controlName": "Running as Non-root"
				},
				{
					"controlName": "Seccomp",
					"images": [
						"nginx1"
					],
					"restrictedField": "spec.containers[*].securityContext.seccompProfile.type",
					"values": ["unknown"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": null,
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": null
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {}
					},
					{
						"name": "nginx",
						"image": "nginx1",
						"securityContext": {
							"seccompProfile": {
								"type": "Unconfined"
							},
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx1",
						"securityContext": {
							"seccompProfile": {
								"type": "RuntimeDefault"
							},
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "Localhost"
							},
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "restricted_seccompProfile_undefined",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_seccompProfile_undefined_spec_level",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Seccomp"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_seccompProfile_undefined_not_match_block",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Running as Non-root user"
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
}

var restricted_capabilities = []testCase{
	{
		name: "restricted_capabilities_drop_undefined",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_capabilities_drop_defines_violate_true",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"KILL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_capabilities_drop_defines_violate_false",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_capabilities_add_undefined",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_capabilities_drop_defines_multiple_capabilities_violate_true",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									null
								],
								"add": [
									"BAR",
									"FOO"
								]
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"BAR",
									"FOO"
								],
								"add": [
									"BAR",
									"BAZ"
								]
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL",
									"FOO"
								],
								"add": [
									"NET_BIND_SERVICE",
									"CHOWN"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_capabilities_drop_defines_multiple_capabilities_allow_positive",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.capabilities.add",
					"values": ["BAR", "FOO", "BAZ", "CHOWN"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								],
								"add": [
									"BAR",
									"FOO"
								]
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL",
									"BAR",
									"FOO"
								],
								"add": [
									"BAR",
									"BAZ"
								]
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL",
									"FOO"
								],
								"add": [
									"NET_BIND_SERVICE",
									"CHOWN"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_capabilities_drop_defines_multiple_capabilities_allow_negative",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.containers[*].securityContext.capabilities.add",
					"values": ["BAR", "FOO", "BAZ"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								],
								"add": [
									"BAR",
									"FOO"
								]
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL",
									"BAR",
									"FOO"
								],
								"add": [
									"BAR",
									"BAZ"
								]
							}
						}
					},
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL",
									"FOO"
								],
								"add": [
									"NET_BIND_SERVICE",
									"CHOWN"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "restricted_capabilities_drop_defines_initContainer_&_ephemeralContainer_allow_positive",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.initContainers[*].securityContext.capabilities.add",
					"values": ["BAR"]
				},
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.ephemeralContainers[*].securityContext.capabilities.add",
					"values": ["FOO"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"initContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								],
								"add": [
									"BAR"
								]
							}
						}
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								],
								"add": [
									"FOO"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_capabilities_drop_defines_initContainer_&_ephemeralContainer_allow_negative",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.initContainers[*].securityContext.capabilities.add",
					"values": ["BAR"]
				},
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					],
					"restrictedField": "spec.ephemeralContainers[*].securityContext.capabilities.add",
					"values": ["BAR"]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"initContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								],
								"add": [
									"BAR"
								]
							}
						}
					}
				],
				"ephemeralContainers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								],
								"add": [
									"FOO"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "restricted_capabilities_add_undefined_not_match_block",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privilege Escalation",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "restricted_capabilities_add_undefined_not_match_pass",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privilege Escalation",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"drop": [
									"ALL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_capabilities_add_defines_violate_true",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"add": [
									"KILL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_capabilities_add_defines_violate_false",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Capabilities",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"add": [
									"NET_BIND_SERVICE"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "restricted_capabilities_add_defines_violate_true_not_match_block",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privilege Escalation",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"add": [
									"KILL"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "restricted_capabilities_add_defines_violate_false_not_match_pass",
		rawRule: []byte(`
		{
			"level": "restricted",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privilege Escalation",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"securityContext": {
					"runAsNonRoot": true,
					"seccompProfile": {
						"type": "RuntimeDefault"
					}
				},
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"allowPrivilegeEscalation": false,
							"capabilities": {
								"add": [
									"NET_BIND_SERVICE"
								]
							}
						}
					}
				]
			}
		}`),
		allowed: false,
	},
}

var wildcard_images = []testCase{
	{
		name: "wildcard_images_violate_true_image_not_match",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privileged Containers",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx:1.2.3",
						"securityContext": {
							"privileged": true
						}
					}
				]
			}
		}`),
		allowed: false,
	},
	{
		name: "wildcard_images_violate_true_image_match",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privileged Containers",
					"images": [
						"nginx"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx",
						"securityContext": {
							"privileged": true
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "wildcard_images_violate_true_image_match_wildcard",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privileged Containers",
					"images": [
						"nginx:*"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "nginx:1.2.3",
						"securityContext": {
							"privileged": true
						}
					}
				]
			}
		}`),
		allowed: true,
	},
	{
		name: "wildcard_images_violate_true_image_not_match_wildcard",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.24",
			"exclude": [
				{
					"controlName": "Privileged Containers",
					"images": [
						"nginx*"
					]
				}
			]
		}`),
		rawPod: []byte(`
		{
			"kind": "Pod",
			"metadata": {
				"name": "test"
			},
			"spec": {
				"containers": [
					{
						"name": "nginx",
						"image": "busybox",
						"securityContext": {
							"privileged": true
						}
					}
				]
			}
		}`),
		allowed: false,
	},
}

type testCase struct {
	name    string
	rawRule []byte
	rawPod  []byte
	allowed bool
}
