package engine

import (
	"encoding/json"
	"reflect"
	"testing"

	jsonpatch "github.com/evanphx/json-patch"
	"gotest.tools/assert"
)

func compareJSONAsMap(t *testing.T, expected, actual []byte) {
	var expectedMap, actualMap map[string]interface{}
	assert.NilError(t, json.Unmarshal(expected, &expectedMap))
	assert.NilError(t, json.Unmarshal(actual, &actualMap))
	assert.Assert(t, reflect.DeepEqual(expectedMap, actualMap))
}

func TestProcessOverlayPatches_NestedListWithAnchor(t *testing.T) {
	resourceRaw := []byte(`
	 {  
		"apiVersion":"v1",
		"kind":"Endpoints",
		"metadata":{  
		   "name":"test-endpoint",
		   "labels":{  
			  "label":"test"
		   }
		},
		"subsets":[  
		   {  
			  "addresses":[  
				 {  
					"ip":"192.168.10.171"
				 }
			  ],
			  "ports":[  
				 {  
					"name":"secure-connection",
					"port":443,
					"protocol":"TCP"
				 }
			  ]
		   }
		]
	 }`)

	overlayRaw := []byte(`
	 {  
		"subsets":[  
		   {  
			  "ports":[  
				 {  
					"(name)":"secure-connection",
					"port":444,
					"protocol":"UDP"
				 }
			  ]
		   }
		]
	 }`)

	var resource, overlay interface{}

	json.Unmarshal(resourceRaw, &resource)
	json.Unmarshal(overlayRaw, &overlay)

	patches, overlayerr := processOverlayPatches(resource, overlay)
	assert.Assert(t, reflect.DeepEqual(overlayerr, overlayError{}))
	assert.Assert(t, patches != nil)

	patch := JoinPatches(patches)
	decoded, err := jsonpatch.DecodePatch(patch)
	assert.NilError(t, err)
	assert.Assert(t, decoded != nil)

	patched, err := decoded.Apply(resourceRaw)
	assert.NilError(t, err)
	assert.Assert(t, patched != nil)

	expectedResult := []byte(`
	 {  
		"apiVersion":"v1",
		"kind":"Endpoints",
		"metadata":{  
		   "name":"test-endpoint",
		   "labels":{  
			  "label":"test"
		   }
		},
		"subsets":[  
		   {  
			  "addresses":[  
				 {  
					"ip":"192.168.10.171"
				 }
			  ],
			  "ports":[  
				 {  
					"name":"secure-connection",
					"port":444.000000,
					"protocol":"UDP"
				 }
			  ]
		   }
		]
	 }`)

	compareJSONAsMap(t, expectedResult, patched)
}

func TestProcessOverlayPatches_InsertIntoArray(t *testing.T) {
	resourceRaw := []byte(`
	 {  
		"apiVersion":"v1",
		"kind":"Endpoints",
		"metadata":{  
		   "name":"test-endpoint",
		   "labels":{  
			  "label":"test"
		   }
		},
		"subsets":[  
		   {  
			  "addresses":[  
				 {  
					"ip":"192.168.10.171"
				 }
			  ],
			  "ports":[  
				 {  
					"name":"secure-connection",
					"port":443,
					"protocol":"TCP"
				 }
			  ]
		   }
		]
	 }`)
	overlayRaw := []byte(`
	 {  
		"subsets":[  
		   {  
			  "addresses":[  
				 {  
					"ip":"192.168.10.172"
				 },
				 {  
					"ip":"192.168.10.173"
				 }
			  ],
			  "ports":[  
				 {  
					"name":"insecure-connection",
					"port":80,
					"protocol":"UDP"
				 }
			  ]
		   }
		]
	 }`)

	var resource, overlay interface{}

	json.Unmarshal(resourceRaw, &resource)
	json.Unmarshal(overlayRaw, &overlay)

	patches, overlayerr := processOverlayPatches(resource, overlay)
	assert.Assert(t, reflect.DeepEqual(overlayerr, overlayError{}))
	assert.Assert(t, patches != nil)

	patch := JoinPatches(patches)

	decoded, err := jsonpatch.DecodePatch(patch)
	assert.NilError(t, err)
	assert.Assert(t, decoded != nil)

	patched, err := decoded.Apply(resourceRaw)
	assert.NilError(t, err)
	assert.Assert(t, patched != nil)

	expectedResult := []byte(`{  
		"apiVersion":"v1",
		"kind":"Endpoints",
		"metadata":{  
		   "name":"test-endpoint",
		   "labels":{  
			  "label":"test"
		   }
		},
		"subsets":[  
		   {  
			  "addresses":[  
				 {  
					"ip":"192.168.10.171"
				 }
			  ],
			  "ports":[  
				 {  
					"name":"secure-connection",
					"port":443,
					"protocol":"TCP"
				 }
			  ]
		   },
		   {  
			  "addresses":[  
				 {  
					"ip":"192.168.10.172"
				 },
				 {  
					"ip":"192.168.10.173"
				 }
			  ],
			  "ports":[  
				 {  
					"name":"insecure-connection",
					"port":80,
					"protocol":"UDP"
				 }
			  ]
		   }
		]
	 }`)

	compareJSONAsMap(t, expectedResult, patched)
}

