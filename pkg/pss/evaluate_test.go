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
		restricted_runAsNonRoot,
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
		for _, result := range checkResults {
			fmt.Printf("failed check result: %v\n", result)
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
		allowed: true,
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

var restricted_runAsNonRoot = []testCase{
	{
		name: "restricted_runAsNonRoot_defines_all_violate_true",
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
		name: "restricted_runAsNonRoot_defines_all_violate_false",
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
}

type testCase struct {
	name    string
	rawRule []byte
	rawPod  []byte
	allowed bool
}
