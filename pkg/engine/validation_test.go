package engine

import (
	"encoding/json"
	"testing"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	"gotest.tools/assert"
)

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

	actualMap := getAnchorsFromMap(unmarshalled)
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

	actualMap := getAnchorsFromMap(unmarshalled)
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

	path, err := validateMap(resource, pattern, pattern, "/")
	assert.Equal(t, path, "")
	assert.NilError(t, err)
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

	path, err := validateMap(resource, pattern, pattern, "/")
	t.Log(path)
	assert.NilError(t, err)
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

	path, err := validateMap(resource, pattern, pattern, "/")
	assert.Equal(t, path, "")
	assert.NilError(t, err)
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

	path, err := validateMap(resource, pattern, pattern, "/")
	assert.Equal(t, path, "")
	assert.NilError(t, err)
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

	path, err := validateMap(resource, pattern, pattern, "/")
	assert.Equal(t, path, "/spec/template/spec/containers/0/")
	assert.Assert(t, err != nil)
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

	path, err := validateMap(resource, pattern, pattern, "/")
	assert.Equal(t, path, "")
	assert.NilError(t, err)
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

	path, err := validateMap(resource, pattern, pattern, "/")
	assert.Equal(t, path, "")
	assert.NilError(t, err)
}

func TestValidateMapElement_TwoElementsInArrayOnePass(t *testing.T) {
	rawPattern := []byte(`[
		{
			"(name)":"nirmata-*",
			"object":[
				{
					"^(key1)":"value*",
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

	path, err := validateResourceElement(resource, pattern, pattern, "/")
	assert.Equal(t, path, "")
	// assert.Equal(t, path, "/1/object/0/key2/")
	// assert.NilError(t, err)
	assert.Assert(t, err == nil)
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

	path, err := validateResourceElement(resource, pattern, pattern, "/")
	assert.Equal(t, path, "")
	assert.NilError(t, err)
}

func TestValidateMap_CorrectRelativePathInConfig(t *testing.T) {
	rawPattern := []byte(`{
		"spec":{
			"containers":[
				{
					"name":"*",
					"resources":{
						"requests":{
							"memory":"$(<=./../../limits/memory)"
						},
						"limits":{
							"memory":"2048Mi"
						}
					}
				}
			]
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
			"containers":[
				{
					"name":"nirmata",
					"resources":{
						"requests":{
							"memory":"1024Mi"
						},
						"limits":{
							"memory":"2048Mi"
						}
					}
				}
			]
		}
	}`)

	var pattern, resource interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	path, err := validateResourceElement(resource, pattern, pattern, "/")
	assert.Equal(t, path, "")
	assert.NilError(t, err)
}

func TestValidateMap_RelativePathDoesNotExists(t *testing.T) {
	rawPattern := []byte(`{
		"spec":{
			"containers":[
				{
					"name":"*",
					"resources":{
						"requests":{
							"memory":"$(./../somekey/somekey2/memory)"
						},
						"limits":{
							"memory":"2048Mi"
						}
					}
				}
			]
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
			"containers":[
				{
					"name":"nirmata",
					"resources":{
						"requests":{
							"memory":"1024Mi"
						},
						"limits":{
							"memory":"2048Mi"
						}
					}
				}
			]
		}
	}`)

	var pattern, resource interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	path, err := validateResourceElement(resource, pattern, pattern, "/")
	assert.Equal(t, path, "/spec/containers/0/resources/requests/memory/")
	assert.Assert(t, err != nil)
}

func TestValidateMap_OnlyAnchorsInPath(t *testing.T) {
	rawPattern := []byte(`{
		"spec":{
			"containers":[
				{
					"name":"*",
					"resources":{
						"requests":{
							"memory":"$()"
						},
						"limits":{
							"memory":"2048Mi"
						}
					}
				}
			]
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
			"containers":[
				{
					"name":"nirmata",
					"resources":{
						"requests":{
							"memory":"1024Mi"
						},
						"limits":{
							"memory":"2048Mi"
						}
					}
				}
			]
		}
	}`)

	var pattern, resource interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	path, err := validateResourceElement(resource, pattern, pattern, "/")
	assert.Equal(t, path, "/spec/containers/0/resources/requests/memory/")
	assert.Assert(t, err != nil)
}

func TestValidateMap_MalformedReferenceOnlyDolarMark(t *testing.T) {
	rawPattern := []byte(`{
		"spec":{
			"containers":[
				{
					"name":"*",
					"resources":{
						"requests":{
							"memory":"$"
						},
						"limits":{
							"memory":"2048Mi"
						}
					}
				}
			]
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
			"containers":[
				{
					"name":"nirmata",
					"resources":{
						"requests":{
							"memory":"1024Mi"
						},
						"limits":{
							"memory":"2048Mi"
						}
					}
				}
			]
		}
	}`)

	var pattern, resource interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	path, err := validateResourceElement(resource, pattern, pattern, "/")
	assert.Equal(t, path, "/spec/containers/0/resources/requests/memory/")
	assert.Assert(t, err != nil)
}

func TestValidateMap_RelativePathWithParentheses(t *testing.T) {
	rawPattern := []byte(`{
		"spec":{
			"containers":[
				{
					"name":"*",
					"resources":{
						"requests":{
							"memory":"$(<=./../../lim(its/mem)ory)"
						},
						"lim(its":{
							"mem)ory":"2048Mi"
						}
					}
				}
			]
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
			"containers":[
				{
					"name":"nirmata",
					"resources":{
						"requests":{
							"memory":"1024Mi"
						},
						"lim(its":{
							"mem)ory":"2048Mi"
						}
					}
				}
			]
		}
	}`)

	var pattern, resource interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	path, err := validateResourceElement(resource, pattern, pattern, "/")
	assert.Equal(t, path, "")
	assert.NilError(t, err)
}

func TestValidateMap_MalformedPath(t *testing.T) {
	rawPattern := []byte(`{
		"spec":{
			"containers":[
				{
					"name":"*",
					"resources":{
						"requests":{
							"memory":"$(>2048)"
						},
						"limits":{
							"memory":"2048Mi"
						}
					}
				}
			]
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
			"containers":[
				{
					"name":"nirmata",
					"resources":{
						"requests":{
							"memory":"1024Mi"
						},
						"limits":{
							"memory":"2048Mi"
						}
					}
				}
			]
		}
	}`)

	var pattern, resource interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	path, err := validateResourceElement(resource, pattern, pattern, "/")
	assert.Equal(t, path, "/spec/containers/0/resources/requests/memory/")
	assert.Assert(t, err != nil)
}

func TestValidateMap_AbosolutePathExists(t *testing.T) {
	rawPattern := []byte(`{
		"spec":{
			"containers":[
				{
					"name":"*",
					"resources":{
						"requests":{
							"memory":"$(<=/spec/containers/0/resources/limits/memory)"
						},
						"limits":{
							"memory":"2048Mi"
						}
					}
				}
			]
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
			"containers":[
				{
					"name":"nirmata",
					"resources":{
						"requests":{
							"memory":"1024Mi"
						},
						"limits":{
							"memory":"2048Mi"
						}
					}
				}
			]
		}
	}`)

	var pattern, resource interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	path, err := validateResourceElement(resource, pattern, pattern, "/")
	assert.Equal(t, path, "")
	assert.Assert(t, err == nil)
}

func TestValidateMap_AbsolutePathToMetadata(t *testing.T) {
	rawPattern := []byte(`{
		"metadata":{
			"labels":{
				"app":"nirmata*"
			}
		},
		"spec":{
			"containers":[
				{
					"(name)":"$(/metadata/labels/app)",
					"(image)":"nirmata.io*"
				}
			]
		}
	}`)

	rawMap := []byte(`{
		"metadata":{
			"labels":{
				"app":"nirmata*"
			}
		},
		"spec":{
			"containers":[
				{
					"name":"nirmata"
				}
			]
		}
	}`)

	var pattern, resource interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	path, err := validateResourceElement(resource, pattern, pattern, "/")
	assert.Equal(t, path, "")
	assert.Assert(t, err == nil)
}

func TestValidateMap_AbsolutePathToMetadata_fail(t *testing.T) {
	rawPattern := []byte(`{
		"metadata":{
			"labels":{
				"app":"nirmata*"
			}
		},
		"spec":{
			"containers":[
				{
					"(name)":"$(/metadata/labels/app)",
					"image":"nirmata.io*"
				}
			]
		}
	}`)

	rawMap := []byte(`{
		"metadata":{
			"labels":{
				"app":"nirmata*"
			}
		},
		"spec":{
			"containers":[
				{
					"name":"nirmata",
					"image":"nginx"
				}
			]
		}
	}`)

	var pattern, resource interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	path, err := validateResourceElement(resource, pattern, pattern, "/")
	assert.Equal(t, path, "/spec/containers/0/image/")
	assert.Assert(t, err != nil)
}

func TestValidateMap_AbosolutePathDoesNotExists(t *testing.T) {
	rawPattern := []byte(`{
		"spec":{
			"containers":[
				{
					"name":"*",
					"resources":{
						"requests":{
							"memory":"$(<=/some/memory)"
						},
						"limits":{
							"memory":"2048Mi"
						}
					}
				}
			]
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
			"containers":[
				{
					"name":"nirmata",
					"resources":{
						"requests":{
							"memory":"1024Mi"
						},
						"limits":{
							"memory":"2048Mi"
						}
					}
				}
			]
		}
	}`)

	var pattern, resource interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	path, err := validateResourceElement(resource, pattern, pattern, "/")
	assert.Equal(t, path, "/spec/containers/0/resources/requests/memory/")
	assert.Assert(t, err != nil)
}

func TestActualizePattern_GivenRelativePathThatExists(t *testing.T) {
	absolutePath := "/spec/containers/0/resources/requests/memory"
	referencePath := "$(<=./../../limits/memory)"

	rawPattern := []byte(`{
		"spec":{
			"containers":[
				{
					"name":"*",
					"resources":{
						"requests":{
							"memory":"$(<=./../../limits/memory)"
						},
						"limits":{
							"memory":"2048Mi"
						}
					}
				}
			]
		}
	}`)

	var pattern interface{}

	json.Unmarshal(rawPattern, &pattern)

	pattern, err := actualizePattern(pattern, referencePath, absolutePath)

	assert.Assert(t, err == nil)
}

func TestFormAbsolutePath_RelativePathExists(t *testing.T) {
	absolutePath := "/spec/containers/0/resources/requests/memory"
	referencePath := "./../../limits/memory"
	expectedString := "/spec/containers/0/resources/limits/memory"

	result := FormAbsolutePath(referencePath, absolutePath)

	assert.Assert(t, result == expectedString)
}

func TestFormAbsolutePath_RelativePathWithBackToTopInTheBegining(t *testing.T) {
	absolutePath := "/spec/containers/0/resources/requests/memory"
	referencePath := "../../limits/memory"
	expectedString := "/spec/containers/0/resources/limits/memory"

	result := FormAbsolutePath(referencePath, absolutePath)

	assert.Assert(t, result == expectedString)
}

func TestFormAbsolutePath_AbsolutePathExists(t *testing.T) {
	absolutePath := "/spec/containers/0/resources/requests/memory"
	referencePath := "/spec/containers/0/resources/limits/memory"

	result := FormAbsolutePath(referencePath, absolutePath)

	assert.Assert(t, result == referencePath)
}

func TestFormAbsolutePath_EmptyPath(t *testing.T) {
	absolutePath := "/spec/containers/0/resources/requests/memory"
	referencePath := ""

	result := FormAbsolutePath(referencePath, absolutePath)

	assert.Assert(t, result == absolutePath)
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

	path, err := validateResourceElement(resource, pattern, pattern, "/")
	assert.Equal(t, path, "/0/object/0/key2/")
	assert.Assert(t, err != nil)
}

func TestValidate_ServiceTest(t *testing.T) {
	rawPolicy := []byte(`{
		"apiVersion":"kyverno.nirmata.io/v1alpha1",
		"kind":"ClusterPolicy",
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

	var policy kyverno.ClusterPolicy
	json.Unmarshal(rawPolicy, &policy)

	resourceUnstructured, err := ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(policy, *resourceUnstructured)
	assert.Assert(t, len(er.PolicyResponse.Rules) == 0)
}

func TestValidate_MapHasFloats(t *testing.T) {
	rawPolicy := []byte(`{
		"apiVersion":"kyverno.nirmata.io/v1alpha1",
		"kind":"ClusterPolicy",
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

	var policy kyverno.ClusterPolicy
	json.Unmarshal(rawPolicy, &policy)

	resourceUnstructured, err := ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(policy, *resourceUnstructured)
	assert.Assert(t, len(er.PolicyResponse.Rules) == 0)
}

func TestValidate_image_tag_fail(t *testing.T) {
	// If image tag is latest then imagepull policy needs to be checked
	rawPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1alpha1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "validate-image"
		},
		"spec": {
		   "rules": [
			  {
				 "name": "validate-tag",
				 "match": {
					"resources": {
					   "kinds": [
						  "Pod"
					   ]
					}
				 },
				 "validate": {
					"message": "An image tag is required",
					"pattern": {
					   "spec": {
						  "containers": [
							 {
								"image": "*:*"
							 }
						  ]
					   }
					}
				 }
			  },
			  {
				 "name": "validate-latest",
				 "match": {
					"resources": {
					   "kinds": [
						  "Pod"
					   ]
					}
				 },
				 "validate": {
					"message": "imagePullPolicy 'Always' required with tag 'latest'",
					"pattern": {
					   "spec": {
						  "containers": [
							 {
								"^(image)": "*latest",
								"imagePullPolicy": "NotPresent"
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
		   "name": "myapp-pod",
		   "labels": {
			  "app": "myapp"
		   }
		},
		"spec": {
		   "containers": [
			  {
				 "name": "nginx",
				 "image": "nginx:latest",
				 "imagePullPolicy": "Always"
			  }
		   ]
		}
	 }
	`)

	var policy kyverno.ClusterPolicy
	json.Unmarshal(rawPolicy, &policy)

	resourceUnstructured, err := ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	// msgs := []string{
	// 	"Validation rule 'validate-tag' failed at '/spec/containers/0/image/' for resource Pod//myapp-pod. An image tag is required",
	// 	"Validation rule 'validate-latest' succesfully validated",
	// }
	er := Validate(policy, *resourceUnstructured)
	for _, r := range er.PolicyResponse.Rules {
		t.Log(r.Message)
		//		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, !er.IsSuccesful())
}

func TestValidate_image_tag_pass(t *testing.T) {
	// If image tag is latest then imagepull policy needs to be checked
	rawPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1alpha1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "validate-image"
		},
		"spec": {
		   "rules": [
			  {
				 "name": "validate-tag",
				 "match": {
					"resources": {
					   "kinds": [
						  "Pod"
					   ]
					}
				 },
				 "validate": {
					"message": "An image tag is required",
					"pattern": {
					   "spec": {
						  "containers": [
							 {
								"image": "*:*"
							 }
						  ]
					   }
					}
				 }
			  },
			  {
				 "name": "validate-latest",
				 "match": {
					"resources": {
					   "kinds": [
						  "Pod"
					   ]
					}
				 },
				 "validate": {
					"message": "imagePullPolicy 'Always' required with tag 'latest'",
					"pattern": {
					   "spec": {
						  "containers": [
							 {
								"^(image)": "*latest",
								"imagePullPolicy": "Always"
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
		   "name": "myapp-pod",
		   "labels": {
			  "app": "myapp"
		   }
		},
		"spec": {
		   "containers": [
			  {
				 "name": "nginx",
				 "image": "nginx:latest",
				 "imagePullPolicy": "Always"
			  }
		   ]
		}
	 }
	`)

	var policy kyverno.ClusterPolicy
	json.Unmarshal(rawPolicy, &policy)

	resourceUnstructured, err := ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	// msgs := []string{
	// 	"Validation rule 'validate-tag' failed at '/spec/containers/0/image/' for resource Pod//myapp-pod. An image tag is required",
	// 	"Validation rule 'validate-latest' succesfully validated",
	// }
	er := Validate(policy, *resourceUnstructured)
	for _, r := range er.PolicyResponse.Rules {
		t.Log(r.Message)
		//		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccesful())
}

func TestValidate_Fail_anyPattern(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1alpha1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "validate-namespace"
		},
		"spec": {
		   "rules": [
			  {
				 "name": "check-default-namespace",
				 "match": {
					"resources": {
					   "kinds": [
						  "Pod"
					   ]
					}
				 },
				 "validate": {
					"message": "A namespace is required",
					"anyPattern": [
					   {
						  "metadata": {
							 "namespace": "?*"
						  }
					   },
					   {
						  "metadata": {
							 "namespace": "!default"
						  }
					   }
					]
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
		   "name": "myapp-pod",
		   "labels": {
			  "app": "myapp"
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
	 }
	`)

	var policy kyverno.ClusterPolicy
	json.Unmarshal(rawPolicy, &policy)

	resourceUnstructured, err := ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(policy, *resourceUnstructured)
	msgs := []string{"Validation rule 'check-default-namespace' failed to validate patterns defined in anyPattern. A namespace is required; anyPattern[0] failed at path /metadata/namespace/; anyPattern[1] failed at path /metadata/namespace/"}
	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, !er.IsSuccesful())
}

func TestValidate_host_network_port(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1alpha1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "validate-host-network-port"
		},
		"spec": {
		   "rules": [
			  {
				 "name": "validate-host-network-port",
				 "match": {
					"resources": {
					   "kinds": [
						  "Pod"
					   ]
					}
				 },
				 "validate": {
					"message": "Host network and port are not allowed",
					"pattern": {
					   "spec": {
						  "hostNetwork": false,
						  "containers": [
							 {
								"name": "*",
								"ports": [
								   {
									  "hostPort": null
								   }
								]
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
		   "name": "nginx-host-network"
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			  {
				 "name": "nginx-host-network",
				 "image": "nginx",
				 "ports": [
					{
					   "containerPort": 80,
					   "hostPort": 80
					}
				 ]
			  }
		   ]
		}
	 }
	 `)

	var policy kyverno.ClusterPolicy
	json.Unmarshal(rawPolicy, &policy)

	resourceUnstructured, err := ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(policy, *resourceUnstructured)
	msgs := []string{"Validation rule 'validate-host-network-port' failed at '/spec/containers/0/ports/0/hostPort/' for resource Pod//nginx-host-network. Host network and port are not allowed"}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, !er.IsSuccesful())
}

func TestValidate_anchor_arraymap_pass(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1alpha1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "validate-host-path"
		},
		"spec": {
		   "rules": [
			  {
				 "name": "validate-host-path",
				 "match": {
					"resources": {
					   "kinds": [
						  "Pod"
					   ]
					}
				 },
				 "validate": {
					"message": "Host path '/var/lib/' is not allowed",
					"pattern": {
					   "spec": {
						  "volumes": [
							 {
								"name": "*",
								"(hostPath)": {
								   "path": "!/var/lib"
								}
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
		   "name": "image-with-hostpath",
		   "labels": {
			  "app.type": "prod",
			  "namespace": "my-namespace"
		   }
		},
		"spec": {
		   "containers": [
			  {
				 "name": "image-with-hostpath",
				 "image": "docker.io/nautiker/curl",
				 "volumeMounts": [
					{
					   "name": "var-lib-etcd",
					   "mountPath": "/var/lib"
					}
				 ]
			  }
		   ],
		   "volumes": [
			  {
				 "name": "var-lib-etcd",
				 "hostPath": {
					"path": "/var/lib1"
				 }
			  }
		   ]
		}
	 }	 `)

	var policy kyverno.ClusterPolicy
	json.Unmarshal(rawPolicy, &policy)

	resourceUnstructured, err := ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(policy, *resourceUnstructured)
	msgs := []string{"Validation rule 'validate-host-path' succesfully validated"}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccesful())
}

func TestValidate_anchor_arraymap_fail(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1alpha1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "validate-host-path"
		},
		"spec": {
		   "rules": [
			  {
				 "name": "validate-host-path",
				 "match": {
					"resources": {
					   "kinds": [
						  "Pod"
					   ]
					}
				 },
				 "validate": {
					"message": "Host path '/var/lib/' is not allowed",
					"pattern": {
					   "spec": {
						  "volumes": [
							 {
								"name": "*",
								"(hostPath)": {
								   "path": "!/var/lib"
								}
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
		   "name": "image-with-hostpath",
		   "labels": {
			  "app.type": "prod",
			  "namespace": "my-namespace"
		   }
		},
		"spec": {
		   "containers": [
			  {
				 "name": "image-with-hostpath",
				 "image": "docker.io/nautiker/curl",
				 "volumeMounts": [
					{
					   "name": "var-lib-etcd",
					   "mountPath": "/var/lib"
					}
				 ]
			  }
		   ],
		   "volumes": [
			  {
				 "name": "var-lib-etcd",
				 "hostPath": {
					"path": "/var/lib"
				 }
			  }
		   ]
		}
	 }	 `)

	var policy kyverno.ClusterPolicy
	json.Unmarshal(rawPolicy, &policy)

	resourceUnstructured, err := ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(policy, *resourceUnstructured)
	msgs := []string{"Validation rule 'validate-host-path' failed at '/spec/volumes/0/hostPath/path/' for resource Pod//image-with-hostpath. Host path '/var/lib/' is not allowed"}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, !er.IsSuccesful())
}

func TestValidate_anchor_map_notfound(t *testing.T) {
	// anchor not present in resource
	rawPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1alpha1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "policy-secaas-k8s"
		},
		"spec": {
		   "rules": [
			  {
				 "name": "pod rule 2",
				 "match": {
					"resources": {
					   "kinds": [
						  "Pod"
					   ]
					}
				 },
				 "validate": {
					"message": "pod: validate run as non root user",
					"pattern": {
					   "spec": {
						  "(securityContext)": {
							 "runAsNonRoot": true
						  }
					   }
					}
				 }
			  }
		   ]
		}
	 }	 `)

	rawResource := []byte(`
	{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
		   "name": "myapp-pod",
		   "labels": {
			  "app": "v1"
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
	 }
`)

	var policy kyverno.ClusterPolicy
	json.Unmarshal(rawPolicy, &policy)

	resourceUnstructured, err := ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(policy, *resourceUnstructured)
	msgs := []string{"Validation rule 'pod rule 2' succesfully validated"}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccesful())
}

func TestValidate_anchor_map_found_valid(t *testing.T) {
	// anchor not present in resource
	rawPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1alpha1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "policy-secaas-k8s"
		},
		"spec": {
		   "rules": [
			  {
				 "name": "pod rule 2",
				 "match": {
					"resources": {
					   "kinds": [
						  "Pod"
					   ]
					}
				 },
				 "validate": {
					"message": "pod: validate run as non root user",
					"pattern": {
					   "spec": {
						  "(securityContext)": {
							 "runAsNonRoot": true
						  }
					   }
					}
				 }
			  }
		   ]
		}
	 }	 `)

	rawResource := []byte(`
	{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
		   "name": "myapp-pod",
		   "labels": {
			  "app": "v1"
		   }
		},
		"spec": {
		   "containers": [
			  {
				 "name": "nginx",
				 "image": "nginx"
			  }
		   ],
		   "securityContext": {
			  "runAsNonRoot": true
		   }
		}
	 }
`)

	var policy kyverno.ClusterPolicy
	json.Unmarshal(rawPolicy, &policy)

	resourceUnstructured, err := ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(policy, *resourceUnstructured)
	msgs := []string{"Validation rule 'pod rule 2' succesfully validated"}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccesful())
}

func TestValidate_anchor_map_found_invalid(t *testing.T) {
	// anchor not present in resource
	rawPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1alpha1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "policy-secaas-k8s"
		},
		"spec": {
		   "rules": [
			  {
				 "name": "pod rule 2",
				 "match": {
					"resources": {
					   "kinds": [
						  "Pod"
					   ]
					}
				 },
				 "validate": {
					"message": "pod: validate run as non root user",
					"pattern": {
					   "spec": {
						  "(securityContext)": {
							 "runAsNonRoot": true
						  }
					   }
					}
				 }
			  }
		   ]
		}
	 }	 `)

	rawResource := []byte(`
	{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
		   "name": "myapp-pod",
		   "labels": {
			  "app": "v1"
		   }
		},
		"spec": {
		   "containers": [
			  {
				 "name": "nginx",
				 "image": "nginx"
			  }
		   ],
		   "securityContext": {
			  "runAsNonRoot": false
		   }
		}
	 }
`)

	var policy kyverno.ClusterPolicy
	json.Unmarshal(rawPolicy, &policy)

	resourceUnstructured, err := ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(policy, *resourceUnstructured)
	msgs := []string{"Validation rule 'pod rule 2' failed at '/spec/securityContext/runAsNonRoot/' for resource Pod//myapp-pod. pod: validate run as non root user"}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, !er.IsSuccesful())
}