func TestProcessOverlayPatches_TestInsertToArray(t *testing.T) {
	overlayRaw := []byte(`
	 {  
		"spec":{  
		   "template":{  
			  "spec":{  
				 "containers":[  
					{  
					   "name":"pi1",
					   "image":"vasylev.perl"
					}
				 ]
			  }
		   }
		}
	 }`)
	resourceRaw := []byte(`{  
		"apiVersion":"batch/v1",
		"kind":"Job",
		"metadata":{  
		   "name":"pi"
		},
		"spec":{  
		   "template":{  
			  "spec":{  
				 "containers":[  
					{  
					   "name":"piv0",
					   "image":"perl",
					   "command":[  
						  "perl"
					   ]
					},
					{  
					   "name":"pi",
					   "image":"perl",
					   "command":[  
						  "perl"
					   ]
					},
					{  
					   "name":"piv1",
					   "image":"perl",
					   "command":[  
						  "perl"
					   ]
					}
				 ],
				 "restartPolicy":"Never"
			  }
		   },
		   "backoffLimit":4
		}
	 }`)

	var resource, overlay interface{}

	json.Unmarshal(resourceRaw, &resource)
	json.Unmarshal(overlayRaw, &overlay)

	patches, overlayerr := processOverlayPatches(resource, overlay)
	assert.Assert(t, reflect.DeepEqual(overlayerr, overlayError{}))
	assert.Assert(t, patches != nil)

	patch := JoinPatches(patches)

	decoded, err := jsonpatch.DecodePatch(patch)
	assert.NilError(t, err)
	assert.Assert(t, decoded != nil)

	patched, err := decoded.Apply(resourceRaw)
	assert.NilError(t, err)
	assert.Assert(t, patched != nil)
}

