package validate

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/kyverno/kyverno/pkg/engine/common"
	"gotest.tools/assert"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
	assert.Assert(t, json.Unmarshal(rawPattern, &pattern))
	assert.Assert(t, json.Unmarshal(rawMap, &resource))

	path, err := validateMap(log.Log, resource, pattern, pattern, "/", common.NewAnchorMap())
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
	assert.Assert(t, json.Unmarshal(rawPattern, &pattern))
	assert.Assert(t, json.Unmarshal(rawMap, &resource))

	path, err := validateMap(log.Log, resource, pattern, pattern, "/", common.NewAnchorMap())
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
	assert.Assert(t, json.Unmarshal(rawPattern, &pattern))
	assert.Assert(t, json.Unmarshal(rawMap, &resource))

	path, err := validateMap(log.Log, resource, pattern, pattern, "/", common.NewAnchorMap())
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
	assert.Assert(t, json.Unmarshal(rawPattern, &pattern))
	assert.Assert(t, json.Unmarshal(rawMap, &resource))

	path, err := validateMap(log.Log, resource, pattern, pattern, "/", common.NewAnchorMap())
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
	assert.Assert(t, json.Unmarshal(rawPattern, &pattern))
	assert.Assert(t, json.Unmarshal(rawMap, &resource))

	path, err := validateMap(log.Log, resource, pattern, pattern, "/", common.NewAnchorMap())
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
	assert.Assert(t, json.Unmarshal(rawPattern, &pattern))
	err := json.Unmarshal(rawMap, &resource)
	assert.NilError(t, err)

	path, err := validateMap(log.Log, resource, pattern, pattern, "/", common.NewAnchorMap())
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
	assert.Assert(t, json.Unmarshal(rawPattern, &pattern))
	assert.Assert(t, json.Unmarshal(rawMap, &resource))

	path, err := validateMap(log.Log, resource, pattern, pattern, "/", common.NewAnchorMap())
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
	assert.Assert(t, json.Unmarshal(rawPattern, &pattern))
	assert.Assert(t, json.Unmarshal(rawMap, &resource))

	path, err := validateResourceElement(log.Log, resource, pattern, pattern, "/", common.NewAnchorMap())
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
	assert.Assert(t, json.Unmarshal(rawPattern, &pattern))
	assert.Assert(t, json.Unmarshal(rawMap, &resource))

	path, err := validateResourceElement(log.Log, resource, pattern, pattern, "/", common.NewAnchorMap())
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
	assert.Assert(t, json.Unmarshal(rawPattern, &pattern))
	assert.Assert(t, json.Unmarshal(rawMap, &resource))

	path, err := validateResourceElement(log.Log, resource, pattern, pattern, "/", common.NewAnchorMap())
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
	assert.Assert(t, json.Unmarshal(rawPattern, &pattern))
	assert.Assert(t, json.Unmarshal(rawMap, &resource))

	path, err := validateResourceElement(log.Log, resource, pattern, pattern, "/", common.NewAnchorMap())
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
	assert.Assert(t, json.Unmarshal(rawPattern, &pattern))
	assert.Assert(t, json.Unmarshal(rawMap, &resource))

	path, err := validateResourceElement(log.Log, resource, pattern, pattern, "/", common.NewAnchorMap())
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
	assert.Assert(t, json.Unmarshal(rawPattern, &pattern))
	assert.Assert(t, json.Unmarshal(rawMap, &resource))

	path, err := validateResourceElement(log.Log, resource, pattern, pattern, "/", common.NewAnchorMap())
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
	assert.Assert(t, json.Unmarshal(rawPattern, &pattern))
	assert.Assert(t, json.Unmarshal(rawMap, &resource))

	path, err := validateResourceElement(log.Log, resource, pattern, pattern, "/", common.NewAnchorMap())
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
	assert.Assert(t, json.Unmarshal(rawPattern, &pattern))
	assert.Assert(t, json.Unmarshal(rawMap, &resource))

	path, err := validateResourceElement(log.Log, resource, pattern, pattern, "/", common.NewAnchorMap())
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
	assert.Assert(t, json.Unmarshal(rawPattern, &pattern))
	assert.Assert(t, json.Unmarshal(rawMap, &resource))

	path, err := validateResourceElement(log.Log, resource, pattern, pattern, "/", common.NewAnchorMap())
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
	assert.Assert(t, json.Unmarshal(rawPattern, &pattern))
	assert.Assert(t, json.Unmarshal(rawMap, &resource))

	path, err := validateResourceElement(log.Log, resource, pattern, pattern, "/", common.NewAnchorMap())
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
	assert.Assert(t, json.Unmarshal(rawPattern, &pattern))
	assert.Assert(t, json.Unmarshal(rawMap, &resource))

	path, err := validateResourceElement(log.Log, resource, pattern, pattern, "/", common.NewAnchorMap())
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
	assert.Assert(t, json.Unmarshal(rawPattern, &pattern))
	assert.Assert(t, json.Unmarshal(rawMap, &resource))

	path, err := validateResourceElement(log.Log, resource, pattern, pattern, "/", common.NewAnchorMap())
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

	assert.Assert(t, json.Unmarshal(rawPattern, &pattern))

	pattern, err := actualizePattern(log.Log, pattern, referencePath, absolutePath)

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
	err := json.Unmarshal(rawPattern, &pattern)
	assert.NilError(t, err)
	err = json.Unmarshal(rawMap, &resource)
	assert.NilError(t, err)

	path, err := validateResourceElement(log.Log, resource, pattern, pattern, "/", common.NewAnchorMap())
	assert.Equal(t, path, "/0/object/0/key2/")
	assert.Assert(t, err != nil)
}

