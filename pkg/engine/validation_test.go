package engine

import (
	"encoding/json"
	"testing"

	kubepolicy "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWrappedWithParentheses_StringIsWrappedWithParentheses(t *testing.T) {
	str := "(something)"
	assert.Assert(t, wrappedWithParentheses(str))
}

func TestWrappedWithParentheses_StringHasOnlyParentheses(t *testing.T) {
	str := "()"
	assert.Assert(t, wrappedWithParentheses(str))
}

func TestWrappedWithParentheses_StringHasNoParentheses(t *testing.T) {
	str := "something"
	assert.Assert(t, !wrappedWithParentheses(str))
}

func TestWrappedWithParentheses_StringHasLeftParentheses(t *testing.T) {
	str := "(something"
	assert.Assert(t, !wrappedWithParentheses(str))
}

func TestWrappedWithParentheses_StringHasRightParentheses(t *testing.T) {
	str := "something)"
	assert.Assert(t, !wrappedWithParentheses(str))
}

func TestWrappedWithParentheses_StringParenthesesInside(t *testing.T) {
	str := "so)m(et(hin)g"
	assert.Assert(t, !wrappedWithParentheses(str))
}

func TestWrappedWithParentheses_Empty(t *testing.T) {
	str := ""
	assert.Assert(t, !wrappedWithParentheses(str))
}

func TestValidateString_AsteriskTest(t *testing.T) {
	pattern := "*"
	value := "anything"
	empty := ""

	assert.Assert(t, validateString(value, pattern, Equal))
	assert.Assert(t, validateString(empty, pattern, Equal))
}

func TestValidateString_LeftAsteriskTest(t *testing.T) {
	pattern := "*right"
	value := "leftright"
	right := "right"

	assert.Assert(t, validateString(value, pattern, Equal))
	assert.Assert(t, validateString(right, pattern, Equal))

	value = "leftmiddle"
	middle := "middle"

	assert.Assert(t, !validateString(value, pattern, Equal))
	assert.Assert(t, !validateString(middle, pattern, Equal))
}

func TestValidateString_MiddleAsteriskTest(t *testing.T) {
	pattern := "ab*ba"
	value := "abbeba"
	assert.Assert(t, validateString(value, pattern, Equal))

	value = "abbca"
	assert.Assert(t, !validateString(value, pattern, Equal))
}

func TestValidateString_QuestionMark(t *testing.T) {
	pattern := "ab?ba"
	value := "abbba"
	assert.Assert(t, validateString(value, pattern, Equal))

	value = "abbbba"
	assert.Assert(t, !validateString(value, pattern, Equal))
}

func TestSkipArrayObject_OneAnchor(t *testing.T) {

	rawAnchors := []byte(`{
		"(name)":"nirmata-*"
	}`)
	rawResource := []byte(`{
		"name":"nirmata-resource",
		"namespace":"kyverno",
		"object":{
			"label":"app",
			"array":[
				1,
				2,
				3
			]
		}
	}`)

	var resource, anchor map[string]interface{}

	json.Unmarshal(rawAnchors, &anchor)
	json.Unmarshal(rawResource, &resource)

	assert.Assert(t, !skipArrayObject(resource, anchor))
}

func TestSkipArrayObject_OneNumberAnchorPass(t *testing.T) {

	rawAnchors := []byte(`{
		"(count)":1
	}`)
	rawResource := []byte(`{
		"name":"nirmata-resource",
		"count":1,
		"namespace":"kyverno",
		"object":{
			"label":"app",
			"array":[
				1,
				2,
				3
			]
		}
	}`)

	var resource, anchor map[string]interface{}

	json.Unmarshal(rawAnchors, &anchor)
	json.Unmarshal(rawResource, &resource)

	assert.Assert(t, !skipArrayObject(resource, anchor))
}

