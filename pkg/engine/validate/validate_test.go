package validate

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/engine/anchor"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"gotest.tools/v3/assert"
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

	path, err := validateMap(logr.Discard(), resource, pattern, pattern, "/", anchor.NewAnchorMap())
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
							"image":"registry.k8s.io/nginx-but-no-slim:0.8",
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

	path, err := validateMap(logr.Discard(), resource, pattern, pattern, "/", anchor.NewAnchorMap())
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
							"image":"registry.k8s.io/nginx-but-no-slim:0.8",
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

	path, err := validateMap(logr.Discard(), resource, pattern, pattern, "/", anchor.NewAnchorMap())
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
							"image":"registry.k8s.io/nginx-but-no-slim:0.8",
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

	path, err := validateMap(logr.Discard(), resource, pattern, pattern, "/", anchor.NewAnchorMap())
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
							"image":"registry.k8s.io/nginx-but-no-slim:0.8",
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

	path, err := validateMap(logr.Discard(), resource, pattern, pattern, "/", anchor.NewAnchorMap())
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
							"image":"registry.k8s.io/nginx-but-no-slim:0.8",
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

	path, err := validateMap(logr.Discard(), resource, pattern, pattern, "/", anchor.NewAnchorMap())
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
							"image":"registry.k8s.io/nginx-but-no-slim:0.8",
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

	path, err := validateMap(logr.Discard(), resource, pattern, pattern, "/", anchor.NewAnchorMap())
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

	path, err := validateResourceElement(logr.Discard(), resource, pattern, pattern, "/", anchor.NewAnchorMap())
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

	path, err := validateResourceElement(logr.Discard(), resource, pattern, pattern, "/", anchor.NewAnchorMap())
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

	pattern, err := variables.SubstituteAll(logr.Discard(), nil, pattern)
	assert.NilError(t, err)

	path, err := validateResourceElement(logr.Discard(), resource, pattern, pattern, "/", anchor.NewAnchorMap())
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

	path, err := validateResourceElement(logr.Discard(), resource, pattern, pattern, "/", anchor.NewAnchorMap())
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

	path, err := validateResourceElement(logr.Discard(), resource, pattern, pattern, "/", anchor.NewAnchorMap())
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

	path, err := validateResourceElement(logr.Discard(), resource, pattern, pattern, "/", anchor.NewAnchorMap())
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

	pattern, err := variables.SubstituteAll(logr.Discard(), nil, pattern)
	assert.NilError(t, err)

	path, err := validateResourceElement(logr.Discard(), resource, pattern, pattern, "/", anchor.NewAnchorMap())
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

	path, err := validateResourceElement(logr.Discard(), resource, pattern, pattern, "/", anchor.NewAnchorMap())
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

	pattern, err := variables.SubstituteAll(logr.Discard(), nil, pattern)
	assert.NilError(t, err)

	path, err := validateResourceElement(logr.Discard(), resource, pattern, pattern, "/", anchor.NewAnchorMap())
	assert.Equal(t, path, "")
	assert.Assert(t, err == nil)
}