func TestValidateMapWildcardKeys(t *testing.T) {
	pattern := []byte(`{"metadata" : {"annotations": {"test/*": "value1"}}}`)
	resource := []byte(`{"metadata" : {"annotations": {"test/bar": "value1"}}}`)
	testValidationPattern(t, "1", pattern, resource, "", true)

	pattern = []byte(`{"metadata" : {"annotations": {"test/b??": "v*"}}}`)
	resource = []byte(`{"metadata" : {"annotations": {"test/bar": "value1"}}}`)
	testValidationPattern(t, "2", pattern, resource, "", true)

	pattern = []byte(`{}`)
	resource = []byte(`{"metadata" : {"annotations": {"test/bar": "value1"}}}`)
	testValidationPattern(t, "3", pattern, resource, "", true)

	pattern = []byte(`{"metadata" : {"annotations": {"test/b??": "v*"}}}`)
	resource = []byte(`{"metadata" : {"labels": {"test/bar": "value1"}}}`)
	testValidationPattern(t, "4", pattern, resource, "/metadata/annotations/", false)

	pattern = []byte(`{"metadata" : {"labels": {"*/test": "foo"}}}`)
	resource = []byte(`{"metadata" : {"labels": {"foo/test": "foo"}}}`)
	testValidationPattern(t, "5", pattern, resource, "", true)

	pattern = []byte(`{"metadata" : {"labels": {"foo/a*": "bar"}}}`)
	resource = []byte(`{"metadata" : {"labels": {"foo/aa?": "bar", "foo/789": "bar"}}}`)
	testValidationPattern(t, "6", pattern, resource, "", true)

	pattern = []byte(`{"metadata" : {"labels": {"foo/ABC*": "bar"}}}`)
	resource = []byte(`{"metadata" : {"labels": {"foo/AB?": "bar", "foo/ABC": "bar2"}}}`)
	testValidationPattern(t, "7", pattern, resource, "/metadata/labels/foo/ABC/", false)

	pattern = []byte(`{"=(metadata)" : {"=(labels)": {"foo/P*": "bar", "foo/Q*": "bar2"}}}`)
	resource = []byte(`{"metadata" : {"labels": {"foo/PQR": "bar", "foo/QR": "bar2"}}}`)
	testValidationPattern(t, "8", pattern, resource, "", true)

	pattern = []byte(`{"metadata" : {"labels": {"foo/1*": "bar"}}}`)
	resource = []byte(`{"metadata" : {"labels": {"foo/123": "bar222"}}}`)
	testValidationPattern(t, "9", pattern, resource, "/metadata/labels/foo/123/", false)

	pattern = []byte(`{"metadata" : {"labels": {"foo/X*": "bar", "foo/A*": "bar2"}}}`)
	resource = []byte(`{"metadata" : {"labels": {"foo/XYZ": "bar"}}}`)
	testValidationPattern(t, "10", pattern, resource, "/metadata/labels/foo/A*/", false)

	pattern = []byte(`{"=(metadata)" : {"=(labels)": {"foo/1*": "bar", "foo/4*": "bar2"}}}`)
	resource = []byte(`{"metadata" : {"labels": {"foo/123": "bar"}}}`)
	testValidationPattern(t, "11", pattern, resource, "/metadata/labels/foo/4*/", false)
}

func testValidationPattern(t *testing.T, num string, patternBytes []byte, resourceBytes []byte, path string, nilErr bool) {
	var pattern, resource interface{}
	err := json.Unmarshal(patternBytes, &pattern)
	assert.NilError(t, err)
	err = json.Unmarshal(resourceBytes, &resource)
	assert.NilError(t, err)

	p, err := validateResourceElement(log.Log, resource, pattern, pattern, "/", common.NewAnchorMap())
	assert.Equal(t, p, path, num)
	if nilErr {
		assert.NilError(t, err, num)
	} else {
		assert.Assert(t, err != nil, num)
	}
}