func TestSkipArrayObject_TwoAnchorsPass(t *testing.T) {
	rawAnchors := []byte(`{
		"(name)":"nirmata-*",
		"(namespace)":"kyv?rno"
	}`)
	rawResource := []byte(`{
		"name":"nirmata-resource",
		"namespace":"kyverno",
		"object":{
			"label":"app",
			"array":[
				1,
				2,
				3
			]
		}
	}`)

	var resource, anchor map[string]interface{}

	json.Unmarshal(rawAnchors, &anchor)
	json.Unmarshal(rawResource, &resource)

	assert.Assert(t, !skipArrayObject(resource, anchor))
}

func TestSkipArrayObject_TwoAnchorsSkip(t *testing.T) {
	rawAnchors := []byte(`{
		"(name)":"nirmata-*",
		"(namespace)":"some-?olicy"
	}`)
	rawResource := []byte(`{
		"name":"nirmata-resource",
		"namespace":"kyverno",
		"object":{
			"label":"app",
			"array":[
				1,
				2,
				3
			]
		}
	}`)

	var resource, anchor map[string]interface{}

	json.Unmarshal(rawAnchors, &anchor)
	json.Unmarshal(rawResource, &resource)

	assert.Assert(t, skipArrayObject(resource, anchor))
}

func TestGetAnchorsFromMap_ThereAreAnchors(t *testing.T) {
	rawMap := []byte(`{
		"(name)":"nirmata-*",
		"notAnchor1":123,
		"(namespace)":"kube-?olicy",
		"notAnchor2":"sample-text",
		"object":{
			"key1":"value1",
			"(key2)":"value2"
		}
	}`)

	var unmarshalled map[string]interface{}
	json.Unmarshal(rawMap, &unmarshalled)

	actualMap := GetAnchorsFromMap(unmarshalled)
	assert.Equal(t, len(actualMap), 2)
	assert.Equal(t, actualMap["(name)"].(string), "nirmata-*")
	assert.Equal(t, actualMap["(namespace)"].(string), "kube-?olicy")
}

func TestGetAnchorsFromMap_ThereAreNoAnchors(t *testing.T) {
	rawMap := []byte(`{
		"name":"nirmata-*",
		"notAnchor1":123,
		"namespace":"kube-?olicy",
		"notAnchor2":"sample-text",
		"object":{
			"key1":"value1",
			"(key2)":"value2"
		}
	}`)

	var unmarshalled map[string]interface{}
	json.Unmarshal(rawMap, &unmarshalled)

	actualMap := GetAnchorsFromMap(unmarshalled)
	assert.Assert(t, len(actualMap) == 0)
}

func TestValidateMap(t *testing.T) {
	rawPattern := []byte(`{
		"spec":{
			"template":{
				"spec":{
					"containers":[
						{
							"name":"?*",
							"resources":{
								"requests":{
									"cpu":"<4|8"
								}
							}
						}
					]
				}
			}
		}
	}`)
	rawMap := []byte(`{
		"apiVersion":"apps/v1",
		"kind":"Deployment",
		"metadata":{
			"name":"nginx-deployment",
			"labels":{
				"app":"nginx"
			}
		},
		"spec":{
			"replicas":3,
			"selector":{
				"matchLabels":{
					"app":"nginx"
				}
			},
			"template":{
				"metadata":{
					"labels":{
						"app":"nginx"
					}
				},
				"spec":{
					"securityContext":{
						"runAsNonRoot":true
					},
					"containers":[
						{
							"name":"nginx",
							"image":"https://nirmata/nginx:latest",
							"imagePullPolicy":"Always",
							"readinessProbe":{
								"exec":{
									"command":[
										"cat",
										"/tmp/healthy"
									]
								},
								"initialDelaySeconds":5,
								"periodSeconds":10
							},
							"livenessProbe":{
								"tcpSocket":{
									"port":8080
								},
								"initialDelaySeconds":15,
								"periodSeconds":11
							},
							"resources":{
								"limits":{
									"memory":"2Gi",
									"cpu":8
								},
								"requests":{
									"memory":"512Mi",
									"cpu":"8"
								}
							},
							"ports":[
								{
									"containerPort":80
								}
							]
						}
					]
				}
			}
		}
	}`)

	var pattern, resource map[string]interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	res := validateMap(resource, pattern, "/")
	assert.NilError(t, res.ToError())
}