func TestValidateMap_AbsolutePathToMetadata(t *testing.T) {
	rawPattern := []byte(`{
		"spec":{
			"containers":[
				{
					"(name)":"kyverno",
					"image":"kyverno.io*"
				}
			]
		}
	}`)

	rawMap := []byte(`{
		"spec":{
			"containers":[
				{
					"name":"kyverno",
					"image": "kyverno.io/test:latest"
				}
			]
		}
	}`)

	var pattern, resource interface{}
	assert.Assert(t, json.Unmarshal(rawPattern, &pattern))
	assert.Assert(t, json.Unmarshal(rawMap, &resource))

	path, err := validateResourceElement(logr.Discard(), resource, pattern, pattern, "/", anchor.NewAnchorMap())
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

	pattern, err := variables.SubstituteAll(logr.Discard(), nil, pattern)
	assert.NilError(t, err)

	path, err := validateResourceElement(logr.Discard(), resource, pattern, pattern, "/", anchor.NewAnchorMap())
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

	path, err := validateResourceElement(logr.Discard(), resource, pattern, pattern, "/", anchor.NewAnchorMap())
	assert.Equal(t, path, "/spec/containers/0/resources/requests/memory/")
	assert.Assert(t, err != nil)
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

	path, err := validateResourceElement(logr.Discard(), resource, pattern, pattern, "/", anchor.NewAnchorMap())
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

	p, err := validateResourceElement(logr.Discard(), resource, pattern, pattern, "/", anchor.NewAnchorMap())
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
		status   engineapi.RuleStatus
	}{
		{
			name:     "test-1",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx:1.2.3", "imagePullPolicy": "Always"}]}}`),
			status:   engineapi.RuleStatusSkip,
		},
		{
			name:     "test-2",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx:latest", "imagePullPolicy": "Always"}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-3",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx", "imagePullPolicy": "Always"}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-4",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx", "imagePullPolicy": "Never"}]}}`),
			status:   engineapi.RuleStatusPass,
		},
		{
			name:     "test-5",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx:latest", "imagePullPolicy": "Never"}]}}`),
			status:   engineapi.RuleStatusPass,
		},
		{
			name:     "test-6",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx:1.2.3", "imagePullPolicy": "Never"}]}}`),
			status:   engineapi.RuleStatusSkip,
		},
		{
			name:     "test-7",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx", "imagePullPolicy": "Always"},{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-8",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx:latest", "imagePullPolicy": "Always"},{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-9",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx:1.2.3", "imagePullPolicy": "Always"},{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"}]}}`),
			status:   engineapi.RuleStatusSkip,
		},
		{
			name:     "test-10",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx", "imagePullPolicy": "Never"},{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"}]}}`),
			status:   engineapi.RuleStatusPass,
		},
		{
			name:     "test-11",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx:latest", "imagePullPolicy": "Never"},{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"}]}}`),
			status:   engineapi.RuleStatusPass,
		},
		{
			name:     "test-12",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx:1.2.3", "imagePullPolicy": "Never"},{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"}]}}`),
			status:   engineapi.RuleStatusSkip,
		},
		{
			name:     "test-13",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"},{"name": "nginx","image": "nginx", "imagePullPolicy": "Always"}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-14",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"},{"name": "nginx","image": "nginx:latest", "imagePullPolicy": "Always"}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-15",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"},{"name": "nginx","image": "nginx:1.2.3", "imagePullPolicy": "Always"}]}}`),
			status:   engineapi.RuleStatusSkip,
		},
		{
			name:     "test-16",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"},{"name": "nginx","image": "nginx", "imagePullPolicy": "Never"}]}}`),
			status:   engineapi.RuleStatusPass,
		},
		{
			name:     "test-17",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"},{"name": "nginx","image": "nginx:latest", "imagePullPolicy": "Never"}]}}`),
			status:   engineapi.RuleStatusPass,
		},
		{
			name:     "test-18",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "busybox","image": "busybox:1.28", "imagePullPolicy": "Always"},{"name": "nginx","image": "nginx:1.2.3", "imagePullPolicy": "Never"}]}}`),
			status:   engineapi.RuleStatusSkip,
		},
		{
			name:     "test-19",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "busybox","image": "busybox", "imagePullPolicy": "Always"},{"name": "nginx","image": "nginx", "imagePullPolicy": "Always"}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-20",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "busybox","image": "busybox:latest", "imagePullPolicy": "Always"},{"name": "nginx","image": "nginx:latest", "imagePullPolicy": "Always"}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-21",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "*:latest | !*:*","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "busybox","image": "busybox:1.2.3", "imagePullPolicy": "Always"},{"name": "nginx","image": "nginx:latest", "imagePullPolicy": "IfNotPresent"}]}}`),
			status:   engineapi.RuleStatusPass,
		},
		{
			name:     "test-22",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","(image)": "!*:* | *:latest","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx:latest", "imagePullPolicy": "Always"}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-23",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","<(image)": "*:latest","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx", "imagePullPolicy": "Always"}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-24",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","<(image)": "*:latest","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx:latest", "imagePullPolicy": "Always"}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-25",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","<(image)": "nginx", "env": [{"<(name)": "foo", "<(value)": "bar" }],"imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx", "env": [{"name": "foo1", "value": "bar" }],"imagePullPolicy": "Always"}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-26",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","<(image)": "nginx", "env": [{"<(name)": "foo", "<(value)": "bar" }],"imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx", "env": [{"name": "foo", "value": "bar" }],"imagePullPolicy": "Always"}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-27",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*", "env": [{"<(name)": "foo", "<(value)": "bar" }],"imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx", "env": [{"name": "foo1", "value": "bar" }],"imagePullPolicy": "Always"}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-28",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*", "env": [{"<(name)": "foo", "<(value)": "bar" }],"imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx", "env": [{"name": "foo", "value": "bar" }],"imagePullPolicy": "IfNotpresent"}]}}`),
			status:   engineapi.RuleStatusPass,
		},
		{
			name:     "test-29",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*", "env": [{"<(name)": "foo", "<(value)": "bar" }],"imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx", "env": [{"name": "foo", "value": "bar" }],"imagePullPolicy": "Always"}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-30",
			pattern:  []byte(`{"metadata": {"<(name)": "nginx"},"spec": {"imagePullSecrets": [{"name": "regcred"}]}}`),
			resource: []byte(`{"metadata": {"name": "somename"},"spec": {"containers": [{"name": "nginx","image": "nginx:latest"}], "imagePullSecrets": [{"name": "cred"}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-31",
			pattern:  []byte(`{"metadata": {"<(name)": "nginx"},"spec": {"imagePullSecrets": [{"name": "regcred"}]}}`),
			resource: []byte(`{"metadata": {"name": "nginx"},"spec": {"containers": [{"name": "nginx","image": "nginx:latest"}], "imagePullSecrets": [{"name": "cred"}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-32",
			pattern:  []byte(`{"metadata": {"labels": {"<(foo)": "bar"}},"spec": {"containers": [{"name": "nginx","image": "!*:latest"}]}}`),
			resource: []byte(`{"metadata": {"name": "nginx","labels": {"foo": "bar"}},"spec": {"containers": [{"name": "nginx","image": "nginx"}]}}`),
			status:   engineapi.RuleStatusPass,
		},
		{
			name:     "test-33",
			pattern:  []byte(`{"metadata": {"labels": {"<(foo)": "bar"}},"spec": {"containers": [{"name": "nginx","image": "!*:latest"}]}}`),
			resource: []byte(`{"metadata": {"name": "nginx","labels": {"foo": "bar"}},"spec": {"containers": [{"name": "nginx","image": "nginx:latest"}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-34",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","<(image)": "nginx"}],"imagePullSecrets": [{"name": "my-registry-secret"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx"}], "imagePullSecrets": [{"name": "cred"}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-35",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","<(image)": "nginx"}],"imagePullSecrets": [{"name": "my-registry-secret"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "somepod"}], "imagePullSecrets": [{"name": "cred"}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-36",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","<(image)": "nginx"}],"imagePullSecrets": [{"name": "my-registry-secret"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx"}], "imagePullSecrets": [{"name": "my-registry-secret"}]}}`),
			status:   engineapi.RuleStatusPass,
		},
		{
			name:     "test-37",
			pattern:  []byte(`{"metadata": {"labels": {"allow-docker": "true"}},"(spec)": {"(volumes)": [{"(hostPath)": {"path": "/var/run/docker.sock"}}]}}`),
			resource: []byte(`{"metadata": {"labels": {"run": "nginx"},"name": "nginx"},"spec": {"containers": [{"image": "nginx","name": "nginx"}]}}`),
			status:   engineapi.RuleStatusSkip,
		},
		{
			name:     "test-38",
			pattern:  []byte(`{"metadata": {"labels": {"allow-docker": "true"}},"(spec)": {"(volumes)": [{"(hostPath)": {"path": "/var/run/docker.sock"}}]}}`),
			resource: []byte(`{"metadata": {"labels": {"run": "nginx"},"name": "nginx"},"spec": {"containers": [{"image": "nginx","name": "nginx"}],"volumes": [{"hostPath": {"path": "/var/run/docker.sock"}}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-39",
			pattern:  []byte(`{"metadata": {"labels": {"allow-docker": "true"}},"(spec)": {"(volumes)": [{"(hostPath)": {"path": "/var/run/docker.sock"}}]}}`),
			resource: []byte(`{"metadata": {"labels": {"run": "nginx"},"name": "nginx"},"spec": {"containers": [{"image": "nginx","name": "nginx"}],"volumes": [{"hostPath": {"path": "/randome/value"}}]}}`),
			status:   engineapi.RuleStatusSkip,
		},
		{
			name:     "test-40",
			pattern:  []byte(`{"metadata": {"labels": {"allow-docker": "true"}},"(spec)": {"(volumes)": [{"(hostPath)": {"path": "/var/run/docker.sock"}}]}}`),
			resource: []byte(`{"metadata": {"labels": {"run": "nginx","allow-docker": "true"},"name": "nginx"},"spec": {"containers": [{"image": "nginx","name": "nginx"}],"volumes": [{"hostPath": {"path": "/var/run/docker.sock"}}]}}`),
			status:   engineapi.RuleStatusPass,
		},
		{
			name:     "test-41",
			pattern:  []byte(`{"metadata": {"labels": {"allow-docker": "true"}},"(spec)": {"(volumes)": [{"(hostPath)": {"path": "/var/run/docker.sock"}}]}}`),
			resource: []byte(`{"metadata": {"labels": {"run": "nginx","allow-docker": "false"},"name": "nginx"},"spec": {"containers": [{"image": "nginx","name": "nginx"}],"volumes": [{"hostPath": {"path": "/var/run/docker.sock"}}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-42",
			pattern:  []byte(`{"metadata": {"labels": {"allow-docker": "true"}},"(spec)": {"(volumes)": [{"(hostPath)": {"path": "/var/run/docker.sock"}}]}}`),
			resource: []byte(`{"metadata": {"labels": {"run": "nginx"},"name": "nginx"},"spec": {"containers": [{"image": "nginx","name": "nginx"}],"volumes": [{"hostPath": {"path": "/var/run/docker.sock"}}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-43",
			pattern:  []byte(`{"spec": {"=(volumes)": [{"(name)": "!cache-volume","=(emptyDir)": {"sizeLimit": "?*"}}]}}`),
			resource: []byte(`{"spec": {"volumes": [{"name": "cache-volume","emptyDir": {}}]}}`),
			status:   engineapi.RuleStatusSkip,
		},
		{
			name:     "test-44",
			pattern:  []byte(`{"spec": {"=(initContainers)": [{"(name)": "!istio-init", "=(securityContext)": {"=(runAsUser)": ">0"}}], "=(containers)": [{"=(securityContext)": {"=(runAsUser)": ">0"}}]}}`),
			resource: []byte(`{"spec": {"initContainers": [{"name": "nginx", "securityContext": {"runAsUser": 1000}}], "containers": [{"name": "nginx", "image": "nginx"}]}}`),
			status:   engineapi.RuleStatusPass,
		},
		{
			name:     "test-45",
			pattern:  []byte(`{"spec": {"=(initContainers)": [{"(name)": "!istio-init", "=(securityContext)": {"=(runAsUser)": ">0"}}], "=(containers)": [{"=(securityContext)": {"=(runAsUser)": ">0"}}]}}`),
			resource: []byte(`{"spec": {"initContainers": [{"name": "nginx", "securityContext": {"runAsUser": 0}}], "containers": [{"name": "nginx", "image": "nginx"}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-46",
			pattern:  []byte(`{"spec": {"=(initContainers)": [{"(name)": "!istio-init", "=(securityContext)": {"=(runAsUser)": ">0"}}], "=(containers)": [{"=(securityContext)": {"=(runAsUser)": ">0"}}]}}`),
			resource: []byte(`{"spec": {"initContainers": [{"name": "istio-init", "securityContext": {"runAsUser": 0}}], "containers": [{"securityContext": {"runAsUser": 1000}}]}}`),
			status:   engineapi.RuleStatusPass,
		},
		{
			name:     "test-47",
			pattern:  []byte(`{"spec": {"=(initContainers)": [{"(name)": "!istio-init", "=(securityContext)": {"=(runAsUser)": ">0"}}], "=(containers)": [{"=(securityContext)": {"=(runAsUser)": ">0"}}]}}`),
			resource: []byte(`{"spec": {"initContainers": [{"name": "istio-init", "securityContext": {"runAsUser": 1000}}], "containers": [{"securityContext": {"runAsUser": 0}}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-48",
			pattern:  []byte(`{"spec": {"=(initContainers)": [{"(name)": "!istio-init", "=(securityContext)": {"=(runAsUser)": ">0"}}], "=(containers)": [{"=(securityContext)": {"=(runAsUser)": ">0"}}]}}`),
			resource: []byte(`{"spec": {"containers": [{"securityContext": {"runAsUser": 1000}}]}}`),
			status:   engineapi.RuleStatusPass,
		},
		{
			name:     "test-49",
			pattern:  []byte(`{"spec": {"=(initContainers)": [{"(name)": "!istio-init", "=(securityContext)": {"=(runAsUser)": ">0"}}], "=(containers)": [{"=(securityContext)": {"=(runAsUser)": ">0"}}]}}`),
			resource: []byte(`{"spec": {"containers": [{"securityContext": {"runAsUser": 0}}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "test-50",
			pattern:  []byte(`{"spec": {"=(initContainers)": [{"(name)": "!istio-init", "=(securityContext)": {"=(runAsUser)": ">0"}}], "=(containers)": [{"=(securityContext)": {"=(runAsUser)": ">0"}}]}}`),
			resource: []byte(`{"spec": {"initContainers": [{"name": "istio-init", "securityContext": {"runAsUser": 0}}], "containers": [{"name": "nginx", "image": "nginx"}]}}`),
			status:   engineapi.RuleStatusPass,
		},
		{
			name:     "test-51",
			pattern:  []byte(`{"spec": {"=(volumes)": [{"(name)": "!credential-socket&!istio-data&!istio-envoy&!workload-certs&!workload-socket","=(emptyDir)": {"sizeLimit": "?*"}}]}}`),
			resource: []byte(`{"spec": {"volumes": [{"name": "credential-socket","emptyDir": {"sizeLimit": "1Gi"}}]}}`),
			status:   engineapi.RuleStatusSkip,
		},
		{
			name:     "test-52",
			pattern:  []byte(`{"spec": {"=(volumes)": [{"(name)": "!credential-socket&!istio-data&!istio-envoy&!workload-certs&!workload-socket","=(emptyDir)": {"sizeLimit": "?*"}}]}}`),
			resource: []byte(`{"spec": {"volumes": [{"name": "cache-volume","emptyDir": {"sizeLimit": "1Gi"}}]}}`),
			status:   engineapi.RuleStatusPass,
		},
		{
			name:     "test-53",
			pattern:  []byte(`{"spec": {"=(volumes)": [{"(name)": "!credential-socket&!istio-data&!istio-envoy&!workload-certs&!workload-socket","=(emptyDir)": {"sizeLimit": "?*"}}]}}`),
			resource: []byte(`{"spec": {"volumes": [{"name": "cache-volume"}]}}`),
			status:   engineapi.RuleStatusPass,
		},
		{
			name:     "test-54",
			pattern:  []byte(`{"spec": {"=(volumes)": [{"(name)": "!credential-socket&!istio-data&!istio-envoy&!workload-certs&!workload-socket","=(emptyDir)": {"sizeLimit": "?*"}}]}}`),
			resource: []byte(`{"spec": {"volumes": [{"name": "cache-volume","emptyDir": {}}]}}`),
			status:   engineapi.RuleStatusFail,
		},
	}

	for _, testCase := range testCases {
		testMatchPattern(t, testCase)
	}
}

func Test_global_anchor(t *testing.T) {
	testCases := []struct {
		name     string
		pattern  []byte
		resource []byte
		status   engineapi.RuleStatus
	}{
		{
			name:     "check_global_anchor_skip",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","<(image)": "*:latest","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx:v1", "imagePullPolicy": "Always"}]}}`),
			status:   engineapi.RuleStatusSkip,
		},
		{
			name:     "check_global_anchor_fail",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","<(image)": "*:latest","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx:latest", "imagePullPolicy": "Always"}]}}`),
			status:   engineapi.RuleStatusFail,
		},
		{
			name:     "check_global_anchor_pass",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","<(image)": "*:latest","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx:latest", "imagePullPolicy": "IfNotPresent"}]}}`),
			status:   engineapi.RuleStatusPass,
		},
		{
			name:     "check_global_anchor_mixed",
			pattern:  []byte(`{"spec": {"containers": [{"name": "*","<(image)": "*:latest","imagePullPolicy": "!Always"}]}}`),
			resource: []byte(`{"spec": {"containers": [{"name": "nginx","image": "nginx:latest", "imagePullPolicy": "IfNotPresent"},{"name": "nginx","image": "nginx:v2", "imagePullPolicy": "IfNotPresent"}]}}`),
			status:   engineapi.RuleStatusPass,
		},
	}

	for i := range testCases {
		testMatchPattern(t, testCases[i])
	}
}

func testMatchPattern(t *testing.T, testCase struct {
	name     string
	pattern  []byte
	resource []byte
	status   engineapi.RuleStatus
},
) {
	var pattern, resource interface{}
	err := json.Unmarshal(testCase.pattern, &pattern)
	assert.NilError(t, err)
	err = json.Unmarshal(testCase.resource, &resource)
	assert.NilError(t, err)

	err = MatchPattern(logr.Discard(), resource, pattern)

	if testCase.status == engineapi.RuleStatusPass {
		assert.NilError(t, err, fmt.Sprintf("\nexpected pass - test: %s\npattern: %s\nresource: %s\n", testCase.name, pattern, resource))
	} else if testCase.status == engineapi.RuleStatusSkip {
		assert.Assert(t, err != nil, fmt.Sprintf("\nexpected skip error - test: %s\npattern: %s\nresource: %s\n", testCase.name, pattern, resource))
		pe, ok := err.(*PatternError)
		if !ok {
			assert.Assert(t, err != nil, fmt.Sprintf("\ninvalid error type - test: %s\npattern: %s\nresource: %s\n", testCase.name, pattern, resource))
		}

		assert.Assert(t, pe.Skip, fmt.Sprintf("\nexpected skip == true - test: %s\npattern: %s\nresource: %s\n", testCase.name, pattern, resource))
	} else if testCase.status == engineapi.RuleStatusError {
		assert.Assert(t, err == nil, fmt.Sprintf("\nexpected error - test: %s\npattern: %s\nresource: %s\n", testCase.name, pattern, resource))
	}
}

// TestPatternError_Unwrap guards the error chain that anchor classification
// depends on. Anchor errors are aggregated into a *PatternError by validateMap,
// validateArray and validateArrayOfMaps before being handed back to skip() and
// fail(), so a *PatternError must stay transparent to errors.Is/errors.As.
func TestPatternError_Unwrap(t *testing.T) {
	inner := fmt.Errorf("inner")
	pe := &PatternError{Err: inner, Path: "/spec/", Skip: true}

	assert.Assert(t, errors.Is(pe, inner), "PatternError must unwrap to its cause")
}

// TestMatchPattern_AnchorClassificationThroughPatternError verifies end to end
// that a conditional anchor mismatch nested in an array of maps is still
// classified as a skip after travelling through the *PatternError aggregation
// layer. This is the common case of selecting a container by name.
func TestMatchPattern_AnchorClassificationThroughPatternError(t *testing.T) {
	var pattern, resource interface{}
	err := json.Unmarshal([]byte(`{"spec":{"containers":[{"(name)":"nginx","image":"nginx:latest"}]}}`), &pattern)
	assert.NilError(t, err)
	err = json.Unmarshal([]byte(`{"spec":{"containers":[{"name":"redis","image":"redis:7"}]}}`), &resource)
	assert.NilError(t, err)

	err = MatchPattern(logr.Discard(), resource, pattern)
	assert.Assert(t, err != nil)

	pe, ok := err.(*PatternError)
	assert.Assert(t, ok, "expected a *PatternError, got %T", err)
	assert.Assert(t, pe.Skip, "conditional anchor mismatch must be reported as a skip")
	assert.Assert(t, anchor.IsConditionalAnchorError(pe), "anchor type must survive PatternError aggregation")
}

// TestMatchPattern_ResourceValueMimickingAnchorMessage is the end to end
// regression test for the removed strings.Contains classification. A resource
// value that happens to contain an anchor error message must not change the
// outcome of the rule: it is a plain mismatch, not a skip.
func TestMatchPattern_ResourceValueMimickingAnchorMessage(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "control", value: "intruder"},
		{name: "conditional anchor message", value: "conditional anchor mismatch"},
		{name: "global anchor message", value: "global anchor mismatch"},
		{name: "negation anchor message", value: "negation anchor matched in resource"},
		{name: "anchor message as substring", value: "x-conditional anchor mismatch-y"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pattern, resource interface{}
			err := json.Unmarshal([]byte(`{"metadata":{"annotations":{"owner":"platform-team"}}}`), &pattern)
			assert.NilError(t, err)
			raw := fmt.Sprintf(`{"metadata":{"annotations":{"owner":%q}}}`, tt.value)
			err = json.Unmarshal([]byte(raw), &resource)
			assert.NilError(t, err)

			err = MatchPattern(logr.Discard(), resource, pattern)
			assert.Assert(t, err != nil, "expected a validation error")

			pe, ok := err.(*PatternError)
			assert.Assert(t, ok, "expected a *PatternError, got %T", err)
			assert.Assert(t, !pe.Skip, "a plain value mismatch must not be reported as a skip")
			assert.Equal(t, pe.Path, "/metadata/annotations/owner/")
		})
	}
}

// Test_negation_anchor covers end to end evaluation of the negation anchor.
// fail() is the only consumer of anchor.IsNegationAnchorError, so this is the
// path most affected by moving classification from message text to error type.
// Assertions are explicit rather than routed through testMatchPattern, which
// has no RuleStatusFail branch.
func Test_negation_anchor(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		resource string
		wantPass bool
	}{{
		name:     "negated key absent, rule passes",
		pattern:  `{"spec":{"X(hostNetwork)":"*"}}`,
		resource: `{"spec":{"containers":[{"name":"a"}]}}`,
		wantPass: true,
	}, {
		name:     "negated key present, rule fails",
		pattern:  `{"spec":{"X(hostNetwork)":"*"}}`,
		resource: `{"spec":{"hostNetwork":true}}`,
		wantPass: false,
	}, {
		name:     "negated key absent in array of maps, rule passes",
		pattern:  `{"spec":{"containers":[{"X(securityContext)":"*"}]}}`,
		resource: `{"spec":{"containers":[{"name":"a"}]}}`,
		wantPass: true,
	}, {
		name:     "negated key present in array of maps, rule fails",
		pattern:  `{"spec":{"containers":[{"X(securityContext)":"*"}]}}`,
		resource: `{"spec":{"containers":[{"name":"a","securityContext":{"privileged":true}}]}}`,
		wantPass: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pattern, resource interface{}
			err := json.Unmarshal([]byte(tt.pattern), &pattern)
			assert.NilError(t, err)
			err = json.Unmarshal([]byte(tt.resource), &resource)
			assert.NilError(t, err)

			err = MatchPattern(logr.Discard(), resource, pattern)
			if tt.wantPass {
				assert.NilError(t, err)
				return
			}

			assert.Assert(t, err != nil, "expected the rule to fail")
			pe, ok := err.(*PatternError)
			assert.Assert(t, ok, "expected a *PatternError, got %T", err)
			// a negation match is a rule failure, never a skip
			assert.Assert(t, !pe.Skip, "negation anchor match must not be reported as a skip")
			// and it must still be recognised as a negation anchor error
			assert.Assert(t, anchor.IsNegationAnchorError(pe), "negation anchor error must survive propagation")
			assert.Assert(t, !anchor.IsConditionalAnchorError(pe))
			assert.Assert(t, !anchor.IsGlobalAnchorError(pe))
		})
	}
}

// Test_existence_anchor_nestedConditional guards the boundary in
// validateExistenceListResource, which deliberately does not propagate the
// per element errors it collected. If that aggregation ever became transparent
// to errors.As, a nested conditional mismatch would leak out and turn an
// existence anchor failure into a skip, silently disabling the rule.
func Test_existence_anchor_nestedConditional(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		wantPass bool
	}{{
		name:     "an element satisfies the existence anchor",
		resource: `{"spec":{"containers":[{"name":"nginx","image":"nginx:latest"}]}}`,
		wantPass: true,
	}, {
		name:     "no element satisfies the conditional anchor nested in the existence anchor",
		resource: `{"spec":{"containers":[{"name":"redis","image":"redis:7"}]}}`,
		wantPass: false,
	}, {
		name:     "conditional anchor matches but the nested value does not",
		resource: `{"spec":{"containers":[{"name":"nginx","image":"nginx:v1"}]}}`,
		wantPass: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pattern, resource interface{}
			err := json.Unmarshal([]byte(`{"spec":{"^(containers)":[{"(name)":"nginx","image":"nginx:latest"}]}}`), &pattern)
			assert.NilError(t, err)
			err = json.Unmarshal([]byte(tt.resource), &resource)
			assert.NilError(t, err)

			err = MatchPattern(logr.Discard(), resource, pattern)
			if tt.wantPass {
				assert.NilError(t, err)
				return
			}

			assert.Assert(t, err != nil, "expected the rule to fail")
			pe, ok := err.(*PatternError)
			assert.Assert(t, ok, "expected a *PatternError, got %T", err)
			assert.Assert(t, !pe.Skip, "an existence anchor failure must not be reported as a skip")
			assert.Assert(t, !anchor.IsConditionalAnchorError(pe), "the nested conditional error must not leak out of the existence anchor")
		})
	}
}