func TestConditionalAnchorWithMultiplePatterns(t *testing.T) {
	testCases := []struct {
		name     string
		pattern  []byte
		resource []byte
		nilErr   bool
	}{
		{
			name:     "test-1",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx:1.2.3", "imagePullPolicy": "Always"}]}}`),
			nilErr:   true,
		},
		{
			name:     "test-2",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx:latest", "imagePullPolicy": "Always"}]}}`),
			nilErr:   false,
		},
		{
			name:     "test-x",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "!*:* | *:latest","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx:latest", "imagePullPolicy": "Always"}]}}`),
			nilErr:   false,
		},
		{
			name:     "test-3",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx", "imagePullPolicy": "Always"}]}}`),
			nilErr:   false,
		},
		{
			name:     "test-4",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx", "imagePullPolicy": "Never"}]}}`),
			nilErr:   true,
		},
		{
			name:     "test-5",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx:latest", "imagePullPolicy": "Never"}]}}`),
			nilErr:   true,
		},
		{
			name:     "test-6",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx:1.2.3", "imagePullPolicy": "Never"}]}}`),
			nilErr:   true,
		},
		{
			name:     "test-7",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx", "imagePullPolicy": "Always"},{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"}]}}`),
			nilErr:   false,
		},
		{
			name:     "test-8",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx:latest", "imagePullPolicy": "Always"},{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"}]}}`),
			nilErr:   false,
		},
		{
			name:     "test-9",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx:1.2.3", "imagePullPolicy": "Always"},{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"}]}}`),
			nilErr:   true,
		},
		{
			name:     "test-10",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx", "imagePullPolicy": "Never"},{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"}]}}`),
			nilErr:   true,
		},
		{
			name:     "test-11",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx:latest", "imagePullPolicy": "Never"},{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"}]}}`),
			nilErr:   true,
		},
		{
			name:     "test-12",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx:1.2.3", "imagePullPolicy": "Never"},{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"}]}}`),
			nilErr:   true,
		},
		{
			name:     "test-13",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"},{"name": "nginx","image": "nginx", "imagePullPolicy": "Always"}]}}`),
			nilErr:   false,
		},
		{
			name:     "test-14",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"},{"name": "nginx","image": "nginx:latest", "imagePullPolicy": "Always"}]}}`),
			nilErr:   false,
		},
		{
			name:     "test-15",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"},{"name": "nginx","image": "nginx:1.2.3", "imagePullPolicy": "Always"}]}}`),
			nilErr:   true,
		},
		{
			name:     "test-16",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"},{"name": "nginx","image": "nginx", "imagePullPolicy": "Never"}]}}`),
			nilErr:   true,
		},
		{
			name:     "test-17",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"},{"name": "nginx","image": "nginx:latest", "imagePullPolicy": "Never"}]}}`),
			nilErr:   true,
		},
		{
			name:     "test-18",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"},{"name": "nginx","image": "nginx:1.2.3", "imagePullPolicy": "Never"}]}}`),
			nilErr:   true,
		},
		{
			name:     "test-19",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "busybox","image": "busybox", "imagePullPolicy": "Always"},{"name": "nginx","image": "nginx", "imagePullPolicy": "Always"}]}}`),
			nilErr:   false,
		},
		{
			name:     "test-20",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "busybox","image": "busybox:latest", "imagePullPolicy": "Always"},{"name": "nginx","image": "nginx:latest", "imagePullPolicy": "Always"}]}}`),
			nilErr:   false,
		},
		{
			name:     "test-21",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "busybox","image": "busybox:1.2.3", "imagePullPolicy": "Always"},{"name": "nginx","image": "nginx:1.2.3", "imagePullPolicy": "Always"}]}}`),
			nilErr:   true,
		},
	}

	for _, testCase := range testCases {
		var pattern, resource interface{}
		err := json.Unmarshal(testCase.pattern, &pattern)
		assert.NilError(t, err)
		err = json.Unmarshal(testCase.resource, &resource)
		assert.NilError(t, err)

		_, err = ValidateResourceWithPattern(log.Log, resource, pattern)
		if testCase.nilErr {
			assert.NilError(t, err, fmt.Sprintf("\ntest: %s\npattern: %s\nresource: %s\n", testCase.name, pattern, resource))
		} else {
			assert.Assert(t,
				err != nil,
				fmt.Sprintf("\ntest: %s\npattern: %s\nresource: %s\nmsg: %v", testCase.name, pattern, resource, err))
		}
	}
}
