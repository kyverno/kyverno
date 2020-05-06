package engine

import (
	"encoding/json"
	"testing"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/utils"
	"gotest.tools/assert"
)

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
	err := json.Unmarshal(rawMap, &unmarshalled)
	assert.NilError(t, err)

	actualMap := utils.GetAnchorsFromMap(unmarshalled)
	assert.Equal(t, len(actualMap), 2)
	assert.Equal(t, actualMap["(name)"].(string), "nirmata-*")
	assert.Equal(t, actualMap["(namespace)"].(string), "kube-?olicy")
}

func TestValidate_image_tag_fail(t *testing.T) {
	// If image tag is latest then imagepull policy needs to be checked
	rawPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
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
								"(image)": "*latest",
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
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	msgs := []string{
		"Validation rule 'validate-tag' succeeded.",
		"Validation error: imagePullPolicy 'Always' required with tag 'latest'; Validation rule 'validate-latest' failed at path '/spec/containers/0/imagePullPolicy/'",
	}
	er := Validate(PolicyContext{Policy: policy, NewResource: *resourceUnstructured})
	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, !er.IsSuccesful())
}

func TestValidate_image_tag_pass(t *testing.T) {
	// If image tag is latest then imagepull policy needs to be checked
	rawPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
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
								"(image)": "*latest",
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
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	msgs := []string{
		"Validation rule 'validate-tag' succeeded.",
		"Validation rule 'validate-latest' succeeded.",
	}
	er := Validate(PolicyContext{Policy: policy, NewResource: *resourceUnstructured})
	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccesful())
}

func TestValidate_Fail_anyPattern(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
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
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(PolicyContext{Policy: policy, NewResource: *resourceUnstructured})
	msgs := []string{"A namespace is required"}
	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, !er.IsSuccesful())
}

func TestValidate_host_network_port(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
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
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(PolicyContext{Policy: policy, NewResource: *resourceUnstructured})
	msgs := []string{"Validation error: Host network and port are not allowed; Validation rule 'validate-host-network-port' failed at path '/spec/containers/0/ports/0/hostPort/'"}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, !er.IsSuccesful())
}

func TestValidate_anchor_arraymap_pass(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
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
								"=(hostPath)": {
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
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(PolicyContext{Policy: policy, NewResource: *resourceUnstructured})
	msgs := []string{"Validation rule 'validate-host-path' succeeded."}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccesful())
}

func TestValidate_anchor_arraymap_fail(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
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
								"=(hostPath)": {
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
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)
	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(PolicyContext{Policy: policy, NewResource: *resourceUnstructured})
	msgs := []string{"Validation error: Host path '/var/lib/' is not allowed; Validation rule 'validate-host-path' failed at path '/spec/volumes/0/hostPath/path/'"}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, !er.IsSuccesful())
}

func TestValidate_anchor_map_notfound(t *testing.T) {
	// anchor not present in resource
	rawPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
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
						  "=(securityContext)": {
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
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(PolicyContext{Policy: policy, NewResource: *resourceUnstructured})
	msgs := []string{"Validation rule 'pod rule 2' succeeded."}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccesful())
}

func TestValidate_anchor_map_found_valid(t *testing.T) {
	// anchor not present in resource
	rawPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
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
						  "=(securityContext)": {
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
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(PolicyContext{Policy: policy, NewResource: *resourceUnstructured})
	msgs := []string{"Validation rule 'pod rule 2' succeeded."}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccesful())
}

func TestValidate_anchor_map_found_invalid(t *testing.T) {
	// anchor not present in resource
	rawPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
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
						  "=(securityContext)": {
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
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(PolicyContext{Policy: policy, NewResource: *resourceUnstructured})
	msgs := []string{"Validation error: pod: validate run as non root user; Validation rule 'pod rule 2' failed at path '/spec/securityContext/runAsNonRoot/'"}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, !er.IsSuccesful())
}

