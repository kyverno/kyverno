package pss

import (
	"encoding/json"
	"fmt"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
)

type testCase struct {
	name    string
	rawRule []byte
	rawPod  []byte
	allowed bool
}

func Test_EvaluatePod(t *testing.T) {
	testCases := append(
		restricted_runAsNonRoot,
	)

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
		assert.Assert(t, allowed, fmt.Sprintf("test \"%s\" fails", test.name))
	}
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
		allowed: false,
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
		allowed: false,
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
		allowed: false,
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
