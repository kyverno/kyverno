package utils

import (
	"encoding/json"
	"testing"

	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	v2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/autogen"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMatchesResourceDescription(t *testing.T) {
	tcs := []struct {
		Description       string
		AdmissionInfo     v2.RequestInfo
		Resource          []byte
		Policy            []byte
		areErrorsExpected bool
	}{
		{
			Description: "Match Any matches the Pod",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"name": "abc",
					"namespace" : "prod"
				},
				"spec": {
					"containers": [
						{
							"name": "cont-name",
							"image": "cont-img",
							"ports": [
								{
									"containerPort": 81
								}
							],
							"resources": {
								"limits": {
									"memory": "30Mi",
									"cpu": "0.2"
								},
								"requests": {
									"memory": "20Mi",
									"cpu": "0.1"
								}
							}
						}
					]
				}
			}`),
			Policy: []byte(`{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "test-policy"
				},
				"spec": {
					"background": false,
					"rules": [
						{
							"name": "any-match-rule",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"names" : ["dev"]
										}
									},
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"namespaces" : ["prod"]
										}
									}
								]
							},
							"mutate": {
								"overlay": {
									"spec": {
										"containers": [
											{
												"(image)": "*",
												"imagePullPolicy": "IfNotPresent"
											}
										]
									}
								}
							}
						}
					]
				}
			}`),
			areErrorsExpected: false,
		},
		{
			Description: "Match Any does not match the Pod",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"name": "abc",
					"namespace" : "default"
				},
				"spec": {
					"containers": [
						{
							"name": "cont-name",
							"image": "cont-img",
							"ports": [
								{
									"containerPort": 81
								}
							],
							"resources": {
								"limits": {
									"memory": "30Mi",
									"cpu": "0.2"
								},
								"requests": {
									"memory": "20Mi",
									"cpu": "0.1"
								}
							}
						}
					]
				}
			}`),
			Policy: []byte(`{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "test-policy"
				},
				"spec": {
					"background": false,
					"rules": [
						{
							"name": "test-rule",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"names" : ["dev"]
										}
									},
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"namespaces" : ["prod"]
										}
									}
								]
							},
							"mutate": {
								"overlay": {
									"spec": {
										"containers": [
											{
												"(image)": "*",
												"imagePullPolicy": "IfNotPresent"
											}
										]
									}
								}
							}
						}
					]
				}
			}`),
			areErrorsExpected: true,
		},
		{
			Description: "Match All matches the Pod",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"name": "abc",
					"namespace" : "prod"
				},
				"spec": {
					"containers": [
						{
							"name": "cont-name",
							"image": "cont-img",
							"ports": [
								{
									"containerPort": 81
								}
							],
							"resources": {
								"limits": {
									"memory": "30Mi",
									"cpu": "0.2"
								},
								"requests": {
									"memory": "20Mi",
									"cpu": "0.1"
								}
							}
						}
					]
				}
			}`),
			Policy: []byte(`{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "test-policy"
				},
				"spec": {
					"background": false,
					"rules": [
						{
							"name": "test-rule",
							"match": {
								"all": [
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"names" : ["abc"]
										}
									},
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"namespaces" : ["prod"]
										}
									}
								]
							},
							"mutate": {
								"overlay": {
									"spec": {
										"containers": [
											{
												"(image)": "*",
												"imagePullPolicy": "IfNotPresent"
											}
										]
									}
								}
							}
						}
					]
				}
			}`),
			areErrorsExpected: false,
		},
		{
			Description: "Match All does not match the Pod",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"name": "abc",
					"namespace" : "prod"
				},
				"spec": {
					"containers": [
						{
							"name": "cont-name",
							"image": "cont-img",
							"ports": [
								{
									"containerPort": 81
								}
							],
							"resources": {
								"limits": {
									"memory": "30Mi",
									"cpu": "0.2"
								},
								"requests": {
									"memory": "20Mi",
									"cpu": "0.1"
								}
							}
						}
					]
				}
			}`),
			Policy: []byte(`{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "test-policy"
				},
				"spec": {
					"background": false,
					"rules": [
						{
							"name": "test-rule",
							"match": {
								"all": [
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"names" : ["xyz"]
										}
									},
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"namespaces" : ["prod"]
										}
									}
								]
							},
							"mutate": {
								"overlay": {
									"spec": {
										"containers": [
											{
												"(image)": "*",
												"imagePullPolicy": "IfNotPresent"
											}
										]
									}
								}
							}
						}
					]
				}
			}`),
			areErrorsExpected: true,
		},
		{
			Description: "Exclude Any excludes the Pod",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"name": "dev",
					"namespace" : "prod"
				},
				"spec": {
					"containers": [
						{
							"name": "cont-name",
							"image": "cont-img",
							"ports": [
								{
									"containerPort": 81
								}
							],
							"resources": {
								"limits": {
									"memory": "30Mi",
									"cpu": "0.2"
								},
								"requests": {
									"memory": "20Mi",
									"cpu": "0.1"
								}
							}
						}
					]
				}
			}`),
			Policy: []byte(`{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "test-policy"
				},
				"spec": {
					"background": false,
					"rules": [
						{
							"name": "test-rule",
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
							"exclude": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"names": [
												"dev"
											]
										}
									},
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"namespaces": [
												"default"
											]
										}
									}
								]
							},
							"mutate": {
								"overlay": {
									"spec": {
										"containers": [
											{
												"(image)": "*",
												"imagePullPolicy": "IfNotPresent"
											}
										]
									}
								}
							}
						}
					]
				}
			}`),
			areErrorsExpected: true,
		},
		{
			Description: "Exclude Any does not exclude the Pod",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"name": "abc",
					"namespace" : "prod"
				},
				"spec": {
					"containers": [
						{
							"name": "cont-name",
							"image": "cont-img",
							"ports": [
								{
									"containerPort": 81
								}
							],
							"resources": {
								"limits": {
									"memory": "30Mi",
									"cpu": "0.2"
								},
								"requests": {
									"memory": "20Mi",
									"cpu": "0.1"
								}
							}
						}
					]
				}
			}`),
			Policy: []byte(`{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "test-policy"
				},
				"spec": {
					"background": false,
					"rules": [
						{
							"name": "test-rule",
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
							"exclude": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"names": [
												"dev"
											]
										}
									},
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"namespaces": [
												"default"
											]
										}
									}
								]
							},
							"mutate": {
								"overlay": {
									"spec": {
										"containers": [
											{
												"(image)": "*",
												"imagePullPolicy": "IfNotPresent"
											}
										]
									}
								}
							}
						}
					]
				}
			}`),
			areErrorsExpected: false,
		},
		{
			Description: "Exclude All excludes the Pod",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"name": "dev",
					"namespace" : "prod"
				},
				"spec": {
					"containers": [
						{
							"name": "cont-name",
							"image": "cont-img",
							"ports": [
								{
									"containerPort": 81
								}
							],
							"resources": {
								"limits": {
									"memory": "30Mi",
									"cpu": "0.2"
								},
								"requests": {
									"memory": "20Mi",
									"cpu": "0.1"
								}
							}
						}
					]
				}
			}`),
			Policy: []byte(`{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "test-policy"
				},
				"spec": {
					"background": false,
					"rules": [
						{
							"name": "test-rule",
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
							"exclude": {
								"all": [
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"names": [
												"dev"
											]
										}
									},
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"namespaces": [
												"prod"
											]
										}
									}
								]
							},
							"mutate": {
								"overlay": {
									"spec": {
										"containers": [
											{
												"(image)": "*",
												"imagePullPolicy": "IfNotPresent"
											}
										]
									}
								}
							}
						}
					]
				}
			}`),
			areErrorsExpected: true,
		},
		{
			Description: "Exclude All does not exclude the Pod",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"name": "abc",
					"namespace" : "prod"
				},
				"spec": {
					"containers": [
						{
							"name": "cont-name",
							"image": "cont-img",
							"ports": [
								{
									"containerPort": 81
								}
							],
							"resources": {
								"limits": {
									"memory": "30Mi",
									"cpu": "0.2"
								},
								"requests": {
									"memory": "20Mi",
									"cpu": "0.1"
								}
							}
						}
					]
				}
			}`),
			Policy: []byte(`{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "test-policy"
				},
				"spec": {
					"background": false,
					"rules": [
						{
							"name": "test-rule",
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
							"exclude": {
								"all": [
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"names": [
												"abc"
											]
										}
									},
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"namespaces": [
												"default"
											]
										}
									}
								]
							},
							"mutate": {
								"overlay": {
									"spec": {
										"containers": [
											{
												"(image)": "*",
												"imagePullPolicy": "IfNotPresent"
											}
										]
									}
								}
							}
						}
					]
				}
			}`),
			areErrorsExpected: false,
		},
		{
			Description: "Should match pod and not exclude it",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"name":"hello-world"},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: false,
		},
		{
			Description: "Should exclude resource since it matches the exclude block",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"system:node"},
			},
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"name":"hello-world"},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: true,
		},
		{
			Description:       "Should not fail if in sync mode, if admission info is empty it should still match resources with specific clusterRoles",
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"name":"hello-world"},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: false,
		},
		{
			Description:       "Should fail since resource does not match because of names field",
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"],"names": ["dev-*"]},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: true,
		},
		{
			Description: "Should pass since resource matches a name in the names field",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"system:node"},
			},
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"],"names": ["dev-*","hello-world"]},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: false,
		},
		{
			Description:       "Should fail since resource gets excluded because of the names field",
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"names": ["dev-*","hello-*"]}},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: true,
		},
		{
			Description:       "Should pass since resource does not get excluded because of the names field",
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"bye-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"names": ["dev-*","hello-*"]}},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: false,
		},
		{
			Description: "Should fail since resource does not match policy",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{"apiVersion":"v1","kind":"Service","metadata":{"name":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"name":"hello-world"},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: true,
		},
		{
			Description: "Should not fail since resource does not match exclude block",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"system:node"},
			},
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"hello-world2","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"name":"hello-world"},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: false,
		},
		{
			Description: "Should pass since group, version, kind match",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{ "apiVersion": "apps/v1", "kind": "Deployment", "metadata": { "creationTimestamp": "2020-09-21T12:56:35Z", "name": "qos-demo", "labels": { "test": "qos" } }, "spec": { "replicas": 1, "selector": { "matchLabels": { "app": "nginx" } }, "template": { "metadata": { "creationTimestamp": "2020-09-21T12:56:35Z", "labels": { "app": "nginx" } }, "spec": { "containers": [ { "name": "nginx", "image": "nginx:latest", "resources": { "limits": { "cpu": "50m" } } } ]}}}}`),
			Policy:            []byte(`{ "apiVersion": "kyverno.io/v1", "kind": "ClusterPolicy", "metadata": { "name": "policy-qos" }, "spec": { "rules": [ { "name": "add-memory-limit", "match": { "resources": { "kinds": [ "apps/v1/Deployment" ], "selector": { "matchLabels": { "test": "qos" } } } }, "mutate": { "overlay": { "spec": { "template": { "spec": { "containers": [ { "(name)": "*", "resources": { "limits": { "+(memory)": "300Mi", "+(cpu)": "100" } } } ] } } } } } }, { "name": "check-cpu-memory-limits", "match": { "resources": { "kinds": [ "apps/v1/Deployment" ], "selector": { "matchLabels": { "test": "qos" } } } }, "validate": { "message": "Resource limits are required for CPU and memory", "pattern": { "spec": { "template": { "spec": { "containers": [ { "(name)": "*", "resources": { "limits": { "memory": "?*", "cpu": "?*" } } } ] } } } } } } ] } }`),
			areErrorsExpected: false,
		},
		{
			Description: "Should pass since version and kind match",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{ "apiVersion": "v1", "kind": "Pod", "metadata": { "name": "myapp-pod2", "labels": { "app": "myapp2" } }, "spec": { "containers": [ { "name": "nginx", "image": "nginx" } ] } }`),
			Policy:            []byte(`{ "apiVersion": "kyverno.io/v1", "kind": "ClusterPolicy", "metadata": { "name": "disallow-latest-tag", "annotations": { "policies.kyverno.io/category": "Workload Isolation", "policies.kyverno.io/description": "The ':latest' tag is mutable and can lead to unexpected errors if the image changes. A best practice is to use an immutable tag that maps to a specific version of an application pod." } }, "spec": {"rules": [ { "name": "require-image-tag", "match": { "resources": { "kinds": [ "v1/Pod" ] } }, "validate": { "failureAction": "enforce", "message": "An image tag is required", "pattern": { "spec": { "containers": [ { "image": "*:*" } ] } } } } ] } }`),
			areErrorsExpected: false,
		},
		{
			Description: "Should fail since resource does not match ",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{"apiVersion":"v1","kind":"Service","metadata":{"name":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"name":"hello-world"},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: true,
		},
		{
			Description: "Should fail since version not match",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{ "apiVersion": "apps/v1beta1", "kind": "Deployment", "metadata": { "creationTimestamp": "2020-09-21T12:56:35Z", "name": "qos-demo", "labels": { "test": "qos" } }, "spec": { "replicas": 1, "selector": { "matchLabels": { "app": "nginx" } }, "template": { "metadata": { "creationTimestamp": "2020-09-21T12:56:35Z", "labels": { "app": "nginx" } }, "spec": { "containers": [ { "name": "nginx", "image": "nginx:latest", "resources": { "limits": { "cpu": "50m" } } } ]}}}}`),
			Policy:            []byte(`{ "apiVersion": "kyverno.io/v1", "kind": "ClusterPolicy", "metadata": { "name": "policy-qos" }, "spec": { "rules": [ { "name": "add-memory-limit", "match": { "resources": { "kinds": [ "apps/v1/Deployment" ], "selector": { "matchLabels": { "test": "qos" } } } }, "mutate": { "overlay": { "spec": { "template": { "spec": { "containers": [ { "(name)": "*", "resources": { "limits": { "+(memory)": "300Mi", "+(cpu)": "100" } } } ] } } } } } }, { "name": "check-cpu-memory-limits", "match": { "resources": { "kinds": [ "apps/v1/Deployment" ], "selector": { "matchLabels": { "test": "qos" } } } }, "validate": { "message": "Resource limits are required for CPU and memory", "pattern": { "spec": { "template": { "spec": { "containers": [ { "(name)": "*", "resources": { "limits": { "memory": "?*", "cpu": "?*" } } } ] } } } } } } ] } }`),
			areErrorsExpected: true,
		},
		{
			Description: "Should fail since cluster role version not match",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{ "kind": "ClusterRole", "apiVersion": "rbac.authorization.k8s.io/v1", "metadata": { "name": "secret-reader-demo", "namespace": "default" }, "rules": [ { "apiGroups": [ "" ], "resources": [ "secrets" ], "verbs": [ "get", "watch", "list" ] } ] }`),
			Policy:            []byte(`{ "apiVersion": "kyverno.io/v1", "kind": "ClusterPolicy", "metadata": { "name": "check-host-path" }, "spec": { "background": true, "rules": [ { "name": "check-host-path", "match": { "resources": { "kinds": [ "rbac.authorization.k8s.io/v1beta1/ClusterRole" ] } }, "validate": { "failureAction": "enforce", "message": "Host path is not allowed", "pattern": { "spec": { "volumes": [ { "name": "*", "hostPath": { "path": "" } } ] } } } } ] } }`),
			areErrorsExpected: true,
		},
		{
			Description: "Test for GVK case sensitive",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{ "apiVersion": "v1", "kind": "Pod", "metadata": { "name": "myapp-pod2", "labels": { "app": "myapp2" } }, "spec": { "containers": [ { "name": "nginx", "image": "nginx" } ] } }`),
			Policy:            []byte(`{ "apiVersion": "kyverno.io/v1", "kind": "ClusterPolicy", "metadata": { "name": "disallow-latest-tag", "annotations": { "policies.kyverno.io/category": "Workload Isolation", "policies.kyverno.io/description": "The ':latest' tag is mutable and can lead to unexpected errors if the image changes. A best practice is to use an immutable tag that maps to a specific version of an application pod." } }, "spec": { "rules": [ { "name": "require-image-tag", "match": { "resources": { "kinds": [ "pod" ] } }, "validate": { "failureAction": "enforce", "message": "An image tag is required", "pattern": { "spec": { "containers": [ { "image": "*:*" } ] } } } } ] } }`),
			areErrorsExpected: true,
		},
		{
			Description: "Test should fail for GVK case sensitive",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{ "apiVersion": "apps/v1", "kind": "Deployment", "metadata": { "creationTimestamp": "2020-09-21T12:56:35Z", "name": "qos-demo", "labels": { "test": "qos" } }, "spec": { "replicas": 1, "selector": { "matchLabels": { "app": "nginx" } }, "template": { "metadata": { "creationTimestamp": "2020-09-21T12:56:35Z", "labels": { "app": "nginx" } }, "spec": { "containers": [ { "name": "nginx", "image": "nginx:latest", "resources": { "limits": { "cpu": "50m" } } } ]}}}}`),
			Policy:            []byte(`{ "apiVersion": "kyverno.io/v1", "kind": "ClusterPolicy", "metadata": { "name": "policy-qos" }, "spec": { "rules": [ { "name": "add-memory-limit", "match": { "resources": { "kinds": [ "apps/v1/deployment" ], "selector": { "matchLabels": { "test": "qos" } } } }, "mutate": { "overlay": { "spec": { "template": { "spec": { "containers": [ { "(name)": "*", "resources": { "limits": { "+(memory)": "300Mi", "+(cpu)": "100" } } } ] } } } } } } ] } }`),
			areErrorsExpected: true,
		},
	}

	for i, tc := range tcs {
		var policy v1.Policy
		err := json.Unmarshal(tc.Policy, &policy)
		if err != nil {
			t.Errorf("Testcase %d invalid policy raw", i+1)
		}
		resource, _ := kubeutils.BytesToUnstructured(tc.Resource)

		for _, rule := range autogen.Default.ComputeRules(&policy, "") {
			err := MatchesResourceDescription(*resource, rule, tc.AdmissionInfo, nil, "", resource.GroupVersionKind(), "", "CREATE")
			if err != nil {
				if !tc.areErrorsExpected {
					t.Errorf("Testcase %d Unexpected error: %v\nmsg: %s", i+1, err, tc.Description)
				}
			} else {
				if tc.areErrorsExpected {
					t.Errorf("Testcase %d Expected Error but received no error", i+1)
				}
			}
		}
	}
}

