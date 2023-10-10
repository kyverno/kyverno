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

		allowed, checkResults, err := EvaluatePod(&rule, &pod)
		assert.Assert(t, err == nil)

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
		name: "baseline_privileged_defines_initContainer_violate_true",
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
				]
			}
		}`),
		allowed: true,
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
			"version": "v1.0",
			"exclude": [
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					],
					"metadata": {
						"name": "test",
						"annotations": {
							"seccomp.security.alpha.kubernetes.io/pod": "Unconfined"
						}
					}
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
		name: "baseline_seccompProfile_metadata_annotations_allow_multiple",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.0",
			"exclude": [
				{
					"controlName": "Seccomp",
					"images": [
						"nginx"
					],
					"metadata": {
						"name": "test",
						"annotations": {
							"container.seccomp.security.alpha.kubernetes.io/a": "Unconfined",
							"container.seccomp.security.alpha.kubernetes.io/b": "unknown"
						}
					}
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
						"name": "a",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "RuntimeDefault"
							}
						}
					},
					{
						"name": "b",
						"image": "nginx",
						"securityContext": {
							"seccompProfile": {
								"type": "RuntimeDefault"
							}
						}
					},
					{
						"name": "c",
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
		name: "baseline_seccompProfile_defines_container_violate_false",
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
		name: "baseline_seccompProfile_defines_spec_violate_false",
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
		name: "baseline_sysctls_multiple_sysctls_pass_v1.27",
		rawRule: []byte(`
		{
			"level": "baseline",
			"version": "v1.27",
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
					"controlName": "Running as Non-root"
				},
				{
					"controlName": "Privilege Escalation"
				},
				{
					"controlName": "Capabilities"
				},
				{
					"controlName": "Seccomp"
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
					"controlName": "Privilege Escalation"
				},
				{
					"controlName": "Capabilities"
				},
				{
					"controlName": "Running as Non-root",
					"images": [
						"nginx"
					]
				},
				{
					"controlName": "Seccomp"
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
					"controlName": "Privilege Escalation"
				},
				{
					"controlName": "Capabilities"
				},
				{
					"controlName": "Seccomp"
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
		name: "restricted_runAsNonRoot_defines_all_violate_spec_true_container_true_spec_level",
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
					"controlName": "Privilege Escalation"
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
		allowed: true,
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