func TestValidateMap_AsteriskForInt(t *testing.T) {
	rawPattern := []byte(`{
		"spec":{
			"template":{
				"spec":{
					"containers":[
						{
							"name":"*",
							"livenessProbe":{
								"periodSeconds":"*"
							}
						}
					]
				}
			}
		}
	}`)
	rawMap := []byte(`{
		"apiVersion":"apps/v1",
		"kind":"StatefulSet",
		"metadata":{
			"name":"game-web",
			"labels":{
				"originalLabel":"isHere"
			}
		},
		"spec":{
			"selector":{
				"matchLabels":{
					"app":"nginxo"
				}
			},
			"serviceName":"nginxo",
			"replicas":3,
			"template":{
				"metadata":{
					"labels":{
						"app":"nginxo"
					}
				},
				"spec":{
					"terminationGracePeriodSeconds":10,
					"containers":[
						{
							"name":"nginxo",
							"image":"k8s.gcr.io/nginx-but-no-slim:0.8",
							"ports":[
								{
									"containerPort":8780,
									"name":"webp"
								}
							],
							"volumeMounts":[
								{
									"name":"www",
									"mountPath":"/usr/share/nginxo/html"
								}
							],
							"livenessProbe":{
								"periodSeconds":11
							}
						}
					]
				}
			},
			"volumeClaimTemplates":[
				{
					"metadata":{
						"name":"www"
					},
					"spec":{
						"accessModes":[
							"ReadWriteOnce"
						],
						"storageClassName":"my-storage-class",
						"resources":{
							"requests":{
								"storage":"1Gi"
							}
						}
					}
				}
			]
		}
	}
	`)

	var pattern, resource map[string]interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	res := validateMap(resource, pattern, "/")
	assert.NilError(t, res.ToError())
}

func TestValidateMap_AsteriskForMap(t *testing.T) {
	rawPattern := []byte(`{
		"spec":{
			"template":{
				"spec":{
					"containers":[
						{
							"name":"*",
							"livenessProbe":"*"
						}
					]
				}
			}
		}
	}`)
	rawMap := []byte(`{
		"apiVersion":"apps/v1",
		"kind":"StatefulSet",
		"metadata":{
			"name":"game-web",
			"labels":{
				"originalLabel":"isHere"
			}
		},
		"spec":{
			"selector":{
				"matchLabels":{
					"app":"nginxo"
				}
			},
			"serviceName":"nginxo",
			"replicas":3,
			"template":{
				"metadata":{
					"labels":{
						"app":"nginxo"
					}
				},
				"spec":{
					"terminationGracePeriodSeconds":10,
					"containers":[
						{
							"name":"nginxo",
							"image":"k8s.gcr.io/nginx-but-no-slim:0.8",
							"ports":[
								{
									"containerPort":8780,
									"name":"webp"
								}
							],
							"volumeMounts":[
								{
									"name":"www",
									"mountPath":"/usr/share/nginxo/html"
								}
							],
							"livenessProbe":{
								"periodSeconds":11
							}
						}
					]
				}
			},
			"volumeClaimTemplates":[
				{
					"metadata":{
						"name":"www"
					},
					"spec":{
						"accessModes":[
							"ReadWriteOnce"
						],
						"storageClassName":"my-storage-class",
						"resources":{
							"requests":{
								"storage":"1Gi"
							}
						}
					}
				}
			]
		}
	}`)

	var pattern, resource map[string]interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	res := validateMap(resource, pattern, "/")
	assert.NilError(t, res.ToError())
}