func TestMatchesResourceDescription_GenerateName(t *testing.T) {
	tcs := []struct {
		Description       string
		AdmissionInfo     v2.RequestInfo
		Resource          []byte
		Policy            []byte
		areErrorsExpected bool
	}{
		{
			Description: "Match Any matches the Pod",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"generateName": "abc",
					"namespace" : "prod"
				},
				"spec": {
					"containers": [
						{
							"name": "cont-name",
							"image": "cont-img",
							"ports": [
								{
									"containerPort": 81
								}
							],
							"resources": {
								"limits": {
									"memory": "30Mi",
									"cpu": "0.2"
								},
								"requests": {
									"memory": "20Mi",
									"cpu": "0.1"
								}
							}
						}
					]
				}
			}`),
			Policy: []byte(`{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "test-policy"
				},
				"spec": {
					"background": false,
					"rules": [
						{
							"name": "any-match-rule",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"names" : ["dev"]
										}
									},
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"namespaces" : ["prod"]
										}
									}
								]
							},
							"mutate": {
								"overlay": {
									"spec": {
										"containers": [
											{
												"(image)": "*",
												"imagePullPolicy": "IfNotPresent"
											}
										]
									}
								}
							}
						}
					]
				}
			}`),
			areErrorsExpected: false,
		},
		{
			Description: "Match Any does not match the Pod",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"generateName": "abc",
					"namespace" : "default"
				},
				"spec": {
					"containers": [
						{
							"name": "cont-name",
							"image": "cont-img",
							"ports": [
								{
									"containerPort": 81
								}
							],
							"resources": {
								"limits": {
									"memory": "30Mi",
									"cpu": "0.2"
								},
								"requests": {
									"memory": "20Mi",
									"cpu": "0.1"
								}
							}
						}
					]
				}
			}`),
			Policy: []byte(`{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "test-policy"
				},
				"spec": {
					"background": false,
					"rules": [
						{
							"name": "test-rule",
							"match": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"names" : ["dev"]
										}
									},
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"namespaces" : ["prod"]
										}
									}
								]
							},
							"mutate": {
								"overlay": {
									"spec": {
										"containers": [
											{
												"(image)": "*",
												"imagePullPolicy": "IfNotPresent"
											}
										]
									}
								}
							}
						}
					]
				}
			}`),
			areErrorsExpected: true,
		},
		{
			Description: "Match All matches the Pod",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"generateName": "abc",
					"namespace" : "prod"
				},
				"spec": {
					"containers": [
						{
							"name": "cont-name",
							"image": "cont-img",
							"ports": [
								{
									"containerPort": 81
								}
							],
							"resources": {
								"limits": {
									"memory": "30Mi",
									"cpu": "0.2"
								},
								"requests": {
									"memory": "20Mi",
									"cpu": "0.1"
								}
							}
						}
					]
				}
			}`),
			Policy: []byte(`{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "test-policy"
				},
				"spec": {
					"background": false,
					"rules": [
						{
							"name": "test-rule",
							"match": {
								"all": [
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"names" : ["abc"]
										}
									},
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"namespaces" : ["prod"]
										}
									}
								]
							},
							"mutate": {
								"overlay": {
									"spec": {
										"containers": [
											{
												"(image)": "*",
												"imagePullPolicy": "IfNotPresent"
											}
										]
									}
								}
							}
						}
					]
				}
			}`),
			areErrorsExpected: false,
		},
		{
			Description: "Match All does not match the Pod",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"generateName": "abc",
					"namespace" : "prod"
				},
				"spec": {
					"containers": [
						{
							"name": "cont-name",
							"image": "cont-img",
							"ports": [
								{
									"containerPort": 81
								}
							],
							"resources": {
								"limits": {
									"memory": "30Mi",
									"cpu": "0.2"
								},
								"requests": {
									"memory": "20Mi",
									"cpu": "0.1"
								}
							}
						}
					]
				}
			}`),
			Policy: []byte(`{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "test-policy"
				},
				"spec": {
					"background": false,
					"rules": [
						{
							"name": "test-rule",
							"match": {
								"all": [
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"names" : ["xyz"]
										}
									},
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"namespaces" : ["prod"]
										}
									}
								]
							},
							"mutate": {
								"overlay": {
									"spec": {
										"containers": [
											{
												"(image)": "*",
												"imagePullPolicy": "IfNotPresent"
											}
										]
									}
								}
							}
						}
					]
				}
			}`),
			areErrorsExpected: true,
		},
		{
			Description: "Exclude Any excludes the Pod",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"generateName": "dev",
					"namespace" : "prod"
				},
				"spec": {
					"containers": [
						{
							"name": "cont-name",
							"image": "cont-img",
							"ports": [
								{
									"containerPort": 81
								}
							],
							"resources": {
								"limits": {
									"memory": "30Mi",
									"cpu": "0.2"
								},
								"requests": {
									"memory": "20Mi",
									"cpu": "0.1"
								}
							}
						}
					]
				}
			}`),
			Policy: []byte(`{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "test-policy"
				},
				"spec": {
					"background": false,
					"rules": [
						{
							"name": "test-rule",
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
							"exclude": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"names": [
												"dev"
											]
										}
									},
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"namespaces": [
												"default"
											]
										}
									}
								]
							},
							"mutate": {
								"overlay": {
									"spec": {
										"containers": [
											{
												"(image)": "*",
												"imagePullPolicy": "IfNotPresent"
											}
										]
									}
								}
							}
						}
					]
				}
			}`),
			areErrorsExpected: true,
		},
		{
			Description: "Exclude Any does not exclude the Pod",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"generateName": "abc",
					"namespace" : "prod"
				},
				"spec": {
					"containers": [
						{
							"name": "cont-name",
							"image": "cont-img",
							"ports": [
								{
									"containerPort": 81
								}
							],
							"resources": {
								"limits": {
									"memory": "30Mi",
									"cpu": "0.2"
								},
								"requests": {
									"memory": "20Mi",
									"cpu": "0.1"
								}
							}
						}
					]
				}
			}`),
			Policy: []byte(`{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "test-policy"
				},
				"spec": {
					"background": false,
					"rules": [
						{
							"name": "test-rule",
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
							"exclude": {
								"any": [
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"names": [
												"dev"
											]
										}
									},
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"namespaces": [
												"default"
											]
										}
									}
								]
							},
							"mutate": {
								"overlay": {
									"spec": {
										"containers": [
											{
												"(image)": "*",
												"imagePullPolicy": "IfNotPresent"
											}
										]
									}
								}
							}
						}
					]
				}
			}`),
			areErrorsExpected: false,
		},
		{
			Description: "Exclude All excludes the Pod",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"generateName": "dev",
					"namespace" : "prod"
				},
				"spec": {
					"containers": [
						{
							"name": "cont-name",
							"image": "cont-img",
							"ports": [
								{
									"containerPort": 81
								}
							],
							"resources": {
								"limits": {
									"memory": "30Mi",
									"cpu": "0.2"
								},
								"requests": {
									"memory": "20Mi",
									"cpu": "0.1"
								}
							}
						}
					]
				}
			}`),
			Policy: []byte(`{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "test-policy"
				},
				"spec": {
					"background": false,
					"rules": [
						{
							"name": "test-rule",
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
							"exclude": {
								"all": [
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"names": [
												"dev"
											]
										}
									},
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"namespaces": [
												"prod"
											]
										}
									}
								]
							},
							"mutate": {
								"overlay": {
									"spec": {
										"containers": [
											{
												"(image)": "*",
												"imagePullPolicy": "IfNotPresent"
											}
										]
									}
								}
							}
						}
					]
				}
			}`),
			areErrorsExpected: true,
		},
		{
			Description: "Exclude All does not exclude the Pod",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"generateName": "abc",
					"namespace" : "prod"
				},
				"spec": {
					"containers": [
						{
							"name": "cont-name",
							"image": "cont-img",
							"ports": [
								{
									"containerPort": 81
								}
							],
							"resources": {
								"limits": {
									"memory": "30Mi",
									"cpu": "0.2"
								},
								"requests": {
									"memory": "20Mi",
									"cpu": "0.1"
								}
							}
						}
					]
				}
			}`),
			Policy: []byte(`{
				"apiVersion": "kyverno.io/v1",
				"kind": "ClusterPolicy",
				"metadata": {
					"name": "test-policy"
				},
				"spec": {
					"background": false,
					"rules": [
						{
							"name": "test-rule",
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
							"exclude": {
								"all": [
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"names": [
												"abc"
											]
										}
									},
									{
										"resources": {
											"kinds": [
												"Pod"
											],
											"namespaces": [
												"default"
											]
										}
									}
								]
							},
							"mutate": {
								"overlay": {
									"spec": {
										"containers": [
											{
												"(image)": "*",
												"imagePullPolicy": "IfNotPresent"
											}
										]
									}
								}
							}
						}
					]
				}
			}`),
			areErrorsExpected: false,
		},
		{
			Description: "Should match pod and not exclude it",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"generateName":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"name":"hello-world"},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: false,
		},
		{
			Description: "Should exclude resource since it matches the exclude block",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"system:node"},
			},
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"generateName":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"name":"hello-world"},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: true,
		},
		{
			Description:       "Should not fail if in sync mode, if admission info is empty it should still match resources with specific clusterRoles",
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"generateName":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"name":"hello-world"},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: false,
		},
		{
			Description:       "Should fail since resource does not match because of names field",
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"generateName":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"],"names": ["dev-*"]},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: true,
		},
		{
			Description: "Should pass since resource matches a name in the names field",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"system:node"},
			},
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"generateName":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"],"names": ["dev-*","hello-world"]},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: false,
		},
		{
			Description:       "Should fail since resource gets excluded because of the names field",
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"generateName":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"names": ["dev-*","hello-*"]}},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: true,
		},
		{
			Description:       "Should pass since resource does not get excluded because of the names field",
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"generateName":"bye-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"names": ["dev-*","hello-*"]}},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: false,
		},
		{
			Description: "Should fail since resource does not match policy",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{"apiVersion":"v1","kind":"Service","metadata":{"generateName":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"name":"hello-world"},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: true,
		},
		{
			Description: "Should not fail since resource does not match exclude block",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"system:node"},
			},
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"generateName":"hello-world2","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"name":"hello-world"},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: false,
		},
		{
			Description: "Should pass since group, version, kind match",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{ "apiVersion": "apps/v1", "kind": "Deployment", "metadata": { "creationTimestamp": "2020-09-21T12:56:35Z", "generateName": "qos-demo", "labels": { "test": "qos" } }, "spec": { "replicas": 1, "selector": { "matchLabels": { "app": "nginx" } }, "template": { "metadata": { "creationTimestamp": "2020-09-21T12:56:35Z", "labels": { "app": "nginx" } }, "spec": { "containers": [ { "name": "nginx", "image": "nginx:latest", "resources": { "limits": { "cpu": "50m" } } } ]}}}}`),
			Policy:            []byte(`{ "apiVersion": "kyverno.io/v1", "kind": "ClusterPolicy", "metadata": { "name": "policy-qos" }, "spec": { "rules": [ { "name": "add-memory-limit", "match": { "resources": { "kinds": [ "apps/v1/Deployment" ], "selector": { "matchLabels": { "test": "qos" } } } }, "mutate": { "overlay": { "spec": { "template": { "spec": { "containers": [ { "(name)": "*", "resources": { "limits": { "+(memory)": "300Mi", "+(cpu)": "100" } } } ] } } } } } }, { "name": "check-cpu-memory-limits", "match": { "resources": { "kinds": [ "apps/v1/Deployment" ], "selector": { "matchLabels": { "test": "qos" } } } }, "validate": { "failureAction": "enforce", "message": "Resource limits are required for CPU and memory", "pattern": { "spec": { "template": { "spec": { "containers": [ { "(name)": "*", "resources": { "limits": { "memory": "?*", "cpu": "?*" } } } ] } } } } } } ] } }`),
			areErrorsExpected: false,
		},
		{
			Description: "Should pass since version and kind match",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{ "apiVersion": "v1", "kind": "Pod", "metadata": { "generateName": "myapp-pod2", "labels": { "app": "myapp2" } }, "spec": { "containers": [ { "name": "nginx", "image": "nginx" } ] } }`),
			Policy:            []byte(`{ "apiVersion": "kyverno.io/v1", "kind": "ClusterPolicy", "metadata": { "name": "disallow-latest-tag", "annotations": { "policies.kyverno.io/category": "Workload Isolation", "policies.kyverno.io/description": "The ':latest' tag is mutable and can lead to unexpected errors if the image changes. A best practice is to use an immutable tag that maps to a specific version of an application pod." } }, "spec": { "rules": [ { "name": "require-image-tag", "match": { "resources": { "kinds": [ "v1/Pod" ] } }, "validate": { "failureAction": "enforce", "message": "An image tag is required", "pattern": { "spec": { "containers": [ { "image": "*:*" } ] } } } } ] } }`),
			areErrorsExpected: false,
		},
		{
			Description: "Should fail since resource does not match ",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{"apiVersion":"v1","kind":"Service","metadata":{"generateName":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"name":"hello-world"},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: true,
		},
		{
			Description: "Should fail since version not match",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{ "apiVersion": "apps/v1beta1", "kind": "Deployment", "metadata": { "creationTimestamp": "2020-09-21T12:56:35Z", "generateName": "qos-demo", "labels": { "test": "qos" } }, "spec": { "replicas": 1, "selector": { "matchLabels": { "app": "nginx" } }, "template": { "metadata": { "creationTimestamp": "2020-09-21T12:56:35Z", "labels": { "app": "nginx" } }, "spec": { "containers": [ { "name": "nginx", "image": "nginx:latest", "resources": { "limits": { "cpu": "50m" } } } ]}}}}`),
			Policy:            []byte(`{ "apiVersion": "kyverno.io/v1", "kind": "ClusterPolicy", "metadata": { "name": "policy-qos" }, "spec": { "rules": [ { "name": "add-memory-limit", "match": { "resources": { "kinds": [ "apps/v1/Deployment" ], "selector": { "matchLabels": { "test": "qos" } } } }, "mutate": { "overlay": { "spec": { "template": { "spec": { "containers": [ { "(name)": "*", "resources": { "limits": { "+(memory)": "300Mi", "+(cpu)": "100" } } } ] } } } } } }, { "name": "check-cpu-memory-limits", "match": { "resources": { "kinds": [ "apps/v1/Deployment" ], "selector": { "matchLabels": { "test": "qos" } } } }, "validate": { "failureAction": "enforce", "message": "Resource limits are required for CPU and memory", "pattern": { "spec": { "template": { "spec": { "containers": [ { "(name)": "*", "resources": { "limits": { "memory": "?*", "cpu": "?*" } } } ] } } } } } } ] } }`),
			areErrorsExpected: true,
		},
		{
			Description: "Should fail since cluster role version not match",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{ "kind": "ClusterRole", "apiVersion": "rbac.authorization.k8s.io/v1", "metadata": { "generateName": "secret-reader-demo", "namespace": "default" }, "rules": [ { "apiGroups": [ "" ], "resources": [ "secrets" ], "verbs": [ "get", "watch", "list" ] } ] }`),
			Policy:            []byte(`{ "apiVersion": "kyverno.io/v1", "kind": "ClusterPolicy", "metadata": { "name": "check-host-path" }, "spec": { "background": true, "rules": [ { "name": "check-host-path", "match": { "resources": { "kinds": [ "rbac.authorization.k8s.io/v1beta1/ClusterRole" ] } }, "validate": { "failureAction": "enforce", "message": "Host path is not allowed", "pattern": { "spec": { "volumes": [ { "name": "*", "hostPath": { "path": "" } } ] } } } } ] } }`),
			areErrorsExpected: true,
		},
		{
			Description: "Test for GVK case sensitive",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{ "apiVersion": "v1", "kind": "Pod", "metadata": { "generateName": "myapp-pod2", "labels": { "app": "myapp2" } }, "spec": { "containers": [ { "name": "nginx", "image": "nginx" } ] } }`),
			Policy:            []byte(`{ "apiVersion": "kyverno.io/v1", "kind": "ClusterPolicy", "metadata": { "name": "disallow-latest-tag", "annotations": { "policies.kyverno.io/category": "Workload Isolation", "policies.kyverno.io/description": "The ':latest' tag is mutable and can lead to unexpected errors if the image changes. A best practice is to use an immutable tag that maps to a specific version of an application pod." } }, "spec": { "rules": [ { "name": "require-image-tag", "match": { "resources": { "kinds": [ "pod" ] } }, "validate": { "failureAction": "enforce", "message": "An image tag is required", "pattern": { "spec": { "containers": [ { "image": "*:*" } ] } } } } ] } }`),
			areErrorsExpected: true,
		},
		{
			Description: "Test should fail for GVK case sensitive",
			AdmissionInfo: v2.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{ "apiVersion": "apps/v1", "kind": "Deployment", "metadata": { "creationTimestamp": "2020-09-21T12:56:35Z", "generateName": "qos-demo", "labels": { "test": "qos" } }, "spec": { "replicas": 1, "selector": { "matchLabels": { "app": "nginx" } }, "template": { "metadata": { "creationTimestamp": "2020-09-21T12:56:35Z", "labels": { "app": "nginx" } }, "spec": { "containers": [ { "name": "nginx", "image": "nginx:latest", "resources": { "limits": { "cpu": "50m" } } } ]}}}}`),
			Policy:            []byte(`{ "apiVersion": "kyverno.io/v1", "kind": "ClusterPolicy", "metadata": { "name": "policy-qos" }, "spec": { "rules": [ { "name": "add-memory-limit", "match": { "resources": { "kinds": [ "apps/v1/deployment" ], "selector": { "matchLabels": { "test": "qos" } } } }, "mutate": { "overlay": { "spec": { "template": { "spec": { "containers": [ { "(name)": "*", "resources": { "limits": { "+(memory)": "300Mi", "+(cpu)": "100" } } } ] } } } } } } ] } }`),
			areErrorsExpected: true,
		},
	}

	for i, tc := range tcs {
		var policy v1.Policy
		err := json.Unmarshal(tc.Policy, &policy)
		if err != nil {
			t.Errorf("Testcase %d invalid policy raw", i+1)
		}
		resource, _ := kubeutils.BytesToUnstructured(tc.Resource)

		for _, rule := range autogen.Default.ComputeRules(&policy, "") {
			err := MatchesResourceDescription(*resource, rule, tc.AdmissionInfo, nil, "", resource.GroupVersionKind(), "", "CREATE")
			if err != nil {
				if !tc.areErrorsExpected {
					t.Errorf("Testcase %d Unexpected error: %v\nmsg: %s", i+1, err, tc.Description)
				}
			} else {
				if tc.areErrorsExpected {
					t.Errorf("Testcase %d Expected Error but received no error", i+1)
				}
			}
		}
	}
}