func TestValidate_AnchorList_pass(t *testing.T) {
	// anchor not present in resource
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "policy-secaas-k8s"
		},
		"spec": {
		  "rules": [
			{
			  "name": "pod image rule",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "validate": {
				"pattern": {
				  "spec": {
					"=(containers)": [
					  {
						"name": "nginx"
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
			"app": "v1"
		  }
		},
		"spec": {
		  "containers": [
			{
			  "name": "nginx"
			},
			{
			  "name": "nginx"
			}
		  ]
		}
	  }	
`)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(PolicyContext{Policy: policy, NewResource: *resourceUnstructured})
	msgs := []string{"Validation rule 'pod image rule' succeeded."}

	for index, r := range er.PolicyResponse.Rules {
		t.Log(r.Message)
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccesful())
}

func TestValidate_AnchorList_fail(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "policy-secaas-k8s"
		},
		"spec": {
		  "rules": [
			{
			  "name": "pod image rule",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "validate": {
				"pattern": {
				  "spec": {
					"=(containers)": [
					  {
						"name": "nginx"
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
			"app": "v1"
		  }
		},
		"spec": {
		  "containers": [
			{
			  "name": "nginx"
			},
			{
			  "name": "busy"
			}
		  ]
		}
	  }	
`)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(PolicyContext{Policy: policy, NewResource: *resourceUnstructured})
	// msgs := []string{"Validation rule 'pod image rule' failed at '/spec/containers/1/name/' for resource Pod//myapp-pod."}
	// for index, r := range er.PolicyResponse.Rules {
	// 	// t.Log(r.Message)
	// 	assert.Equal(t, r.Message, msgs[index])
	// }
	assert.Assert(t, !er.IsSuccesful())
}

func TestValidate_existenceAnchor_fail(t *testing.T) {
	// anchor not present in resource
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "policy-secaas-k8s"
		},
		"spec": {
		  "rules": [
			{
			  "name": "pod image rule",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "validate": {
				"pattern": {
				  "spec": {
					"^(containers)": [
					  {
						"name": "nginx"
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
			"app": "v1"
		  }
		},
		"spec": {
		  "containers": [
			{
			  "name": "busy1"
			},
			{
			  "name": "busy"
			}
		  ]
		}
	  }	
`)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(PolicyContext{Policy: policy, NewResource: *resourceUnstructured})
	// msgs := []string{"Validation rule 'pod image rule' failed at '/spec/containers/' for resource Pod//myapp-pod."}

	// for index, r := range er.PolicyResponse.Rules {
	// 	t.Log(r.Message)
	// 	assert.Equal(t, r.Message, msgs[index])
	// }
	assert.Assert(t, !er.IsSuccesful())
}

func TestValidate_existenceAnchor_pass(t *testing.T) {
	// anchor not present in resource
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "policy-secaas-k8s"
		},
		"spec": {
		  "rules": [
			{
			  "name": "pod image rule",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "validate": {
				"pattern": {
				  "spec": {
					"^(containers)": [
					  {
						"name": "nginx"
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
			"app": "v1"
		  }
		},
		"spec": {
		  "containers": [
			{
			  "name": "nginx"
			},
			{
			  "name": "busy"
			}
		  ]
		}
	  }	
`)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(PolicyContext{Policy: policy, NewResource: *resourceUnstructured})
	msgs := []string{"Validation rule 'pod image rule' succeeded."}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccesful())
}

func TestValidate_negationAnchor_deny(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
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
				"message": "Host path is not allowed",
				"pattern": {
				  "spec": {
					"volumes": [
					  {
						"name": "*",
						"X(hostPath)": null
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
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(PolicyContext{Policy: policy, NewResource: *resourceUnstructured})
	msgs := []string{"Validation error: Host path is not allowed; Validation rule 'validate-host-path' failed at path '/spec/volumes/0/hostPath/'"}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, !er.IsSuccesful())
}

func TestValidate_negationAnchor_pass(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
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
				"message": "Host path is not allowed",
				"pattern": {
				  "spec": {
					"volumes": [
					  {
						"name": "*",
						"X(hostPath)": null
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
				 "emptyDir": {}
			  }
		   ]
		}
	 }
	 	 `)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(PolicyContext{Policy: policy, NewResource: *resourceUnstructured})
	msgs := []string{"Validation rule 'validate-host-path' succeeded."}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccesful())
}

func Test_VariableSubstitutionPathNotExistInPattern(t *testing.T) {
	resourceRaw := []byte(`{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "check-root-user"
		},
		"spec": {
			"containers": [
				{
					"name": "check-root-user-a",
					"image": "nginxinc/nginx-unprivileged",
					"securityContext": {
						"runAsNonRoot": true
					}
				}
			]
		}
	}`)

	policyraw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "substitute-variable"
		},
		"spec": {
		  "rules": [
			{
			  "name": "test-path-not-exist",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "validate": {
				"pattern": {
				  "spec": {
					"containers": [
					  {
						"name": "{{request.object.metadata.name1}}*"
					  }
					]
				  }
				}
			  }
			}
		  ]
		}
	  }`)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(policyraw, &policy)
	assert.NilError(t, err)
	resourceUnstructured, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = ctx.AddResource(resourceRaw)
	assert.NilError(t, err)

	policyContext := PolicyContext{
		Policy:      policy,
		Context:     ctx,
		NewResource: *resourceUnstructured}
	er := Validate(policyContext)
	assert.Assert(t, !er.PolicyResponse.Rules[0].Success)
	assert.Equal(t, er.PolicyResponse.Rules[0].Message, "Validation error: ; Validation rule 'test-path-not-exist' failed. 'could not find variable request.object.metadata.name1 at path /spec/containers/0/name'")
}

func Test_VariableSubstitutionPathNotExistInAnyPattern_OnePatternStatisfies(t *testing.T) {
	resourceRaw := []byte(`{
		"apiVersion": "v1",
		"kind": "Deployment",
		"metadata": {
		  "name": "test"
		},
		"spec": {
		  "template": {
			"spec": {
			  "containers": [
				{
				  "name": "test-pod",
				  "image": "nginxinc/nginx-unprivileged"
				}
			  ]
			}
		  }
		}
	  }`)

	policyraw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "substitute-variable"
		},
		"spec": {
		  "rules": [
			{
			  "name": "test-path-not-exist",
			  "match": {
				"resources": {
				  "kinds": [
					"Deployment"
				  ]
				}
			  },
			  "validate": {
				"anyPattern": [
				  {
					"spec": {
					  "template": {
						"spec": {
						  "containers": [
							{
							  "name": "{{request.object.metadata.name1}}*"
							}
						  ]
						}
					  }
					}
				  },
				  {
					"spec": {
					  "template": {
						"spec": {
						  "containers": [
							{
							  "name": "{{request.object.metadata.name}}*"
							}
						  ]
						}
					  }
					}
				  }
				]
			  }
			}
		  ]
		}
	  }`)

	var policy kyverno.ClusterPolicy
	assert.NilError(t, json.Unmarshal(policyraw, &policy))
	resourceUnstructured, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = ctx.AddResource(resourceRaw)
	assert.NilError(t, err)

	policyContext := PolicyContext{
		Policy:      policy,
		Context:     ctx,
		NewResource: *resourceUnstructured}
	er := Validate(policyContext)
	assert.Assert(t, er.PolicyResponse.Rules[0].Success)
	assert.Equal(t, er.PolicyResponse.Rules[0].Message, "Validation rule 'test-path-not-exist' anyPattern[1] succeeded.")
}

func Test_VariableSubstitutionPathNotExistInAnyPattern_AllPathNotPresent(t *testing.T) {
	resourceRaw := []byte(`{
		"apiVersion": "v1",
		"kind": "Deployment",
		"metadata": {
		  "name": "test"
		},
		"spec": {
		  "template": {
			"spec": {
			  "containers": [
				{
				  "name": "test-pod",
				  "image": "nginxinc/nginx-unprivileged"
				}
			  ]
			}
		  }
		}
	  }`)

	policyraw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "substitute-variable"
		},
		"spec": {
		  "rules": [
			{
			  "name": "test-path-not-exist",
			  "match": {
				"resources": {
				  "kinds": [
					"Deployment"
				  ]
				}
			  },
			  "validate": {
				"anyPattern": [
				  {
					"spec": {
					  "template": {
						"spec": {
						  "containers": [
							{
							  "name": "{{request.object.metadata.name1}}*"
							}
						  ]
						}
					  }
					}
				  },
				  {
					"spec": {
					  "template": {
						"spec": {
						  "containers": [
							{
							  "name": "{{request.object.metadata.name2}}*"
							}
						  ]
						}
					  }
					}
				  }
				]
			  }
			}
		  ]
		}
	  }`)

	var policy kyverno.ClusterPolicy
	assert.NilError(t, json.Unmarshal(policyraw, &policy))
	resourceUnstructured, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = ctx.AddResource(resourceRaw)
	assert.NilError(t, err)

	policyContext := PolicyContext{
		Policy:      policy,
		Context:     ctx,
		NewResource: *resourceUnstructured}
	er := Validate(policyContext)
	assert.Assert(t, !er.PolicyResponse.Rules[0].Success)
	assert.Equal(t, er.PolicyResponse.Rules[0].Message, "Substitutions failed: [could not find variable request.object.metadata.name1 at path /spec/template/spec/containers/0/name could not find variable request.object.metadata.name2 at path /spec/template/spec/containers/0/name]")
}

func Test_VariableSubstitutionPathNotExistInAnyPattern_AllPathPresent_NonePatternSatisfy(t *testing.T) {
	resourceRaw := []byte(`{
		"apiVersion": "v1",
		"kind": "Deployment",
		"metadata": {
		  "name": "test"
		},
		"spec": {
		  "template": {
			"spec": {
			  "containers": [
				{
				  "name": "pod-test-pod",
				  "image": "nginxinc/nginx-unprivileged"
				}
			  ]
			}
		  }
		}
	  }`)

	policyraw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "substitute-variable"
		},
		"spec": {
		  "rules": [
			{
			  "name": "test-path-not-exist",
			  "match": {
				"resources": {
				  "kinds": [
					"Deployment"
				  ]
				}
			  },
			  "validate": {
				"anyPattern": [
				  {
					"spec": {
					  "template": {
						"spec": {
						  "containers": [
							{
							  "name": "{{request.object.metadata.name}}*"
							}
						  ]
						}
					  }
					}
				  },
				  {
					"spec": {
					  "template": {
						"spec": {
						  "containers": [
							{
							  "name": "{{request.object.metadata.name}}*"
							}
						  ]
						}
					  }
					}
				  }
				]
			  }
			}
		  ]
		}
	  }`)

	var policy kyverno.ClusterPolicy
	assert.NilError(t, json.Unmarshal(policyraw, &policy))
	resourceUnstructured, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = ctx.AddResource(resourceRaw)
	assert.NilError(t, err)

	policyContext := PolicyContext{
		Policy:      policy,
		Context:     ctx,
		NewResource: *resourceUnstructured}
	er := Validate(policyContext)

	// expectedMsg := "Validation error: ; Validation rule test-path-not-exist anyPattern[0] failed at path /spec/template/spec/containers/0/name/. Validation rule test-path-not-exist anyPattern[1] failed at path /spec/template/spec/containers/0/name/."
	assert.Assert(t, !er.PolicyResponse.Rules[0].Success)
	assert.Equal(t, er.PolicyResponse.Rules[0].Message, "Validation rule 'test-path-not-exist' has failed")
}