func TestValidateMap_AsteriskForArray(t *testing.T) {
	rawPattern := []byte(`{
		"spec":{
			"template":{
				"spec":{
					"containers":"*"
				}
			}
		}
	}`)
	rawMap := []byte(`{
		"apiVersion":"apps/v1",
		"kind":"StatefulSet",
		"metadata":{
			"name":"game-web",
			"labels":{
				"originalLabel":"isHere"
			}
		},
		"spec":{
			"selector":{
				"matchLabels":{
					"app":"nginxo"
				}
			},
			"serviceName":"nginxo",
			"replicas":3,
			"template":{
				"metadata":{
					"labels":{
						"app":"nginxo"
					}
				},
				"spec":{
					"terminationGracePeriodSeconds":10,
					"containers":[
						{
							"name":"nginxo",
							"image":"k8s.gcr.io/nginx-but-no-slim:0.8",
							"ports":[
								{
									"containerPort":8780,
									"name":"webp"
								}
							],
							"volumeMounts":[
								{
									"name":"www",
									"mountPath":"/usr/share/nginxo/html"
								}
							],
							"livenessProbe":{
								"periodSeconds":11
							}
						}
					]
				}
			},
			"volumeClaimTemplates":[
				{
					"metadata":{
						"name":"www"
					},
					"spec":{
						"accessModes":[
							"ReadWriteOnce"
						],
						"storageClassName":"my-storage-class",
						"resources":{
							"requests":{
								"storage":"1Gi"
							}
						}
					}
				}
			]
		}
	}`)

	var pattern, resource map[string]interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	res := validateMap(resource, pattern, "/")
	assert.NilError(t, res.ToError())
}

func TestValidateMap_AsteriskFieldIsMissing(t *testing.T) {
	rawPattern := []byte(`{
		"spec":{
			"template":{
				"spec":{
					"containers":[
						{
							"name":"*",
							"livenessProbe":"*"
						}
					]
				}
			}
		}
	}`)
	rawMap := []byte(`{
		"apiVersion":"apps/v1",
		"kind":"StatefulSet",
		"metadata":{
			"name":"game-web",
			"labels":{
				"originalLabel":"isHere"
			}
		},
		"spec":{
			"selector":{
				"matchLabels":{
					"app":"nginxo"
				}
			},
			"serviceName":"nginxo",
			"replicas":3,
			"template":{
				"metadata":{
					"labels":{
						"app":"nginxo"
					}
				},
				"spec":{
					"terminationGracePeriodSeconds":10,
					"containers":[
						{
							"name":"nginxo",
							"image":"k8s.gcr.io/nginx-but-no-slim:0.8",
							"ports":[
								{
									"containerPort":8780,
									"name":"webp"
								}
							],
							"volumeMounts":[
								{
									"name":"www",
									"mountPath":"/usr/share/nginxo/html"
								}
							],
							"livenessProbe":null
						}
					]
				}
			},
			"volumeClaimTemplates":[
				{
					"metadata":{
						"name":"www"
					},
					"spec":{
						"accessModes":[
							"ReadWriteOnce"
						],
						"storageClassName":"my-storage-class",
						"resources":{
							"requests":{
								"storage":"1Gi"
							}
						}
					}
				}
			]
		}
	}`)

	var pattern, resource map[string]interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	res := validateMap(resource, pattern, "/")
	assert.Assert(t, res.ToError() != nil)
}

func TestValidateMap_livenessProbeIsNull(t *testing.T) {
	rawPattern := []byte(`{
		"spec":{
			"template":{
				"spec":{
					"containers":[
						{
							"name":"*",
							"livenessProbe":null
						}
					]
				}
			}
		}
	}`)
	rawMap := []byte(`{
		"apiVersion":"apps/v1",
		"kind":"StatefulSet",
		"metadata":{
			"name":"game-web",
			"labels":{
				"originalLabel":"isHere"
			}
		},
		"spec":{
			"selector":{
				"matchLabels":{
					"app":"nginxo"
				}
			},
			"serviceName":"nginxo",
			"replicas":3,
			"template":{
				"metadata":{
					"labels":{
						"app":"nginxo"
					}
				},
				"spec":{
					"terminationGracePeriodSeconds":10,
					"containers":[
						{
							"name":"nginxo",
							"image":"k8s.gcr.io/nginx-but-no-slim:0.8",
							"ports":[
								{
									"containerPort":8780,
									"name":"webp"
								}
							],
							"volumeMounts":[
								{
									"name":"www",
									"mountPath":"/usr/share/nginxo/html"
								}
							],
							"livenessProbe":null
						}
					]
				}
			},
			"volumeClaimTemplates":[
				{
					"metadata":{
						"name":"www"
					},
					"spec":{
						"accessModes":[
							"ReadWriteOnce"
						],
						"storageClassName":"my-storage-class",
						"resources":{
							"requests":{
								"storage":"1Gi"
							}
						}
					}
				}
			]
		}
	}`)

	var pattern, resource map[string]interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	res := validateMap(resource, pattern, "/")
	assert.NilError(t, res.ToError())
}