func TestProcessOverlayPatches_ImagePullPolicy(t *testing.T) {
	overlayRaw := []byte(`{
		"spec": {
			"template": {
				"spec": {
					"containers": [
						{
							"(image)": "*:latest",
							"imagePullPolicy": "IfNotPresent",
							"ports": [
								{
									"containerPort": 8080
								}
							]
						}
					]
				}
			}
		}
	}`)
	resourceRaw := []byte(`{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
			"name": "nginx-deployment",
			"labels": {
				"app": "nginx"
			}
		},
		"spec": {
			"replicas": 1,
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
							"image": "nginx:latest",
							"ports": [
								{
									"containerPort": 80
								}
							]
						},
						{
							"name": "ghost",
							"image": "ghost:latest"
						}
					]
				}
			}
		}
	}`)

	var resource, overlay interface{}

	json.Unmarshal(resourceRaw, &resource)
	json.Unmarshal(overlayRaw, &overlay)

	patches, overlayerr := processOverlayPatches(resource, overlay)
	assert.Assert(t, reflect.DeepEqual(overlayerr, overlayError{}))
	assert.Assert(t, len(patches) != 0)

	doc, err := ApplyPatches(resourceRaw, patches)
	assert.NilError(t, err)
	expectedResult := []byte(`{  
		"apiVersion":"apps/v1",
		"kind":"Deployment",
		"metadata":{  
		   "name":"nginx-deployment",
		   "labels":{  
			  "app":"nginx"
		   }
		},
		"spec":{  
		   "replicas":1,
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
					   "image":"nginx:latest",
					   "imagePullPolicy":"IfNotPresent",
					   "name":"nginx",
					   "ports":[  
						  {  
							 "containerPort":80
						  },
						  {  
							 "containerPort":8080
						  }
					   ]
					},
					{  
					   "image":"ghost:latest",
					   "imagePullPolicy":"IfNotPresent",
					   "name":"ghost",
					   "ports":[  
						  {  
							 "containerPort":8080
						  }
					   ]
					}
				 ]
			  }
		   }
		}
	 }`)

	compareJSONAsMap(t, expectedResult, doc)

	overlayRaw = []byte(`{
		"spec": {
			"template": {
				"metadata": {
					"labels": {
						"(app)": "nginx"
					}
				},
				"spec": {
					"containers": [
						{
							"(image)": "*:latest",
							"imagePullPolicy": "IfNotPresent",
							"ports": [
								{
									"containerPort": 8080
								}
							]
						}
					]
				}
			}
		}
	}`)

	json.Unmarshal(overlayRaw, &overlay)

	patches, err = processOverlayPatches(resource, overlay)
	assert.Assert(t, reflect.DeepEqual(err, overlayError{}))
	assert.Assert(t, len(patches) != 0)

	doc, err = ApplyPatches(resourceRaw, patches)
	assert.NilError(t, err)

	compareJSONAsMap(t, expectedResult, doc)

	overlayRaw = []byte(`{
		"spec": {
			"template": {
				"metadata": {
					"labels": {
						"(app)": "nginx1"
					}
				},
				"spec": {
					"containers": [
						{
							"(image)": "*:latest",
							"imagePullPolicy": "IfNotPresent",
							"ports": [
								{
									"containerPort": 8080
								}
							]
						}
					]
				}
			}
		}
	}`)

	json.Unmarshal(overlayRaw, &overlay)

	patches, err = processOverlayPatches(resource, overlay)
	assert.Error(t, err, "[overlayError:0] Policy not applied, conditions are not met at /spec/template/metadata/labels/app/, [overlayError:0] Failed validating value nginx with overlay nginx1")
	assert.Assert(t, len(patches) == 0)
}

func TestProcessOverlayPatches_AddingAnchor(t *testing.T) {
	overlayRaw := []byte(`{
		"metadata": {
			"name": "nginx-deployment",
			"labels": {
				"+(app)": "should-not-be-here",
				"+(key1)": "value1"
			}
		}
	}`)
	resourceRaw := []byte(`{
		"metadata": {
			"name": "nginx-deployment",
			"labels": {
				"app": "nginx"
			}
		}
	}`)

	var resource, overlay interface{}

	json.Unmarshal(resourceRaw, &resource)
	json.Unmarshal(overlayRaw, &overlay)

	patches, overlayerr := processOverlayPatches(resource, overlay)
	assert.Assert(t, reflect.DeepEqual(overlayerr, overlayError{}))
	assert.Assert(t, len(patches) != 0)

	doc, err := ApplyPatches(resourceRaw, patches)
	assert.NilError(t, err)
	expectedResult := []byte(`{  
		"metadata":{  
		   "labels":{  
			  "app":"nginx",
			  "key1":"value1"
		   },
		   "name":"nginx-deployment"
		}
	 }`)

	compareJSONAsMap(t, expectedResult, doc)
}

