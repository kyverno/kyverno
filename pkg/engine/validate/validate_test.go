package validate

import (
	"encoding/json"
	"testing"

	"gotest.tools/assert"
)

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
	rawPattern := []byte(`{
		"^(list)": [
		  {
			"(name)": "nirmata-*",
			"object": [
			  {
				"(key1)": "value*",
				"key2": "value*"
			  }
			]
		  }
		]
	  }`)
	rawMap := []byte(`{
		"list": [
		  {
			"name": "nirmata-1",
			"object": [
			  {
				"key1": "value1",
				"key2": "value2"
			  }
			]
		  },
		  {
			"name": "nirmata-1",
			"object": [
			  {
				"key1": "not_value",
				"key2": "not_value"
			  }
			]
		  }
		]
	  }`)

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

	result := formAbsolutePath(referencePath, absolutePath)

	assert.Assert(t, result == expectedString)
}

func TestFormAbsolutePath_RelativePathWithBackToTopInTheBegining(t *testing.T) {
	absolutePath := "/spec/containers/0/resources/requests/memory"
	referencePath := "../../limits/memory"
	expectedString := "/spec/containers/0/resources/limits/memory"

	result := formAbsolutePath(referencePath, absolutePath)

	assert.Assert(t, result == expectedString)
}

func TestFormAbsolutePath_AbsolutePathExists(t *testing.T) {
	absolutePath := "/spec/containers/0/resources/requests/memory"
	referencePath := "/spec/containers/0/resources/limits/memory"

	result := formAbsolutePath(referencePath, absolutePath)

	assert.Assert(t, result == referencePath)
}

func TestFormAbsolutePath_EmptyPath(t *testing.T) {
	absolutePath := "/spec/containers/0/resources/requests/memory"
	referencePath := ""

	result := formAbsolutePath(referencePath, absolutePath)

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