func TestValidateMap_livenessProbeIsMissing(t *testing.T) {
	rawPattern := []byte(`{
		"spec":{
			"template":{
				"spec":{
					"containers":[
						{
							"name":"*",
 							"livenessProbe" : null
						}
					]
				}
			}
		}
	}`)
	rawMap := []byte(`{
		"apiVersion":"apps/v1",
		"kind":"StatefulSet",
		"metadata":{
			"name":"game-web",
			"labels":{
				"originalLabel":"isHere"
			}
		},
		"spec":{
			"selector":{
				"matchLabels":{
					"app":"nginxo"
				}
			},
			"serviceName":"nginxo",
			"replicas":3,
			"template":{
				"metadata":{
					"labels":{
						"app":"nginxo"
					}
				},
				"spec":{
					"terminationGracePeriodSeconds":10,
					"containers":[
						{
							"name":"nginxo",
							"image":"k8s.gcr.io/nginx-but-no-slim:0.8",
							"ports":[
								{
									"containerPort":8780,
									"name":"webp"
								}
							],
							"volumeMounts":[
								{
									"name":"www",
									"mountPath":"/usr/share/nginxo/html"
								}
							]
						}
					]
				}
			},
			"volumeClaimTemplates":[
				{
					"metadata":{
						"name":"www"
					},
					"spec":{
						"accessModes":[
							"ReadWriteOnce"
						],
						"storageClassName":"my-storage-class",
						"resources":{
							"requests":{
								"storage":"1Gi"
							}
						}
					}
				}
			]
		}
	}`)

	var pattern, resource map[string]interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	res := validateMap(resource, pattern, "/")
	assert.NilError(t, res.ToError())
}

func TestValidateMapElement_TwoElementsInArrayOnePass(t *testing.T) {
	rawPattern := []byte(`[
		{
			"(name)":"nirmata-*",
			"object":[
				{
					"(key1)":"value*",
					"key2":"value*"
				}
			]
		}
	]`)
	rawMap := []byte(`[
		{
			"name":"nirmata-1",
			"object":[
				{
					"key1":"value1",
					"key2":"value2"
				}
			]
		},
		{
			"name":"nirmata-1",
			"object":[
				{
					"key1":"not_value",
					"key2":"not_value"
				}
			]
		}
	]`)

	var pattern, resource interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	res := validateResourceElement(resource, pattern, "/")
	assert.NilError(t, res.ToError())
}

func TestValidateMapElement_OneElementInArrayPass(t *testing.T) {
	rawPattern := []byte(`[
		{
			"(name)":"nirmata-*",
			"object":[
				{
					"(key1)":"value*",
					"key2":"value*"
				}
			]
		}
	]`)
	rawMap := []byte(`[
		{
			"name":"nirmata-1",
			"object":[
				{
					"key1":"value1",
					"key2":"value2"
				}
			]
		}
	]`)

	var pattern, resource interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	res := validateResourceElement(resource, pattern, "/")
	assert.NilError(t, res.ToError())
}

func TestValidateMapElement_OneElementInArrayNotPass(t *testing.T) {
	rawPattern := []byte(`[
		{
			"(name)":"nirmata-*",
			"object":[
				{
					"(key1)":"value*",
					"key2":"value*"
				}
			]
		}
	]`)
	rawMap := []byte(`[
		{
			"name":"nirmata-1",
			"object":[
				{
					"key1":"value5",
					"key2":"1value1"
				}
			]
		}
	]`)

	var pattern, resource interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	res := validateResourceElement(resource, pattern, "/")
	assert.Assert(t, res.ToError() != nil)
}