func TestProcessOverlayPatches_AddingAnchorInsideListElement(t *testing.T) {
	overlayRaw := []byte(`
	{
		"spec": {
			"template": {
				"spec": {
					"containers": [
						{
							"(image)": "*:latest",
							"+(imagePullPolicy)": "IfNotPresent"
						}
					]
				}
			}
		}
	}`)
	resourceRaw := []byte(`
	{  
		"apiVersion":"apps/v1",
		"kind":"Deployment",
		"metadata":{  
			"name":"nginx-deployment",
			"labels":{  
				"app":"nginx"
			}
		},
		"spec":{  
			"replicas":1,
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
							"image":"nginx:latest"
						},
						{  
							"image":"ghost:latest",
							"imagePullPolicy":"Always"
						},
						{  
							"image":"debian:latest"
						},
						{  
							"image":"ubuntu:latest",
							"imagePullPolicy":"Always"
						}
					]
				}
			}
		}
	}`)

	var resource, overlay interface{}

	json.Unmarshal(resourceRaw, &resource)
	json.Unmarshal(overlayRaw, &overlay)

	patches, overlayerr := processOverlayPatches(resource, overlay)
	assert.Assert(t, reflect.DeepEqual(overlayerr, overlayError{}))
	assert.Assert(t, len(patches) != 0)

	doc, err := ApplyPatches(resourceRaw, patches)
	assert.NilError(t, err)
	expectedResult := []byte(`
	{  
		"apiVersion":"apps/v1",
		"kind":"Deployment",
		"metadata":{  
			"name":"nginx-deployment",
			"labels":{  
				"app":"nginx"
			}
		},
		"spec":{  
			"replicas":1,
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
							"image":"nginx:latest",
							"imagePullPolicy":"IfNotPresent"
						},
						{  
							"image":"ghost:latest",
							"imagePullPolicy":"Always"
						},
						{  
							"image":"debian:latest",
							"imagePullPolicy":"IfNotPresent"
						},
						{  
							"image":"ubuntu:latest",
							"imagePullPolicy":"Always"
						}
					]
				}
			}
		}
	}`)
	compareJSONAsMap(t, expectedResult, doc)

	// multiple anchors
	overlayRaw = []byte(`
	{
		"spec": {
			"template": {
				"metadata": {
					"labels": {
						"(app)": "nginx"
					}
				},
				"spec": {
					"containers": [
						{
							"(image)": "*:latest",
							"+(imagePullPolicy)": "IfNotPresent"
						}
					]
				}
			}
		}
	}`)

	json.Unmarshal(overlayRaw, &overlay)

	patches, err = processOverlayPatches(resource, overlay)
	assert.Assert(t, reflect.DeepEqual(err, overlayError{}))
	assert.Assert(t, len(patches) != 0)

	doc, err = ApplyPatches(resourceRaw, patches)
	assert.NilError(t, err)

	compareJSONAsMap(t, expectedResult, doc)
}

func TestProcessOverlayPatches_anchorOnPeer(t *testing.T) {
	resourceRaw := []byte(`
	{  
	   "apiVersion":"v1",
	   "kind":"Endpoints",
	   "metadata":{  
		  "name":"test-endpoint",
		  "labels":{  
			 "label":"test"
		  }
	   },
	   "subsets":[  
		  {  
			 "addresses":[  
				{  
				   "ip":"192.168.10.171"
				}
			 ],
			 "ports":[  
				{  
				   "name":"secure-connection",
				   "port":443,
				   "protocol":"TCP"
				}
			 ]
		  }
	   ]
	}`)

	overlayRaw := []byte(`
	{  
	   "subsets":[  
		  {  
		   "addresses":[  
			   {  
				  "(ip)":"192.168.10.171"
			   }
			],
			 "ports":[  
				{  
				   "(name)":"secure-connection",
				   "port":444,
				   "protocol":"UDP"
				}
			 ]
		  }
	   ]
	}`)

	var resource, overlay interface{}

	json.Unmarshal(resourceRaw, &resource)
	json.Unmarshal(overlayRaw, &overlay)

	patches, overlayerr := processOverlayPatches(resource, overlay)
	assert.Assert(t, reflect.DeepEqual(overlayerr, overlayError{}))
	assert.Assert(t, len(patches) != 0)

	doc, err := ApplyPatches(resourceRaw, patches)
	assert.NilError(t, err)
	expectedResult := []byte(`	{  
		"apiVersion":"v1",
		"kind":"Endpoints",
		"metadata":{  
		   "name":"test-endpoint",
		   "labels":{  
			  "label":"test"
		   }
		},
		"subsets":[  
		   {  
			  "addresses":[  
				 {  
					"ip":"192.168.10.171"
				 }
			  ],
			  "ports":[  
				 {  
					"name":"secure-connection",
					"port":444,
					"protocol":"UDP"
				 }
			  ]
		   }
		]
	 }`)

	compareJSONAsMap(t, expectedResult, doc)

	overlayRaw = []byte(`
	{  
	   "subsets":[  
		  {  
		   "addresses":[  
			   {  
				  "ip":"192.168.10.171"
			   }
			],
			 "ports":[  
				{  
				   "(name)":"secure-connection",
				   "(port)":444,
				   "protocol":"UDP"
				}
			 ]
		  }
	   ]
	}`)

	json.Unmarshal(overlayRaw, &overlay)

	patches, err = processOverlayPatches(resource, overlay)
	assert.Error(t, err, "[overlayError:0] Policy not applied, conditions are not met at /subsets/0/ports/0/port/, [overlayError:0] Failed validating value 443 with overlay 444")
	assert.Assert(t, len(patches) == 0)
}