// Match multiple kinds
func TestResourceDescriptionMatch_MultipleKind(t *testing.T) {
	rawResource := []byte(`{
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
					   "image": "nginx:1.7.9",
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
	 }`)
	resource, err := kubeutils.BytesToUnstructured(rawResource)
	if err != nil {
		t.Errorf("unable to convert raw resource to unstructured: %v", err)
	}
	resourceDescription := v1.ResourceDescription{
		Kinds: []string{"Deployment", "Pods"},
		Selector: &metav1.LabelSelector{
			MatchLabels:      nil,
			MatchExpressions: nil,
		},
	}
	rule := v1.Rule{MatchResources: v1.MatchResources{ResourceDescription: resourceDescription}}

	if err := MatchesResourceDescription(*resource, rule, v2.RequestInfo{}, nil, "", resource.GroupVersionKind(), "", "CREATE"); err != nil {
		t.Errorf("Testcase has failed due to the following:%v", err)
	}
}

func TestResourceDescriptionMatch_ExcludeDefaultGroups(t *testing.T) {

	// slightly simplified ingress controller pod that lives in the ingress-nginx namespace
	rawResource := []byte(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "ingress-nginx-controller-57bc6474bb-mcpt2",
			"namespace": "ingress-nginx"
		},
		"spec": {
			"containers": [
				{
					"args": ["/nginx-ingress-controller"],
					"env": [],
					"image": "registry.k8s.io/ingress-nginx/controller:v1.5.1",
					"imagePullPolicy": "IfNotPresent",
					"name": "controller",
					"securityContext": {
						"allowPrivilegeEscalation": true,
						"capabilities": {
							"add": [
								"NET_BIND_SERVICE"
							],
							"drop": [
								"ALL"
							]
						},
						"runAsUser": 101
					},
					"volumeMounts": [
						{
							"mountPath": "/usr/local/certificates/",
							"name": "webhook-cert",
							"readOnly": true
						},
						{
							"mountPath": "/var/run/secrets/kubernetes.io/serviceaccount",
							"name": "kube-api-access-vc2qz",
							"readOnly": true
						}
					]
				}
			],
			"securityContext": {},
			"serviceAccount": "ingress-nginx",
			"serviceAccountName": "ingress-nginx"
		}
	}`)
	resource, err := kubeutils.BytesToUnstructured(rawResource)
	if err != nil {
		t.Errorf("unable to convert raw resource to unstructured: %v", err)
	}

	// this rule should match only pods in the user1-restricted namespace, and also pods of User:user1.
	rule := v1.Rule{
		MatchResources: v1.MatchResources{Any: v1.ResourceFilters{
			// pods in user1-restricted namespace
			v1.ResourceFilter{
				ResourceDescription: v1.ResourceDescription{
					Kinds: []string{"Pod"},
					Namespaces: []string{
						"user1-restricted",
					},
				},
			},
			// pods for User user1 account
			v1.ResourceFilter{
				ResourceDescription: v1.ResourceDescription{
					Kinds: []string{"Pod"},
				},
				UserInfo: v1.UserInfo{
					Subjects: []rbacv1.Subject{
						{
							Kind: "User",
							Name: "user1",
						},
					},
				},
			},
		}},
		ExcludeResources: &v1.MatchResources{},
	}

	// this is the request info that was also passed with the mocked pod
	requestInfo := v2.RequestInfo{
		AdmissionUserInfo: authenticationv1.UserInfo{
			Username: "system:serviceaccount:kube-system:replicaset-controller",
			UID:      "8f36cad4-eb68-4931-bea8-8a42dd1fee4c",
			Groups: []string{
				"system:serviceaccounts",
				"system:serviceaccounts:kube-system",
				"system:authenticated",
			},
		},
	}

	// First test: confirm that this above rule produces errors (and raise an error if err == nil)
	if err := MatchesResourceDescription(*resource, rule, requestInfo, nil, "", resource.GroupVersionKind(), "", "CREATE"); err == nil {
		t.Error("Testcase was expected to fail, but err was nil")
	}

	// This next rule *should* match, because we explicitly match the ingress-nginx namespace this time.
	rule2 := v1.Rule{
		MatchResources: v1.MatchResources{Any: v1.ResourceFilters{
			v1.ResourceFilter{
				ResourceDescription: v1.ResourceDescription{
					Kinds: []string{"Pod"},
					Namespaces: []string{
						"ingress-nginx",
					},
				},
			},
		}},
		ExcludeResources: &v1.MatchResources{Any: v1.ResourceFilters{}},
	}

	// Second test: confirm that matching this rule does not create any errors (and raise if err != nil)
	if err := MatchesResourceDescription(*resource, rule2, requestInfo, nil, "", resource.GroupVersionKind(), "", "CREATE"); err != nil {
		t.Errorf("Testcase was expected to not fail, but err was %s", err)
	}

	// Now we extend the previous rule to have an Exclude part. Making it 'not-empty' should make the exclude-code run.
	rule2.ExcludeResources = &v1.MatchResources{Any: v1.ResourceFilters{
		v1.ResourceFilter{
			ResourceDescription: v1.ResourceDescription{
				Kinds: []string{"Pod"},
			},
		},
	}}

	// Third test: confirm that now the custom exclude-snippet should run in CheckSubjects() and that should result in this rule failing (raise if err == nil for that reason)
	if err := MatchesResourceDescription(*resource, rule2, requestInfo, nil, "", resource.GroupVersionKind(), "", "CREATE"); err == nil {
		t.Error("Testcase was expected to fail, but err was nil #1!")
	}
}

// Match resource name
func TestResourceDescriptionMatch_Name(t *testing.T) {
	rawResource := []byte(`{
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
					   "image": "nginx:1.7.9",
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
	 }`)
	resource, err := kubeutils.BytesToUnstructured(rawResource)
	if err != nil {
		t.Errorf("unable to convert raw resource to unstructured: %v", err)
	}
	resourceDescription := v1.ResourceDescription{
		Kinds: []string{"Deployment"},
		Name:  "nginx-deployment",
		Selector: &metav1.LabelSelector{
			MatchLabels:      nil,
			MatchExpressions: nil,
		},
	}
	rule := v1.Rule{MatchResources: v1.MatchResources{ResourceDescription: resourceDescription}}

	if err := MatchesResourceDescription(*resource, rule, v2.RequestInfo{}, nil, "", resource.GroupVersionKind(), "", "CREATE"); err != nil {
		t.Errorf("Testcase has failed due to the following:%v", err)
	}
}

func TestResourceDescriptionMatch_GenerateName(t *testing.T) {
	rawResource := []byte(`{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
		   "generateName": "nginx-deployment",
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
					   "image": "nginx:1.7.9",
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
	 }`)
	resource, err := kubeutils.BytesToUnstructured(rawResource)
	if err != nil {
		t.Errorf("unable to convert raw resource to unstructured: %v", err)
	}
	resourceDescription := v1.ResourceDescription{
		Kinds: []string{"Deployment"},
		Name:  "nginx-deployment",
		Selector: &metav1.LabelSelector{
			MatchLabels:      nil,
			MatchExpressions: nil,
		},
	}
	rule := v1.Rule{MatchResources: v1.MatchResources{ResourceDescription: resourceDescription}}

	if err := MatchesResourceDescription(*resource, rule, v2.RequestInfo{}, nil, "", resource.GroupVersionKind(), "", "CREATE"); err != nil {
		t.Errorf("Testcase has failed due to the following:%v", err)
	}
}

// Match resource regex
func TestResourceDescriptionMatch_Name_Regex(t *testing.T) {
	rawResource := []byte(`{
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
					   "image": "nginx:1.7.9",
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
	 }`)
	resource, err := kubeutils.BytesToUnstructured(rawResource)
	if err != nil {
		t.Errorf("unable to convert raw resource to unstructured: %v", err)
	}
	resourceDescription := v1.ResourceDescription{
		Kinds: []string{"Deployment"},
		Name:  "nginx-*",
		Selector: &metav1.LabelSelector{
			MatchLabels:      nil,
			MatchExpressions: nil,
		},
	}
	rule := v1.Rule{MatchResources: v1.MatchResources{ResourceDescription: resourceDescription}}

	if err := MatchesResourceDescription(*resource, rule, v2.RequestInfo{}, nil, "", resource.GroupVersionKind(), "", "CREATE"); err != nil {
		t.Errorf("Testcase has failed due to the following:%v", err)
	}
}

func TestResourceDescriptionMatch_GenerateName_Regex(t *testing.T) {
	rawResource := []byte(`{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
		   "generateName": "nginx-deployment",
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
					   "image": "nginx:1.7.9",
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
	 }`)
	resource, err := kubeutils.BytesToUnstructured(rawResource)
	if err != nil {
		t.Errorf("unable to convert raw resource to unstructured: %v", err)
	}
	resourceDescription := v1.ResourceDescription{
		Kinds: []string{"Deployment"},
		Name:  "nginx-*",
		Selector: &metav1.LabelSelector{
			MatchLabels:      nil,
			MatchExpressions: nil,
		},
	}
	rule := v1.Rule{MatchResources: v1.MatchResources{ResourceDescription: resourceDescription}}

	if err := MatchesResourceDescription(*resource, rule, v2.RequestInfo{}, nil, "", resource.GroupVersionKind(), "", "CREATE"); err != nil {
		t.Errorf("Testcase has failed due to the following:%v", err)
	}
}

// Match expressions for labels to not match
func TestResourceDescriptionMatch_Label_Expression_NotMatch(t *testing.T) {
	rawResource := []byte(`{
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
					   "image": "nginx:1.7.9",
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
	 }`)
	resource, err := kubeutils.BytesToUnstructured(rawResource)
	if err != nil {
		t.Errorf("unable to convert raw resource to unstructured: %v", err)
	}
	resourceDescription := v1.ResourceDescription{
		Kinds: []string{"Deployment"},
		Name:  "nginx-*",
		Selector: &metav1.LabelSelector{
			MatchLabels: nil,
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "label2",
					Operator: "NotIn",
					Values: []string{
						"sometest1",
					},
				},
			},
		},
	}
	rule := v1.Rule{MatchResources: v1.MatchResources{ResourceDescription: resourceDescription}}

	if err := MatchesResourceDescription(*resource, rule, v2.RequestInfo{}, nil, "", resource.GroupVersionKind(), "", "CREATE"); err != nil {
		t.Errorf("Testcase has failed due to the following:%v", err)
	}
}

// Match label expression in matching set
func TestResourceDescriptionMatch_Label_Expression_Match(t *testing.T) {
	rawResource := []byte(`{
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
					   "image": "nginx:1.7.9",
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
	 }`)
	resource, err := kubeutils.BytesToUnstructured(rawResource)
	if err != nil {
		t.Errorf("unable to convert raw resource to unstructured: %v", err)
	}
	resourceDescription := v1.ResourceDescription{
		Kinds: []string{"Deployment"},
		Name:  "nginx-*",
		Selector: &metav1.LabelSelector{
			MatchLabels: nil,
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "app",
					Operator: "NotIn",
					Values: []string{
						"nginx1",
						"nginx2",
					},
				},
			},
		},
	}
	rule := v1.Rule{MatchResources: v1.MatchResources{ResourceDescription: resourceDescription}}

	if err := MatchesResourceDescription(*resource, rule, v2.RequestInfo{}, nil, "", resource.GroupVersionKind(), "", "CREATE"); err != nil {
		t.Errorf("Testcase has failed due to the following:%v", err)
	}
}

// check for exclude conditions
func TestResourceDescriptionExclude_Label_Expression_Match(t *testing.T) {
	rawResource := []byte(`{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
		   "name": "nginx-deployment",
		   "labels": {
			  "app": "nginx",
			  "block": "true"
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
					   "image": "nginx:1.7.9",
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
	 }`)
	resource, err := kubeutils.BytesToUnstructured(rawResource)
	if err != nil {
		t.Errorf("unable to convert raw resource to unstructured: %v", err)
	}
	resourceDescription := v1.ResourceDescription{
		Kinds: []string{"Deployment"},
		Name:  "nginx-*",
		Selector: &metav1.LabelSelector{
			MatchLabels: nil,
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "app",
					Operator: "NotIn",
					Values: []string{
						"nginx1",
						"nginx2",
					},
				},
			},
		},
	}

	resourceDescriptionExclude := v1.ResourceDescription{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"block": "true",
			},
		},
	}

	rule := v1.Rule{
		MatchResources:   v1.MatchResources{ResourceDescription: resourceDescription},
		ExcludeResources: &v1.MatchResources{ResourceDescription: resourceDescriptionExclude},
	}

	if err := MatchesResourceDescription(*resource, rule, v2.RequestInfo{}, nil, "", resource.GroupVersionKind(), "", "CREATE"); err == nil {
		t.Errorf("Testcase has failed due to the following:\n Function has returned no error, even though it was supposed to fail")
	}
}