func TestValidate_ServiceTest(t *testing.T) {
	rawPolicy := []byte(`{
		"apiVersion":"kyverno.nirmata.io/v1alpha1",
		"kind":"Policy",
		"metadata":{
			"name":"policy-service"
		},
		"spec":{
			"rules":[
				{
					"name":"ps1",
					"resource":{
						"kinds":[
							"Service"
						],
						"name":"game-service*"
					},
					"mutate":{
						"patches":[
							{
								"path":"/metadata/labels/isMutated",
								"op":"add",
								"value":"true"
							},
							{
								"path":"/metadata/labels/secretLabel",
								"op":"replace",
								"value":"weKnow"
							},
							{
								"path":"/metadata/labels/originalLabel",
								"op":"remove"
							},
							{
								"path":"/spec/selector/app",
								"op":"replace",
								"value":"mutedApp"
							}
						]
					},
					"validate":{
						"message":"This resource is broken",
						"pattern":{
							"spec":{
								"ports":[
									{
										"name":"hs",
										"protocol":32
									}
								]
							}
						}
					}
				}
			]
		}
	}`)
	rawResource := []byte(`{
		"kind":"Service",
		"apiVersion":"v1",
		"metadata":{
			"name":"game-service",
			"labels":{
				"originalLabel":"isHere",
				"secretLabel":"thisIsMySecret"
			}
		},
		"spec":{
			"selector":{
				"app":"MyApp"
			},
			"ports":[
				{
					"name":"http",
					"protocol":"TCP",
					"port":80,
					"targetPort":9376
				}
			]
		}
	}
	`)

	var policy kubepolicy.Policy
	json.Unmarshal(rawPolicy, &policy)

	gvk := metav1.GroupVersionKind{
		Kind: "Service",
	}

	assert.Assert(t, Validate(policy, rawResource, gvk) != nil)
}

func TestValidate_MapHasFloats(t *testing.T) {
	rawPolicy := []byte(`{
		"apiVersion":"kyverno.nirmata.io/v1alpha1",
		"kind":"Policy",
		"metadata":{
			"name":"policy-deployment-changed"
		},
		"spec":{
			"rules":[
				{
					"name":"First policy v2",
					"resource":{
						"kinds":[
							"Deployment"
						],
						"name":"nginx-*"
					},
					"mutate":{
						"patches":[
							{
								"path":"/metadata/labels/isMutated",
								"op":"add",
								"value":"true"
							},
							{
								"path":"/metadata/labels/app",
								"op":"replace",
								"value":"nginx_is_mutated"
							}
						]
					},
					"validate":{
						"message":"replicas number is wrong",
						"pattern":{
							"metadata":{
								"labels":{
									"app":"*"
								}
							},
							"spec":{
								"replicas":3
							}
						}
					}
				}
			]
		}
	}`)
	rawResource := []byte(`{
		"apiVersion":"apps/v1",
		"kind":"Deployment",
		"metadata":{
			"name":"nginx-deployment",
			"labels":{
				"app":"nginx"
			}
		},
		"spec":{
			"replicas":3,
			"selector":{
				"matchLabels":{
					"app":"nginx"
				}
			},
			"template":{
				"metadata":{
					"labels":{
						"app":"nginx"
					}
				},
				"spec":{
					"containers":[
						{
							"name":"nginx",
							"image":"nginx:1.7.9",
							"ports":[
								{
									"containerPort":80
								}
							]
						}
					]
				}
			}
		}
	}
	`)

	var policy kubepolicy.Policy
	json.Unmarshal(rawPolicy, &policy)

	gvk := metav1.GroupVersionKind{
		Kind: "Deployment",
	}

	res := Validate(policy, rawResource, gvk)
	assert.NilError(t, res.ToError())
}