func TestProcessOverlayPatches_insertWithCondition(t *testing.T) {
	resourceRaw := []byte(`{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
		   "name": "psp-demo-unprivileged",
		   "labels": {
			  "app.type": "prod"
		   }
		},
		"spec": {
		   "replicas": 1,
		   "selector": {
			  "matchLabels": {
				 "app": "psp"
			  }
		   },
		   "template": {
			  "metadata": {
				 "labels": {
					"app": "psp"
				 }
			  },
			  "spec": {
				 "securityContext": {
					"runAsNonRoot": true
				 },
				 "containers": [
					{
					   "name": "sec-ctx-unprivileged",
					   "image": "nginxinc/nginx-unprivileged",
					   "securityContext": {
						  "runAsNonRoot": true,
						  "allowPrivilegeEscalation": false
					   },
					   "env": [
						  {
							 "name": "ENV_KEY",
							 "value": "ENV_VALUE"
						  }
					   ]
					}
				 ]
			  }
		   }
		}
	 }`)

	overlayRaw := []byte(`{
		"spec": {
		   "template": {
			  "spec": {
				 "containers": [
					{
					   "(image)": "*/nginx-unprivileged",
					   "securityContext": {
						  "(runAsNonRoot)": true,
						  "allowPrivilegeEscalation": true
					   },
					   "env": [
						  {
							 "name": "ENV_NEW_KEY",
							 "value": "ENV_NEW_VALUE"
						  }
					   ]
					}
				 ]
			  }
		   }
		}
	 }`)

	var resource, overlay interface{}

	json.Unmarshal(resourceRaw, &resource)
	json.Unmarshal(overlayRaw, &overlay)

	patches, overlayerr := processOverlayPatches(resource, overlay)
	assert.Assert(t, reflect.DeepEqual(overlayerr, overlayError{}))
	assert.Assert(t, len(patches) != 0)

	doc, err := ApplyPatches(resourceRaw, patches)
	assert.NilError(t, err)
	expectedResult := []byte(`{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
		   "name": "psp-demo-unprivileged",
		   "labels": {
			  "app.type": "prod"
		   }
		},
		"spec": {
		   "replicas": 1,
		   "selector": {
			  "matchLabels": {
				 "app": "psp"
			  }
		   },
		   "template": {
			  "metadata": {
				 "labels": {
					"app": "psp"
				 }
			  },
			  "spec": {
				 "securityContext": {
					"runAsNonRoot": true
				 },
				 "containers": [
					{
					   "name": "sec-ctx-unprivileged",
					   "image": "nginxinc/nginx-unprivileged",
					   "securityContext": {
						  "runAsNonRoot": true,
						  "allowPrivilegeEscalation": true
					   },
					   "env": [
						  {
							 "name": "ENV_KEY",
							 "value": "ENV_VALUE"
						  },
						  {
							 "name": "ENV_NEW_KEY",
							 "value": "ENV_NEW_VALUE"
						 }
					   ]
					}
				 ]
			  }
		   }
		}
	 }`)

	compareJSONAsMap(t, expectedResult, doc)
}

func TestProcessOverlayPatches_InsertIfNotPresentWithConditions(t *testing.T) {
	overlayRaw := []byte(`
	{
		"metadata": {
		   "annotations": {
			  "+(cluster-autoscaler.kubernetes.io/safe-to-evict)": true
		   }
		},
		"spec": {
		   "volumes": [
			  {
				 "(emptyDir)": {}
			  }
		   ]
		}
	 }`)

	resourceRaw := []byte(`
	{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
		   "name": "pod-with-emptydir"
		},
		"spec": {
		   "containers": [
			  {
				 "image": "k8s.gcr.io/test-webserver",
				 "name": "test-container",
				 "volumeMounts": [
					{
					   "mountPath": "/cache",
					   "name": "cache-volume"
					}
				 ]
			  }
		   ],
		   "volumes": [
			  {
				 "name": "cache-volume",
				 "emptyDir": {}
			  }
		   ]
		}
	 }`)

	var resource, overlay interface{}

	json.Unmarshal(resourceRaw, &resource)
	json.Unmarshal(overlayRaw, &overlay)

	patches, overlayerr := processOverlayPatches(resource, overlay)
	assert.Assert(t, reflect.DeepEqual(overlayerr, overlayError{}))
	assert.Assert(t, len(patches) != 0)

	doc, err := ApplyPatches(resourceRaw, patches)
	assert.NilError(t, err)

	expectedResult := []byte(`
	{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
		   "name": "pod-with-emptydir",
		   "annotations": {
			  "cluster-autoscaler.kubernetes.io/safe-to-evict": "true"
		   }
		},
		"spec": {
		   "containers": [
			  {
				 "image": "k8s.gcr.io/test-webserver",
				 "name": "test-container",
				 "volumeMounts": [
					{
					   "mountPath": "/cache",
					   "name": "cache-volume"
					}
				 ]
			  }
		   ],
		   "volumes": [
			  {
				 "name": "cache-volume",
				 "emptyDir": {}
			  }
		   ]
		}
	 }`)

	t.Log(string(doc))
	compareJSONAsMap(t, expectedResult, doc)
}

func Test_wrapBoolean(t *testing.T) {
	tests := []struct {
		test     string
		expected string
	}{
		{
			test:     `{ "op": "add", "path": "/metadata/annotations", "value":{"cluster-autoscaler.kubernetes.io/safe-to-evict":true} }`,
			expected: `{ "op": "add", "path": "/metadata/annotations", "value":{"cluster-autoscaler.kubernetes.io/safe-to-evict":"true"} }`,
		},
		{
			test:     `{ "op": "add", "path": "/metadata/annotations", "value":{"cluster-autoscaler.kubernetes.io/safe-to-evict": true} }`,
			expected: `{ "op": "add", "path": "/metadata/annotations", "value":{"cluster-autoscaler.kubernetes.io/safe-to-evict":"true"} }`,
		},
		{
			test:     `{ "op": "add", "path": "/metadata/annotations", "value":{"cluster-autoscaler.kubernetes.io/safe-to-evict": false } }`,
			expected: `{ "op": "add", "path": "/metadata/annotations", "value":{"cluster-autoscaler.kubernetes.io/safe-to-evict":"false"} }`,
		},
		{
			test:     `{ "op": "add", "path": "/metadata/annotations/cluster-autoscaler.kubernetes.io~1safe-to-evict", "value": false }`,
			expected: `{ "op": "add", "path": "/metadata/annotations/cluster-autoscaler.kubernetes.io~1safe-to-evict", "value":"false"}`,
		},
	}

	for _, testcase := range tests {
		out := wrapBoolean(testcase.test)
		t.Log(out)
		assert.Assert(t, testcase.expected == out)
	}
}

func TestApplyOverlay_ConditionOnArray(t *testing.T) {
	resourceRaw := []byte(`
	{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
		   "name": "myapp-pod",
		   "labels": {
			  "app": "myapp",
			  "dedicated": "spark"
		   }
		},
		"spec": {
		   "containers": [
			  {
				 "name": "myapp-container",
				 "image": "busybox",
				 "command": [
					"sh",
					"-c",
					"echo Hello Kubernetes! && sleep 3600"
				 ]
			  }
		   ],
		   "affinity": {
			  "nodeAffinity": {
				 "a": {
					"b": [
					   {
						  "matchExpressions": [
							 {
								"key": "dedicated",
								"operator": "NotIn",
								"values": [
								   "spark"
								]
							 }
						  ]
					   }
					]
				 }
			  }
		   }
		}
	 }
	`)

	overlayRaw := []byte(`
	{
		"spec": {
		   "affinity": {
			  "nodeAffinity": {
				 "a": {
					"b": [
					   {
						  "matchExpressions": [
							 {
								"(key)": "dedicated",
								"operator": "In",
								"(values)": [
								   "spark"
								]
							 }
						  ]
					   }
					]
				 }
			  }
		   }
		}
	 }
	`)
	var resource, overlay interface{}

	assert.NilError(t, json.Unmarshal(resourceRaw, &resource))
	assert.NilError(t, json.Unmarshal(overlayRaw, &overlay))

	expectedPatches := []byte(`[
{ "op": "replace", "path": "/spec/affinity/nodeAffinity/a/b/0/matchExpressions/0/operator", "value":"In" }
]`)
	p, err := applyOverlay(resource, overlay, "/")
	assert.NilError(t, err)
	assert.Assert(t, string(JoinPatches(p)) == string(expectedPatches))
}
