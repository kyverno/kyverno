package engine

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/store"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	utils2 "github.com/kyverno/kyverno/pkg/utils"
	"gotest.tools/assert"
	admissionv1 "k8s.io/api/admission/v1"
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
		"validation rule 'validate-tag' passed.",
		"validation error: imagePullPolicy 'Always' required with tag 'latest'. rule validate-latest failed at path /spec/containers/0/imagePullPolicy/",
	}

	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}

	assert.Assert(t, !er.IsSuccessful())
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
		"validation rule 'validate-tag' passed.",
		"validation rule 'validate-latest' passed.",
	}
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	assert.Assert(t, !er.IsSuccessful())

	msgs := []string{"validation error: A namespace is required. rule check-default-namespace[0] failed at path /metadata/namespace/ rule check-default-namespace[1] failed at path /metadata/namespace/"}
	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	msgs := []string{"validation error: Host network and port are not allowed. rule validate-host-network-port failed at path /spec/containers/0/ports/0/hostPort/"}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, !er.IsSuccessful())
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	msgs := []string{"validation rule 'validate-host-path' passed."}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	msgs := []string{"validation error: Host path '/var/lib/' is not allowed. rule validate-host-path failed at path /spec/volumes/0/hostPath/path/"}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, !er.IsSuccessful())
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	msgs := []string{"validation rule 'pod rule 2' passed."}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	msgs := []string{"validation rule 'pod rule 2' passed."}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}

	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_inequality_List_Processing(t *testing.T) {
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
						  "=(supplementalGroups)": ">0"
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
		   "supplementalGroups": [
			  "2",
			  "5",
			  "10"
		   ]
		}
	 }
`)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	msgs := []string{"validation rule 'pod rule 2' passed."}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}

	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_inequality_List_ProcessingBrackets(t *testing.T) {
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
						"=(supplementalGroups)": [
							">0 & <100001"
						  ]
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
		   "supplementalGroups": [
			  "2",
			  "5",
			  "10",
			  "100",
			  "10000",
			  "1000",
			  "543"
		   ]
		}
	 }
`)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	msgs := []string{"validation rule 'pod rule 2' passed."}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}

	assert.Assert(t, er.IsSuccessful())
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	msgs := []string{"validation error: pod: validate run as non root user. rule pod rule 2 failed at path /spec/securityContext/runAsNonRoot/"}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, !er.IsSuccessful())
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	msgs := []string{"validation rule 'pod image rule' passed."}

	for index, r := range er.PolicyResponse.Rules {
		t.Log(r.Message)
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	assert.Assert(t, !er.IsSuccessful())
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	assert.Assert(t, !er.IsSuccessful())
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	msgs := []string{"validation rule 'pod image rule' passed."}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	msgs := []string{"validation error: Host path is not allowed. rule validate-host-path failed at path /spec/volumes/0/hostPath/"}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, !er.IsSuccessful())
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	msgs := []string{"validation rule 'validate-host-path' passed."}

	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
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
	err = context.AddResource(ctx, resourceRaw)
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      &policy,
		JSONContext: ctx,
		NewResource: *resourceUnstructured}
	er := Validate(policyContext)

	assert.Equal(t, len(er.PolicyResponse.Rules), 1)
	assert.Equal(t, er.PolicyResponse.Rules[0].Status, response.RuleStatusError)
	assert.Assert(t, strings.Contains(er.PolicyResponse.Rules[0].Message, "Unknown key \"name1\" in path"))
}

func Test_VariableSubstitutionPathNotExistInAnyPattern_OnePatternStatisfiesButSubstitutionFails(t *testing.T) {
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
	err = context.AddResource(ctx, resourceRaw)
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      &policy,
		JSONContext: ctx,
		NewResource: *resourceUnstructured}
	er := Validate(policyContext)

	assert.Equal(t, len(er.PolicyResponse.Rules), 1)
	assert.Equal(t, er.PolicyResponse.Rules[0].Status, response.RuleStatusError)
	assert.Assert(t, strings.Contains(er.PolicyResponse.Rules[0].Message, "Unknown key \"name1\" in path"))
}

func Test_VariableSubstitution_NotOperatorWithStringVariable(t *testing.T) {
	resourceRaw := []byte(`{
		"apiVersion": "v1",
		"kind": "Deployment",
		"metadata": {
		  "name": "test"
		},
		"spec": {
		  "content": "sample text"
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
			  "name": "not-operator-with-variable-should-alway-fail-validation",
			  "match": {
				"resources": {
				  "kinds": [
					"Deployment"
				  ]
				}
			  },
			  "validate": {
				"pattern": {
			      "spec": {
				    "content": "!{{ request.object.spec.content }}"
				  }
				}
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
	err = context.AddResource(ctx, resourceRaw)
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      &policy,
		JSONContext: ctx,
		NewResource: *resourceUnstructured}
	er := Validate(policyContext)
	assert.Equal(t, er.PolicyResponse.Rules[0].Status, response.RuleStatusFail)
	assert.Equal(t, er.PolicyResponse.Rules[0].Message, "validation error: rule not-operator-with-variable-should-alway-fail-validation failed at path /spec/content/")
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
	err = context.AddResource(ctx, resourceRaw)
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      &policy,
		JSONContext: ctx,
		NewResource: *resourceUnstructured}
	er := Validate(policyContext)

	assert.Equal(t, len(er.PolicyResponse.Rules), 1)
	assert.Equal(t, er.PolicyResponse.Rules[0].Status, response.RuleStatusError)
	assert.Assert(t, strings.Contains(er.PolicyResponse.Rules[0].Message, "Unknown key \"name1\" in path"))
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
	err = context.AddResource(ctx, resourceRaw)
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      &policy,
		JSONContext: ctx,
		NewResource: *resourceUnstructured}
	er := Validate(policyContext)

	assert.Equal(t, er.PolicyResponse.Rules[0].Status, response.RuleStatusFail)
	assert.Equal(t, er.PolicyResponse.Rules[0].Message,
		"validation error: rule test-path-not-exist[0] failed at path /spec/template/spec/containers/0/name/ rule test-path-not-exist[1] failed at path /spec/template/spec/containers/0/name/")
}

func Test_VariableSubstitutionValidate_VariablesInMessageAreResolved(t *testing.T) {
	resourceRaw := []byte(`{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
		  "name": "busybox",
		  "labels": {
			"app": "busybox",
			"color": "red",
			"animal": "cow",
			"food": "pizza",
			"car": "jeep",
			"env": "qa"
		  }
		},
		"spec": {
		  "replicas": 1,
		  "selector": {
			"matchLabels": {
			  "app": "busybox"
			}
		  },
		  "template": {
			"metadata": {
			  "labels": {
				"app": "busybox"
			  }
			},
			"spec": {
			  "containers": [
				{
				  "image": "busybox:1.28",
				  "name": "busybox",
				  "command": [
					"sleep",
					"9999"
				  ]
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
		  "name": "cm-array-example"
		},
		"spec": {
		  "validationFailureAction": "enforce",
		  "background": false,
		  "rules": [
			{
			  "name": "validate-role-annotation",
			  "match": {
				"resources": {
				  "kinds": [
					"Deployment"
				  ]
				}
			  },
			  "validate": {
				"message": "The animal {{ request.object.metadata.labels.animal }} is not in the allowed list of animals.",
				"deny": {
				  "conditions": [
					{
					  "key": "{{ request.object.metadata.labels.animal }}",
					  "operator": "NotIn",
					  "value": [
						"snake",
						"bear",
						"cat",
						"dog"
					]
					}
				  ]
				}
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
	err = context.AddResource(ctx, resourceRaw)
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      &policy,
		JSONContext: ctx,
		NewResource: *resourceUnstructured}
	er := Validate(policyContext)
	assert.Equal(t, er.PolicyResponse.Rules[0].Status, response.RuleStatusFail)
	assert.Equal(t, er.PolicyResponse.Rules[0].Message, "The animal cow is not in the allowed list of animals.")
}

func Test_Flux_Kustomization_PathNotPresent(t *testing.T) {
	tests := []struct {
		name             string
		policyRaw        []byte
		resourceRaw      []byte
		expectedResults  []response.RuleStatus
		expectedMessages []string
	}{
		{
			name:      "path-not-present",
			policyRaw: []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"flux-multi-tenancy"},"spec":{"validationFailureAction":"enforce","rules":[{"name":"serviceAccountName","exclude":{"resources":{"namespaces":["flux-system"]}},"match":{"resources":{"kinds":["Kustomization","HelmRelease"]}},"validate":{"message":".spec.serviceAccountName is required","pattern":{"spec":{"serviceAccountName":"?*"}}}},{"name":"sourceRefNamespace","exclude":{"resources":{"namespaces":["flux-system"]}},"match":{"resources":{"kinds":["Kustomization","HelmRelease"]}},"validate":{"message":"spec.sourceRef.namespace must be the same as metadata.namespace","deny":{"conditions":[{"key":"{{request.object.spec.sourceRef.namespace}}","operator":"NotEquals","value":"{{request.object.metadata.namespace}}"}]}}}]}}`),
			// referred variable path not present
			resourceRaw:      []byte(`{"apiVersion":"kustomize.toolkit.fluxcd.io/v1beta1","kind":"Kustomization","metadata":{"name":"dev-team","namespace":"apps"},"spec":{"serviceAccountName":"dev-team","interval":"5m","sourceRef":{"kind":"GitRepository","name":"dev-team"},"prune":true,"validation":"client"}}`),
			expectedResults:  []response.RuleStatus{response.RuleStatusPass, response.RuleStatusError},
			expectedMessages: []string{"validation rule 'serviceAccountName' passed.", "failed to substitute variables in deny conditions: failed to resolve request.object.spec.sourceRef.namespace at path /0/key: JMESPath query failed: Unknown key \"namespace\" in path"},
		},
		{
			name:      "resource-with-violation",
			policyRaw: []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"flux-multi-tenancy"},"spec":{"validationFailureAction":"enforce","rules":[{"name":"serviceAccountName","exclude":{"resources":{"namespaces":["flux-system"]}},"match":{"resources":{"kinds":["Kustomization","HelmRelease"]}},"validate":{"message":".spec.serviceAccountName is required","pattern":{"spec":{"serviceAccountName":"?*"}}}},{"name":"sourceRefNamespace","exclude":{"resources":{"namespaces":["flux-system"]}},"match":{"resources":{"kinds":["Kustomization","HelmRelease"]}},"validate":{"message":"spec.sourceRef.namespace {{request.object.spec.sourceRef.namespace}} must be the same as metadata.namespace {{request.object.metadata.namespace}}","deny":{"conditions":[{"key":"{{request.object.spec.sourceRef.namespace}}","operator":"NotEquals","value":"{{request.object.metadata.namespace}}"}]}}}]}}`),
			// referred variable path present with different value
			resourceRaw:      []byte(`{"apiVersion":"kustomize.toolkit.fluxcd.io/v1beta1","kind":"Kustomization","metadata":{"name":"dev-team","namespace":"apps"},"spec":{"serviceAccountName":"dev-team","interval":"5m","sourceRef":{"kind":"GitRepository","name":"dev-team","namespace":"default"},"prune":true,"validation":"client"}}`),
			expectedResults:  []response.RuleStatus{response.RuleStatusPass, response.RuleStatusFail},
			expectedMessages: []string{"validation rule 'serviceAccountName' passed.", "spec.sourceRef.namespace default must be the same as metadata.namespace apps"},
		},
		{
			name:      "resource-comply",
			policyRaw: []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"flux-multi-tenancy"},"spec":{"validationFailureAction":"enforce","rules":[{"name":"serviceAccountName","exclude":{"resources":{"namespaces":["flux-system"]}},"match":{"resources":{"kinds":["Kustomization","HelmRelease"]}},"validate":{"message":".spec.serviceAccountName is required","pattern":{"spec":{"serviceAccountName":"?*"}}}},{"name":"sourceRefNamespace","exclude":{"resources":{"namespaces":["flux-system"]}},"match":{"resources":{"kinds":["Kustomization","HelmRelease"]}},"validate":{"message":"spec.sourceRef.namespace must be the same as metadata.namespace","deny":{"conditions":[{"key":"{{request.object.spec.sourceRef.namespace}}","operator":"NotEquals","value":"{{request.object.metadata.namespace}}"}]}}}]}}`),
			// referred variable path present with same value - validate passes
			resourceRaw:      []byte(`{"apiVersion":"kustomize.toolkit.fluxcd.io/v1beta1","kind":"Kustomization","metadata":{"name":"dev-team","namespace":"apps"},"spec":{"serviceAccountName":"dev-team","interval":"5m","sourceRef":{"kind":"GitRepository","name":"dev-team","namespace":"apps"},"prune":true,"validation":"client"}}`),
			expectedResults:  []response.RuleStatus{response.RuleStatusPass, response.RuleStatusPass},
			expectedMessages: []string{"validation rule 'serviceAccountName' passed.", "validation rule 'sourceRefNamespace' passed."},
		},
	}

	for _, test := range tests {
		var policy kyverno.ClusterPolicy
		assert.NilError(t, json.Unmarshal(test.policyRaw, &policy))
		resourceUnstructured, err := utils.ConvertToUnstructured(test.resourceRaw)
		assert.NilError(t, err)

		ctx := context.NewContext()
		err = context.AddResource(ctx, test.resourceRaw)
		assert.NilError(t, err)

		policyContext := &PolicyContext{
			Policy:      &policy,
			JSONContext: ctx,
			NewResource: *resourceUnstructured}
		er := Validate(policyContext)

		for i, rule := range er.PolicyResponse.Rules {
			assert.Equal(t, er.PolicyResponse.Rules[i].Status, test.expectedResults[i], "\ntest %s failed\nexpected: %s\nactual: %s", test.name, test.expectedResults[i].String(), er.PolicyResponse.Rules[i].Status.String())
			assert.Equal(t, er.PolicyResponse.Rules[i].Message, test.expectedMessages[i], "\ntest %s failed\nexpected: %s\nactual: %s", test.name, test.expectedMessages[i], rule.Message)
		}
	}
}

type testCase struct {
	description   string
	policy        []byte
	request       []byte
	userInfo      []byte
	requestDenied bool
}

func Test_denyFeatureIssue744_BlockUpdate(t *testing.T) {
	testcases := []testCase{
		{
			description:   "Blocks update requests for resources with label allow-updates(success case)",
			policy:        []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"block-updates-success"},"spec":{"validationFailureAction":"enforce","background":false,"rules":[{"name":"check-allow-updates","match":{"resources":{"selector":{"matchLabels":{"allow-updates":"false"}}}},"exclude":{"clusterRoles":["random"]},"validate":{"message":"Updating {{request.object.kind}} / {{request.object.metadata.name}} is not allowed","deny":{"conditions":{"all":[{"key":"{{request.operation}}","operator":"Equals","value":"UPDATE"}]}}}}]}}`),
			request:       []byte(`{"uid":"7b0600b7-0258-4ecb-9666-c2839bd19612","kind":{"group":"","version":"v1","kind":"Pod"},"resource":{"group":"","version":"v1","resource":"pods"},"subResource":"status","requestKind":{"group":"","version":"v1","kind":"Pod"},"requestResource":{"group":"","version":"v1","resource":"pods"},"requestSubResource":"status","name":"hello-world","namespace":"default","operation":"UPDATE","userInfo":{"username":"system:node:kind-control-plane","groups":["system:authenticated"]},"object":{"kind":"Pod","apiVersion":"v1","metadata":{"name":"hello-world","namespace":"default","uid":"2b42971e-6fcf-41a7-ae44-80963f957eae","resourceVersion":"3438","creationTimestamp":"2020-05-06T20:41:37Z","labels":{"allow-updates":"false","something":"hereeeeeseee"},"annotations":{"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"kind\":\"Pod\",\"metadata\":{\"annotations\":{},\"labels\":{\"allow-updates\":\"false\",\"something\":\"hereeeeeseee\"},\"name\":\"hello-world\",\"namespace\":\"default\"},\"spec\":{\"containers\":[{\"image\":\"hello-world:latest\",\"name\":\"hello-world\",\"ports\":[{\"containerPort\":80}],\"resources\":{\"limits\":{\"cpu\":\"0.2\",\"memory\":\"30Mi\"},\"requests\":{\"cpu\":\"0.1\",\"memory\":\"20Mi\"}}}]}}\n"}},"spec":{"volumes":[{"name":"default-token-4q2mj","secret":{"secretName":"default-token-4q2mj","defaultMode":420}}],"containers":[{"name":"hello-world","image":"hello-world:latest","ports":[{"containerPort":80,"protocol":"TCP"}],"resources":{"limits":{"cpu":"200m","memory":"30Mi"},"requests":{"cpu":"100m","memory":"20Mi"}},"volumeMounts":[{"name":"default-token-4q2mj","readOnly":true,"mountPath":"/var/run/secrets/kubernetes.io/serviceaccount"}],"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File","imagePullPolicy":"Always"}],"restartPolicy":"Always","terminationGracePeriodSeconds":30,"dnsPolicy":"ClusterFirst","serviceAccountName":"default","serviceAccount":"default","nodeName":"kind-control-plane","securityContext":{},"schedulerName":"default-scheduler","tolerations":[{"key":"node.kubernetes.io/not-ready","operator":"Exists","effect":"NoExecute","tolerationSeconds":300},{"key":"node.kubernetes.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":300}],"priority":0,"enableServiceLinks":true},"status":{"phase":"Running","conditions":[{"type":"Initialized","status":"True","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:41:37Z"},{"type":"Ready","status":"False","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:41:37Z","reason":"ContainersNotReady","message":"containers with unready status: [hello-world]"},{"type":"ContainersReady","status":"False","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:41:37Z","reason":"ContainersNotReady","message":"containers with unready status: [hello-world]"},{"type":"PodScheduled","status":"True","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:41:37Z"}],"hostIP":"172.17.0.2","podIP":"10.244.0.8","podIPs":[{"ip":"10.244.0.8"}],"startTime":"2020-05-06T20:41:37Z","containerStatuses":[{"name":"hello-world","state":{"terminated":{"exitCode":0,"reason":"Completed","startedAt":"2020-05-06T20:42:01Z","finishedAt":"2020-05-06T20:42:01Z","containerID":"containerd://46dc1c3dead976b5cc6e5f6a8dc86988e8ce401e6fd903d4637848dd4baac0c4"}},"lastState":{},"ready":false,"restartCount":0,"image":"docker.io/library/hello-world:latest","imageID":"docker.io/library/hello-world@sha256:8e3114318a995a1ee497790535e7b88365222a21771ae7e53687ad76563e8e76","containerID":"containerd://46dc1c3dead976b5cc6e5f6a8dc86988e8ce401e6fd903d4637848dd4baac0c4","started":false}],"qosClass":"Burstable"}},"oldObject":{"kind":"Pod","apiVersion":"v1","metadata":{"name":"hello-world","namespace":"default","uid":"2b42971e-6fcf-41a7-ae44-80963f957eae","resourceVersion":"3438","creationTimestamp":"2020-05-06T20:41:37Z","labels":{"allow-updates":"false","something":"hereeeeeseee"},"annotations":{"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"kind\":\"Pod\",\"metadata\":{\"annotations\":{},\"labels\":{\"allow-updates\":\"false\",\"something\":\"hereeeeeseee\"},\"name\":\"hello-world\",\"namespace\":\"default\"},\"spec\":{\"containers\":[{\"image\":\"hello-world:latest\",\"name\":\"hello-world\",\"ports\":[{\"containerPort\":80}],\"resources\":{\"limits\":{\"cpu\":\"0.2\",\"memory\":\"30Mi\"},\"requests\":{\"cpu\":\"0.1\",\"memory\":\"20Mi\"}}}]}}\n"}},"spec":{"volumes":[{"name":"default-token-4q2mj","secret":{"secretName":"default-token-4q2mj","defaultMode":420}}],"containers":[{"name":"hello-world","image":"hello-world:latest","ports":[{"containerPort":80,"protocol":"TCP"}],"resources":{"limits":{"cpu":"200m","memory":"30Mi"},"requests":{"cpu":"100m","memory":"20Mi"}},"volumeMounts":[{"name":"default-token-4q2mj","readOnly":true,"mountPath":"/var/run/secrets/kubernetes.io/serviceaccount"}],"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File","imagePullPolicy":"Always"}],"restartPolicy":"Always","terminationGracePeriodSeconds":30,"dnsPolicy":"ClusterFirst","serviceAccountName":"default","serviceAccount":"default","nodeName":"kind-control-plane","securityContext":{},"schedulerName":"default-scheduler","tolerations":[{"key":"node.kubernetes.io/not-ready","operator":"Exists","effect":"NoExecute","tolerationSeconds":300},{"key":"node.kubernetes.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":300}],"priority":0,"enableServiceLinks":true},"status":{"phase":"Pending","conditions":[{"type":"Initialized","status":"True","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:41:37Z"},{"type":"Ready","status":"False","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:41:37Z","reason":"ContainersNotReady","message":"containers with unready status: [hello-world]"},{"type":"ContainersReady","status":"False","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:41:37Z","reason":"ContainersNotReady","message":"containers with unready status: [hello-world]"},{"type":"PodScheduled","status":"True","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:41:37Z"}],"hostIP":"172.17.0.2","startTime":"2020-05-06T20:41:37Z","containerStatuses":[{"name":"hello-world","state":{"waiting":{"reason":"ContainerCreating"}},"lastState":{},"ready":false,"restartCount":0,"image":"hello-world:latest","imageID":"","started":false}],"qosClass":"Burstable"}},"dryRun":false,"options":{"kind":"UpdateOptions","apiVersion":"meta.k8s.io/v1"}}`),
			userInfo:      []byte(`{"roles":["kube-system:kubeadm:kubelet-config-1.17","kube-system:kubeadm:nodes-kubeadm-config"],"clusterRoles":["system:basic-user","system:certificates.k8s.io:certificatesigningrequests:selfnodeclient","system:public-info-viewer","system:discovery"],"userInfo":{"username":"kubernetes-admin","groups":["system:authenticated"]}}`),
			requestDenied: true,
		},
		{
			description:   "Blocks update requests for resources with label allow-updates(failure case)",
			policy:        []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"block-updates-failure"},"spec":{"validationFailureAction":"enforce","background":false,"rules":[{"name":"check-allow-deletes","match":{"resources":{"selector":{"matchLabels":{"allow-deletes":"false"}}}},"exclude":{"clusterRoles":["random"]},"validate":{"message":"Deleting {{request.oldObject.kind}} / {{request.oldObject.metadata.name}} is not allowed","deny":{"conditions":{"all":[{"key":"{{request.operation}}","operator":"Equal","value":"DELETE"}]}}}}]}}`),
			request:       []byte(`{"uid":"9c284cdb-b0de-42aa-adf5-649a44bc861b","kind":{"group":"","version":"v1","kind":"Pod"},"resource":{"group":"","version":"v1","resource":"pods"},"requestKind":{"group":"","version":"v1","kind":"Pod"},"requestResource":{"group":"","version":"v1","resource":"pods"},"name":"hello-world","namespace":"default","operation":"CREATE","userInfo":{"username":"kubernetes-admin","groups":["system:masters","system:authenticated"]},"object":{"kind":"Pod","apiVersion":"v1","metadata":{"name":"hello-world","namespace":"default","uid":"41a928a7-73f4-419f-bd64-de11f4f0a8ca","creationTimestamp":"2020-05-06T20:43:50Z","labels":{"allow-updates":"false","something":"hereeeeeseee"},"annotations":{"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"kind\":\"Pod\",\"metadata\":{\"annotations\":{},\"labels\":{\"allow-updates\":\"false\",\"something\":\"hereeeeeseee\"},\"name\":\"hello-world\",\"namespace\":\"default\"},\"spec\":{\"containers\":[{\"image\":\"hello-world:latest\",\"name\":\"hello-world\",\"ports\":[{\"containerPort\":80}],\"resources\":{\"limits\":{\"cpu\":\"0.2\",\"memory\":\"30Mi\"},\"requests\":{\"cpu\":\"0.1\",\"memory\":\"20Mi\"}}}]}}\n"}},"spec":{"volumes":[{"name":"default-token-4q2mj","secret":{"secretName":"default-token-4q2mj"}}],"containers":[{"name":"hello-world","image":"hello-world:latest","ports":[{"containerPort":80,"protocol":"TCP"}],"resources":{"limits":{"cpu":"200m","memory":"30Mi"},"requests":{"cpu":"100m","memory":"20Mi"}},"volumeMounts":[{"name":"default-token-4q2mj","readOnly":true,"mountPath":"/var/run/secrets/kubernetes.io/serviceaccount"}],"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File","imagePullPolicy":"Always"}],"restartPolicy":"Always","terminationGracePeriodSeconds":30,"dnsPolicy":"ClusterFirst","serviceAccountName":"default","serviceAccount":"default","securityContext":{},"schedulerName":"default-scheduler","tolerations":[{"key":"node.kubernetes.io/not-ready","operator":"Exists","effect":"NoExecute","tolerationSeconds":300},{"key":"node.kubernetes.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":300}],"priority":0,"enableServiceLinks":true},"status":{"phase":"Pending","qosClass":"Burstable"}},"oldObject":null,"dryRun":false,"options":{"kind":"CreateOptions","apiVersion":"meta.k8s.io/v1"}}`),
			userInfo:      []byte(`{"roles":null,"clusterRoles":["system:public-info-viewer","cluster-admin","system:discovery","system:basic-user"],"userInfo":{"username":"kubernetes-admin","groups":["system:masters","system:authenticated"]}}`),
			requestDenied: false,
		},
	}

	for _, testcase := range testcases {
		executeTest(t, testcase)
	}
}

func Test_denyFeatureIssue744_DenyAll(t *testing.T) {
	testcases := []testCase{
		{
			description:   "Deny all requests on a namespace",
			policy:        []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"block-request"},"spec":{"validationFailureAction":"enforce","rules":[{"name":"block-request","match":{"resources":{"namespaces":["kube-system"]}},"validate":{"deny":{}}}]}}`),
			request:       []byte(`{"uid":"2cf2b192-2c25-4f14-ac3a-315408d398f2","kind":{"group":"","version":"v1","kind":"Pod"},"resource":{"group":"","version":"v1","resource":"pods"},"requestKind":{"group":"","version":"v1","kind":"Pod"},"requestResource":{"group":"","version":"v1","resource":"pods"},"name":"hello-world","namespace":"default","operation":"UPDATE","userInfo":{"username":"kubernetes-admin","groups":["system:masters","system:authenticated"]},"object":{"kind":"Pod","apiVersion":"v1","metadata":{"name":"hello-world","namespace":"default","uid":"f5c33eaf-79d8-4bc0-8819-749b3606012c","resourceVersion":"5470","creationTimestamp":"2020-05-06T20:57:15Z","labels":{"allow-updates":"false","something":"existes","something2":"feeereeeeeeeee"},"annotations":{"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"kind\":\"Pod\",\"metadata\":{\"annotations\":{},\"labels\":{\"allow-updates\":\"false\",\"something\":\"existes\",\"something2\":\"feeereeeeeeeee\"},\"name\":\"hello-world\",\"namespace\":\"default\"},\"spec\":{\"containers\":[{\"image\":\"hello-world:latest\",\"name\":\"hello-world\",\"ports\":[{\"containerPort\":80}],\"resources\":{\"limits\":{\"cpu\":\"0.2\",\"memory\":\"30Mi\"},\"requests\":{\"cpu\":\"0.1\",\"memory\":\"20Mi\"}}}]}}\n"}},"spec":{"volumes":[{"name":"default-token-4q2mj","secret":{"secretName":"default-token-4q2mj","defaultMode":420}}],"containers":[{"name":"hello-world","image":"hello-world:latest","ports":[{"containerPort":80,"protocol":"TCP"}],"resources":{"limits":{"cpu":"200m","memory":"30Mi"},"requests":{"cpu":"100m","memory":"20Mi"}},"volumeMounts":[{"name":"default-token-4q2mj","readOnly":true,"mountPath":"/var/run/secrets/kubernetes.io/serviceaccount"}],"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File","imagePullPolicy":"Always"}],"restartPolicy":"Always","terminationGracePeriodSeconds":30,"dnsPolicy":"ClusterFirst","serviceAccountName":"default","serviceAccount":"default","nodeName":"kind-control-plane","securityContext":{},"schedulerName":"default-scheduler","tolerations":[{"key":"node.kubernetes.io/not-ready","operator":"Exists","effect":"NoExecute","tolerationSeconds":300},{"key":"node.kubernetes.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":300}],"priority":0,"enableServiceLinks":true},"status":{"phase":"Pending","conditions":[{"type":"Initialized","status":"True","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:57:15Z"},{"type":"Ready","status":"False","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:57:15Z","reason":"ContainersNotReady","message":"containers with unready status: [hello-world]"},{"type":"ContainersReady","status":"False","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:57:15Z","reason":"ContainersNotReady","message":"containers with unready status: [hello-world]"},{"type":"PodScheduled","status":"True","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:57:15Z"}],"hostIP":"172.17.0.2","startTime":"2020-05-06T20:57:15Z","containerStatuses":[{"name":"hello-world","state":{"waiting":{"reason":"ContainerCreating"}},"lastState":{},"ready":false,"restartCount":0,"image":"hello-world:latest","imageID":"","started":false}],"qosClass":"Burstable"}},"oldObject":{"kind":"Pod","apiVersion":"v1","metadata":{"name":"hello-world","namespace":"default","uid":"f5c33eaf-79d8-4bc0-8819-749b3606012c","resourceVersion":"5470","creationTimestamp":"2020-05-06T20:57:15Z","labels":{"allow-updates":"false","something":"existes","something2":"feeereeeeeeeee"},"annotations":{"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"kind\":\"Pod\",\"metadata\":{\"annotations\":{},\"labels\":{\"allow-updates\":\"false\",\"something\":\"existes\",\"something2\":\"feeereeeeeeeee\"},\"name\":\"hello-world\",\"namespace\":\"default\"},\"spec\":{\"containers\":[{\"image\":\"hello-world:latest\",\"name\":\"hello-world\",\"ports\":[{\"containerPort\":80}],\"resources\":{\"limits\":{\"cpu\":\"0.2\",\"memory\":\"30Mi\"},\"requests\":{\"cpu\":\"0.1\",\"memory\":\"20Mi\"}}}]}}\n"}},"spec":{"volumes":[{"name":"default-token-4q2mj","secret":{"secretName":"default-token-4q2mj","defaultMode":420}}],"containers":[{"name":"hello-world","image":"hello-world:latest","ports":[{"containerPort":80,"protocol":"TCP"}],"resources":{"limits":{"cpu":"200m","memory":"30Mi"},"requests":{"cpu":"100m","memory":"20Mi"}},"volumeMounts":[{"name":"default-token-4q2mj","readOnly":true,"mountPath":"/var/run/secrets/kubernetes.io/serviceaccount"}],"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File","imagePullPolicy":"Always"}],"restartPolicy":"Always","terminationGracePeriodSeconds":30,"dnsPolicy":"ClusterFirst","serviceAccountName":"default","serviceAccount":"default","nodeName":"kind-control-plane","securityContext":{},"schedulerName":"default-scheduler","tolerations":[{"key":"node.kubernetes.io/not-ready","operator":"Exists","effect":"NoExecute","tolerationSeconds":300},{"key":"node.kubernetes.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":300}],"priority":0,"enableServiceLinks":true},"status":{"phase":"Pending","conditions":[{"type":"Initialized","status":"True","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:57:15Z"},{"type":"Ready","status":"False","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:57:15Z","reason":"ContainersNotReady","message":"containers with unready status: [hello-world]"},{"type":"ContainersReady","status":"False","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:57:15Z","reason":"ContainersNotReady","message":"containers with unready status: [hello-world]"},{"type":"PodScheduled","status":"True","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:57:15Z"}],"hostIP":"172.17.0.2","startTime":"2020-05-06T20:57:15Z","containerStatuses":[{"name":"hello-world","state":{"waiting":{"reason":"ContainerCreating"}},"lastState":{},"ready":false,"restartCount":0,"image":"hello-world:latest","imageID":"","started":false}],"qosClass":"Burstable"}},"dryRun":false,"options":{"kind":"UpdateOptions","apiVersion":"meta.k8s.io/v1"}}`),
			userInfo:      []byte(`{"roles":null,"clusterRoles":null,"userInfo":{"username":"kubernetes-admin","groups":["system:masters","system:authenticated"]}}`),
			requestDenied: false,
		},
	}

	for _, testcase := range testcases {
		executeTest(t, testcase)
	}
}

func Test_denyFeatureIssue744_BlockFields(t *testing.T) {
	testcases := []testCase{
		{
			description:   "Blocks certain fields(success case)",
			policy:        []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"prevent-field-update-success"},"spec":{"validationFailureAction":"enforce","background":false,"rules":[{"name":"prevent-field-update","match":{"resources":{"selector":{"matchLabels":{"allow-updates":"false"}}}},"validate":{"message":"Updating field label 'something' is not allowed","deny":{"conditions":{"all":[{"key":"{{request.object.metadata.labels.something}}","operator":"NotEqual","value":""},{"key":"{{request.object.metadata.labels.something}}","operator":"NotEquals","value":"{{request.oldObject.metadata.labels.something}}"}]}}}}]}}`),
			request:       []byte(`{"uid":"11d46f83-a31b-444e-8209-c43b24f1af8a","kind":{"group":"","version":"v1","kind":"Pod"},"resource":{"group":"","version":"v1","resource":"pods"},"requestKind":{"group":"","version":"v1","kind":"Pod"},"requestResource":{"group":"","version":"v1","resource":"pods"},"name":"hello-world","namespace":"default","operation":"UPDATE","userInfo":{"username":"kubernetes-admin","groups":["system:masters","system:authenticated"]},"object":{"kind":"Pod","apiVersion":"v1","metadata":{"name":"hello-world","namespace":"default","uid":"42bd0f0a-4b1f-4f7c-a40d-4dbed5522732","resourceVersion":"4333","creationTimestamp":"2020-05-06T20:51:58Z","labels":{"allow-updates":"false","something":"existes","something2":"feeereeeeeeeee"},"annotations":{"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"kind\":\"Pod\",\"metadata\":{\"annotations\":{},\"labels\":{\"allow-updates\":\"false\",\"something\":\"existes\",\"something2\":\"feeereeeeeeeee\"},\"name\":\"hello-world\",\"namespace\":\"default\"},\"spec\":{\"containers\":[{\"image\":\"hello-world:latest\",\"name\":\"hello-world\",\"ports\":[{\"containerPort\":80}],\"resources\":{\"limits\":{\"cpu\":\"0.2\",\"memory\":\"30Mi\"},\"requests\":{\"cpu\":\"0.1\",\"memory\":\"20Mi\"}}}]}}\n"}},"spec":{"volumes":[{"name":"default-token-4q2mj","secret":{"secretName":"default-token-4q2mj","defaultMode":420}}],"containers":[{"name":"hello-world","image":"hello-world:latest","ports":[{"containerPort":80,"protocol":"TCP"}],"resources":{"limits":{"cpu":"200m","memory":"30Mi"},"requests":{"cpu":"100m","memory":"20Mi"}},"volumeMounts":[{"name":"default-token-4q2mj","readOnly":true,"mountPath":"/var/run/secrets/kubernetes.io/serviceaccount"}],"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File","imagePullPolicy":"Always"}],"restartPolicy":"Always","terminationGracePeriodSeconds":30,"dnsPolicy":"ClusterFirst","serviceAccountName":"default","serviceAccount":"default","securityContext":{},"schedulerName":"default-scheduler","tolerations":[{"key":"node.kubernetes.io/not-ready","operator":"Exists","effect":"NoExecute","tolerationSeconds":300},{"key":"node.kubernetes.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":300}],"priority":0,"enableServiceLinks":true},"status":{"phase":"Pending","qosClass":"Burstable"}},"oldObject":{"kind":"Pod","apiVersion":"v1","metadata":{"name":"hello-world","namespace":"default","uid":"42bd0f0a-4b1f-4f7c-a40d-4dbed5522732","resourceVersion":"4333","creationTimestamp":"2020-05-06T20:51:58Z","labels":{"allow-updates":"false","something":"exists","something2":"feeereeeeeeeee"},"annotations":{"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"kind\":\"Pod\",\"metadata\":{\"annotations\":{},\"labels\":{\"allow-updates\":\"false\",\"something\":\"exists\",\"something2\":\"feeereeeeeeeee\"},\"name\":\"hello-world\",\"namespace\":\"default\"},\"spec\":{\"containers\":[{\"image\":\"hello-world:latest\",\"name\":\"hello-world\",\"ports\":[{\"containerPort\":80}],\"resources\":{\"limits\":{\"cpu\":\"0.2\",\"memory\":\"30Mi\"},\"requests\":{\"cpu\":\"0.1\",\"memory\":\"20Mi\"}}}]}}\n"}},"spec":{"volumes":[{"name":"default-token-4q2mj","secret":{"secretName":"default-token-4q2mj","defaultMode":420}}],"containers":[{"name":"hello-world","image":"hello-world:latest","ports":[{"containerPort":80,"protocol":"TCP"}],"resources":{"limits":{"cpu":"200m","memory":"30Mi"},"requests":{"cpu":"100m","memory":"20Mi"}},"volumeMounts":[{"name":"default-token-4q2mj","readOnly":true,"mountPath":"/var/run/secrets/kubernetes.io/serviceaccount"}],"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File","imagePullPolicy":"Always"}],"restartPolicy":"Always","terminationGracePeriodSeconds":30,"dnsPolicy":"ClusterFirst","serviceAccountName":"default","serviceAccount":"default","securityContext":{},"schedulerName":"default-scheduler","tolerations":[{"key":"node.kubernetes.io/not-ready","operator":"Exists","effect":"NoExecute","tolerationSeconds":300},{"key":"node.kubernetes.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":300}],"priority":0,"enableServiceLinks":true},"status":{"phase":"Pending","qosClass":"Burstable"}},"dryRun":false,"options":{"kind":"UpdateOptions","apiVersion":"meta.k8s.io/v1"}}`),
			userInfo:      []byte(`{"roles":null,"clusterRoles":null,"userInfo":{"username":"kubernetes-admin","groups":["system:masters","system:authenticated"]}}`),
			requestDenied: true,
		},
		{
			description:   "Blocks certain fields(failure case)",
			policy:        []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"prevent-field-update-failure"},"spec":{"validationFailureAction":"enforce","background":false,"rules":[{"name":"prevent-field-update","match":{"resources":{"selector":{"matchLabels":{"allow-updates":"false"}}}},"validate":{"message":"Updating field label 'something' is not allowed","deny":{"conditions":{"all":[{"key":"{{request.object.metadata.labels.something}}","operator":"NotEqual","value":""},{"key":"{{request.object.metadata.labels.something}}","operator":"NotEquals","value":"{{request.oldObject.metadata.labels.something}}"}]}}}}]}}`),
			request:       []byte(`{"uid":"cbdce9bb-741d-466a-a440-36155eb4b45b","kind":{"group":"","version":"v1","kind":"Pod"},"resource":{"group":"","version":"v1","resource":"pods"},"requestKind":{"group":"","version":"v1","kind":"Pod"},"requestResource":{"group":"","version":"v1","resource":"pods"},"name":"hello-world","namespace":"kube-system","operation":"CREATE","userInfo":{"username":"kubernetes-admin","groups":["system:masters","system:authenticated"]},"object":{"kind":"Pod","apiVersion":"v1","metadata":{"name":"hello-world","namespace":"kube-system","uid":"490c240c-f96a-4d5a-8860-75597bab0a7e","creationTimestamp":"2020-05-06T21:01:50Z","annotations":{"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"kind\":\"Pod\",\"metadata\":{\"annotations\":{},\"name\":\"hello-world\",\"namespace\":\"kube-system\"},\"spec\":{\"containers\":[{\"image\":\"hello-world:latest\",\"name\":\"hello-world\",\"ports\":[{\"containerPort\":80}],\"resources\":{\"limits\":{\"cpu\":\"0.2\",\"memory\":\"30Mi\"},\"requests\":{\"cpu\":\"0.1\",\"memory\":\"20Mi\"}}}]}}\n"}},"spec":{"volumes":[{"name":"default-token-8h2h8","secret":{"secretName":"default-token-8h2h8"}}],"containers":[{"name":"hello-world","image":"hello-world:latest","ports":[{"containerPort":80,"protocol":"TCP"}],"resources":{"limits":{"cpu":"200m","memory":"30Mi"},"requests":{"cpu":"100m","memory":"20Mi"}},"volumeMounts":[{"name":"default-token-8h2h8","readOnly":true,"mountPath":"/var/run/secrets/kubernetes.io/serviceaccount"}],"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File","imagePullPolicy":"Always"}],"restartPolicy":"Always","terminationGracePeriodSeconds":30,"dnsPolicy":"ClusterFirst","serviceAccountName":"default","serviceAccount":"default","securityContext":{},"schedulerName":"default-scheduler","tolerations":[{"key":"node.kubernetes.io/not-ready","operator":"Exists","effect":"NoExecute","tolerationSeconds":300},{"key":"node.kubernetes.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":300}],"priority":0,"enableServiceLinks":true},"status":{"phase":"Pending","qosClass":"Burstable"}},"oldObject":null,"dryRun":false,"options":{"kind":"CreateOptions","apiVersion":"meta.k8s.io/v1"}}`),
			userInfo:      []byte(`{"roles":null,"clusterRoles":null,"userInfo":{"username":"kubernetes-admin","groups":["system:masters","system:authenticated"]}}`),
			requestDenied: false,
		},
	}

	for _, testcase := range testcases {
		executeTest(t, testcase)
	}
}

func Test_BlockLabelRemove(t *testing.T) {
	testcases := []testCase{
		{
			description:   "Blocks certain fields(success case)",
			policy:        []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"prevent-label-remove"},"spec":{"validationFailureAction":"enforce","background":false,"rules":[{"name":"prevent-field-update","match":{"resources":{"selector":{"matchLabels":{"allow-updates":"false"}}}},"validate":{"message":"not allowed","deny":{"conditions":{"all":[{"key":"{{ request.operation }}","operator":"In","value":"[\"DELETE\", \"UPDATE\"]"}]}}}}]}}`),
			request:       []byte(`{"uid":"11d46f83-a31b-444e-8209-c43b24f1af8a","kind":{"group":"","version":"v1","kind":"Pod"},"resource":{"group":"","version":"v1","resource":"pods"},"requestKind":{"group":"","version":"v1","kind":"Pod"},"requestResource":{"group":"","version":"v1","resource":"pods"},"name":"hello-world","namespace":"default","operation":"UPDATE","userInfo":{"username":"kubernetes-admin","groups":["system:masters","system:authenticated"]},"object":{"kind":"Pod","apiVersion":"v1","metadata":{"name":"hello-world","namespace":"default","uid":"42bd0f0a-4b1f-4f7c-a40d-4dbed5522732","resourceVersion":"4333","creationTimestamp":"2020-05-06T20:51:58Z","labels":{"something":"exists","something2":"feeereeeeeeeee"},"annotations":{"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"kind\":\"Pod\",\"metadata\":{\"annotations\":{},\"labels\":{\"allow-updates\":\"false\",\"something\":\"existes\",\"something2\":\"feeereeeeeeeee\"},\"name\":\"hello-world\",\"namespace\":\"default\"},\"spec\":{\"containers\":[{\"image\":\"hello-world:latest\",\"name\":\"hello-world\",\"ports\":[{\"containerPort\":80}],\"resources\":{\"limits\":{\"cpu\":\"0.2\",\"memory\":\"30Mi\"},\"requests\":{\"cpu\":\"0.1\",\"memory\":\"20Mi\"}}}]}}\n"}},"spec":{"volumes":[{"name":"default-token-4q2mj","secret":{"secretName":"default-token-4q2mj","defaultMode":420}}],"containers":[{"name":"hello-world","image":"hello-world:latest","ports":[{"containerPort":80,"protocol":"TCP"}],"resources":{"limits":{"cpu":"200m","memory":"30Mi"},"requests":{"cpu":"100m","memory":"20Mi"}},"volumeMounts":[{"name":"default-token-4q2mj","readOnly":true,"mountPath":"/var/run/secrets/kubernetes.io/serviceaccount"}],"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File","imagePullPolicy":"Always"}],"restartPolicy":"Always","terminationGracePeriodSeconds":30,"dnsPolicy":"ClusterFirst","serviceAccountName":"default","serviceAccount":"default","securityContext":{},"schedulerName":"default-scheduler","tolerations":[{"key":"node.kubernetes.io/not-ready","operator":"Exists","effect":"NoExecute","tolerationSeconds":300},{"key":"node.kubernetes.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":300}],"priority":0,"enableServiceLinks":true},"status":{"phase":"Pending","qosClass":"Burstable"}},"oldObject":{"kind":"Pod","apiVersion":"v1","metadata":{"name":"hello-world","namespace":"default","uid":"42bd0f0a-4b1f-4f7c-a40d-4dbed5522732","resourceVersion":"4333","creationTimestamp":"2020-05-06T20:51:58Z","labels":{"allow-updates":"false","something":"exists","something2":"feeereeeeeeeee"},"annotations":{"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"kind\":\"Pod\",\"metadata\":{\"annotations\":{},\"labels\":{\"allow-updates\":\"false\",\"something\":\"exists\",\"something2\":\"feeereeeeeeeee\"},\"name\":\"hello-world\",\"namespace\":\"default\"},\"spec\":{\"containers\":[{\"image\":\"hello-world:latest\",\"name\":\"hello-world\",\"ports\":[{\"containerPort\":80}],\"resources\":{\"limits\":{\"cpu\":\"0.2\",\"memory\":\"30Mi\"},\"requests\":{\"cpu\":\"0.1\",\"memory\":\"20Mi\"}}}]}}\n"}},"spec":{"volumes":[{"name":"default-token-4q2mj","secret":{"secretName":"default-token-4q2mj","defaultMode":420}}],"containers":[{"name":"hello-world","image":"hello-world:latest","ports":[{"containerPort":80,"protocol":"TCP"}],"resources":{"limits":{"cpu":"200m","memory":"30Mi"},"requests":{"cpu":"100m","memory":"20Mi"}},"volumeMounts":[{"name":"default-token-4q2mj","readOnly":true,"mountPath":"/var/run/secrets/kubernetes.io/serviceaccount"}],"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File","imagePullPolicy":"Always"}],"restartPolicy":"Always","terminationGracePeriodSeconds":30,"dnsPolicy":"ClusterFirst","serviceAccountName":"default","serviceAccount":"default","securityContext":{},"schedulerName":"default-scheduler","tolerations":[{"key":"node.kubernetes.io/not-ready","operator":"Exists","effect":"NoExecute","tolerationSeconds":300},{"key":"node.kubernetes.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":300}],"priority":0,"enableServiceLinks":true},"status":{"phase":"Pending","qosClass":"Burstable"}},"dryRun":false,"options":{"kind":"UpdateOptions","apiVersion":"meta.k8s.io/v1"}}`),
			userInfo:      []byte(`{"roles":null,"clusterRoles":null,"userInfo":{"username":"kubernetes-admin","groups":["system:masters","system:authenticated"]}}`),
			requestDenied: true,
		},
	}

	for _, testcase := range testcases {
		executeTest(t, testcase)
	}
}

func Test_denyFeatureIssue744_BlockDelete(t *testing.T) {
	testcases := []testCase{
		{
			description:   "Blocks delete requests for resources with label allow-deletes(success case)",
			policy:        []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"block-deletes-success"},"spec":{"validationFailureAction":"enforce","background":false,"rules":[{"name":"check-allow-deletes","match":{"resources":{"selector":{"matchLabels":{"allow-deletes":"false"}}}},"exclude":{"clusterRoles":["random"]},"validate":{"message":"Deleting {{request.oldObject.kind}} / {{request.oldObject.metadata.name}} is not allowed","deny":{"conditions":{"all":[{"key":"{{request.operation}}","operator":"Equal","value":"DELETE"}]}}}}]}}`),
			request:       []byte(`{"uid":"b553344a-172a-4257-8ec4-a8f379f8b844","kind":{"group":"","version":"v1","kind":"Pod"},"resource":{"group":"","version":"v1","resource":"pods"},"requestKind":{"group":"","version":"v1","kind":"Pod"},"requestResource":{"group":"","version":"v1","resource":"pods"},"name":"hello-world","namespace":"default","operation":"DELETE","userInfo":{"username":"kubernetes-admin","groups":["system:masters","system:authenticated"]},"object":null,"oldObject":{"kind":"Pod","apiVersion":"v1","metadata":{"name":"hello-world","namespace":"default","uid":"f093e3da-f13a-474f-87e8-43e98fe363bf","resourceVersion":"1983","creationTimestamp":"2020-05-06T20:28:43Z","labels":{"allow-deletes":"false"},"annotations":{"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"kind\":\"Pod\",\"metadata\":{\"annotations\":{},\"labels\":{\"allow-deletes\":\"false\"},\"name\":\"hello-world\",\"namespace\":\"default\"},\"spec\":{\"containers\":[{\"image\":\"hello-world:latest\",\"name\":\"hello-world\",\"ports\":[{\"containerPort\":80}],\"resources\":{\"limits\":{\"cpu\":\"0.2\",\"memory\":\"30Mi\"},\"requests\":{\"cpu\":\"0.1\",\"memory\":\"20Mi\"}}}]}}\n"}},"spec":{"volumes":[{"name":"default-token-4q2mj","secret":{"secretName":"default-token-4q2mj","defaultMode":420}}],"containers":[{"name":"hello-world","image":"hello-world:latest","ports":[{"containerPort":80,"protocol":"TCP"}],"resources":{"limits":{"cpu":"200m","memory":"30Mi"},"requests":{"cpu":"100m","memory":"20Mi"}},"volumeMounts":[{"name":"default-token-4q2mj","readOnly":true,"mountPath":"/var/run/secrets/kubernetes.io/serviceaccount"}],"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File","imagePullPolicy":"Always"}],"restartPolicy":"Always","terminationGracePeriodSeconds":30,"dnsPolicy":"ClusterFirst","serviceAccountName":"default","serviceAccount":"default","nodeName":"kind-control-plane","securityContext":{},"schedulerName":"default-scheduler","tolerations":[{"key":"node.kubernetes.io/not-ready","operator":"Exists","effect":"NoExecute","tolerationSeconds":300},{"key":"node.kubernetes.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":300}],"priority":0,"enableServiceLinks":true},"status":{"phase":"Pending","conditions":[{"type":"Initialized","status":"True","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:28:43Z"},{"type":"Ready","status":"False","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:28:43Z","reason":"ContainersNotReady","message":"containers with unready status: [hello-world]"},{"type":"ContainersReady","status":"False","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:28:43Z","reason":"ContainersNotReady","message":"containers with unready status: [hello-world]"},{"type":"PodScheduled","status":"True","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:28:43Z"}],"hostIP":"172.17.0.2","startTime":"2020-05-06T20:28:43Z","containerStatuses":[{"name":"hello-world","state":{"waiting":{"reason":"ContainerCreating"}},"lastState":{},"ready":false,"restartCount":0,"image":"hello-world:latest","imageID":"","started":false}],"qosClass":"Burstable"}},"dryRun":false,"options":{"kind":"DeleteOptions","apiVersion":"meta.k8s.io/v1","gracePeriodSeconds":30,"propagationPolicy":"Background"}}`),
			userInfo:      []byte(`{"roles":null,"clusterRoles":["cluster-admin","system:basic-user","system:discovery","system:public-info-viewer"],"userInfo":{"username":"kubernetes-admin","groups":["system:masters","system:authenticated"]}}`),
			requestDenied: true,
		},
		{
			description:   "Blocks delete requests for resources with label allow-deletes(failure case)",
			policy:        []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"block-deletes-failure"},"spec":{"validationFailureAction":"enforce","background":false,"rules":[{"name":"check-allow-deletes","match":{"resources":{"selector":{"matchLabels":{"allow-deletes":"false"}}}},"exclude":{"clusterRoles":["random"]},"validate":{"message":"Deleting {{request.oldObject.kind}} / {{request.oldObject.metadata.name}} is not allowed","deny":{"conditions":{"all":[{"key":"{{request.operation}}","operator":"Equal","value":"DELETE"}]}}}}]}}`),
			request:       []byte(`{"uid":"9a83234d-95d1-4105-b6bf-7d72fd0183ce","kind":{"group":"","version":"v1","kind":"Pod"},"resource":{"group":"","version":"v1","resource":"pods"},"subResource":"status","requestKind":{"group":"","version":"v1","kind":"Pod"},"requestResource":{"group":"","version":"v1","resource":"pods"},"requestSubResource":"status","name":"hello-world","namespace":"default","operation":"UPDATE","userInfo":{"username":"system:node:kind-control-plane","groups":["system:nodes","system:authenticated"]},"object":{"kind":"Pod","apiVersion":"v1","metadata":{"name":"hello-world","namespace":"default","uid":"10fb7e1f-3710-43fa-9b7d-fc532b5ff70e","resourceVersion":"2829","creationTimestamp":"2020-05-06T20:36:51Z","labels":{"allow-deletes":"false"},"annotations":{"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"kind\":\"Pod\",\"metadata\":{\"annotations\":{},\"labels\":{\"allow-deletes\":\"false\"},\"name\":\"hello-world\",\"namespace\":\"default\"},\"spec\":{\"containers\":[{\"image\":\"hello-world:latest\",\"name\":\"hello-world\",\"ports\":[{\"containerPort\":80}],\"resources\":{\"limits\":{\"cpu\":\"0.2\",\"memory\":\"30Mi\"},\"requests\":{\"cpu\":\"0.1\",\"memory\":\"20Mi\"}}}]}}\n"}},"spec":{"volumes":[{"name":"default-token-4q2mj","secret":{"secretName":"default-token-4q2mj","defaultMode":420}}],"containers":[{"name":"hello-world","image":"hello-world:latest","ports":[{"containerPort":80,"protocol":"TCP"}],"resources":{"limits":{"cpu":"200m","memory":"30Mi"},"requests":{"cpu":"100m","memory":"20Mi"}},"volumeMounts":[{"name":"default-token-4q2mj","readOnly":true,"mountPath":"/var/run/secrets/kubernetes.io/serviceaccount"}],"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File","imagePullPolicy":"Always"}],"restartPolicy":"Always","terminationGracePeriodSeconds":30,"dnsPolicy":"ClusterFirst","serviceAccountName":"default","serviceAccount":"default","nodeName":"kind-control-plane","securityContext":{},"schedulerName":"default-scheduler","tolerations":[{"key":"node.kubernetes.io/not-ready","operator":"Exists","effect":"NoExecute","tolerationSeconds":300},{"key":"node.kubernetes.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":300}],"priority":0,"enableServiceLinks":true},"status":{"phase":"Pending","conditions":[{"type":"Initialized","status":"True","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:36:51Z"},{"type":"Ready","status":"False","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:36:51Z","reason":"ContainersNotReady","message":"containers with unready status: [hello-world]"},{"type":"ContainersReady","status":"False","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:36:51Z","reason":"ContainersNotReady","message":"containers with unready status: [hello-world]"},{"type":"PodScheduled","status":"True","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:36:51Z"}],"hostIP":"172.17.0.2","startTime":"2020-05-06T20:36:51Z","containerStatuses":[{"name":"hello-world","state":{"waiting":{"reason":"ContainerCreating"}},"lastState":{},"ready":false,"restartCount":0,"image":"hello-world:latest","imageID":"","started":false}],"qosClass":"Burstable"}},"oldObject":{"kind":"Pod","apiVersion":"v1","metadata":{"name":"hello-world","namespace":"default","uid":"10fb7e1f-3710-43fa-9b7d-fc532b5ff70e","resourceVersion":"2829","creationTimestamp":"2020-05-06T20:36:51Z","labels":{"allow-deletes":"false"},"annotations":{"kubectl.kubernetes.io/last-applied-configuration":"{\"apiVersion\":\"v1\",\"kind\":\"Pod\",\"metadata\":{\"annotations\":{},\"labels\":{\"allow-deletes\":\"false\"},\"name\":\"hello-world\",\"namespace\":\"default\"},\"spec\":{\"containers\":[{\"image\":\"hello-world:latest\",\"name\":\"hello-world\",\"ports\":[{\"containerPort\":80}],\"resources\":{\"limits\":{\"cpu\":\"0.2\",\"memory\":\"30Mi\"},\"requests\":{\"cpu\":\"0.1\",\"memory\":\"20Mi\"}}}]}}\n"}},"spec":{"volumes":[{"name":"default-token-4q2mj","secret":{"secretName":"default-token-4q2mj","defaultMode":420}}],"containers":[{"name":"hello-world","image":"hello-world:latest","ports":[{"containerPort":80,"protocol":"TCP"}],"resources":{"limits":{"cpu":"200m","memory":"30Mi"},"requests":{"cpu":"100m","memory":"20Mi"}},"volumeMounts":[{"name":"default-token-4q2mj","readOnly":true,"mountPath":"/var/run/secrets/kubernetes.io/serviceaccount"}],"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File","imagePullPolicy":"Always"}],"restartPolicy":"Always","terminationGracePeriodSeconds":30,"dnsPolicy":"ClusterFirst","serviceAccountName":"default","serviceAccount":"default","nodeName":"kind-control-plane","securityContext":{},"schedulerName":"default-scheduler","tolerations":[{"key":"node.kubernetes.io/not-ready","operator":"Exists","effect":"NoExecute","tolerationSeconds":300},{"key":"node.kubernetes.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":300}],"priority":0,"enableServiceLinks":true},"status":{"phase":"Pending","conditions":[{"type":"PodScheduled","status":"True","lastProbeTime":null,"lastTransitionTime":"2020-05-06T20:36:51Z"}],"qosClass":"Burstable"}},"dryRun":false,"options":{"kind":"UpdateOptions","apiVersion":"meta.k8s.io/v1"}}`),
			userInfo:      []byte(`{"roles":["kube-system:kubeadm:nodes-kubeadm-config","kube-system:kubeadm:kubelet-config-1.17"],"clusterRoles":["system:discovery","system:certificates.k8s.io:certificatesigningrequests:selfnodeclient","system:public-info-viewer","system:basic-user"],"userInfo":{"username":"kubernetes-admin","groups":["system:nodes","system:authenticated"]}}`),
			requestDenied: false,
		},
	}

	for _, testcase := range testcases {
		executeTest(t, testcase)
	}
}

func executeTest(t *testing.T, test testCase) {
	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(test.policy, &policy)
	if err != nil {
		t.Fatal(err)
	}

	var request *admissionv1.AdmissionRequest
	err = json.Unmarshal(test.request, &request)
	if err != nil {
		t.Fatal(err)
	}

	var userInfo urkyverno.RequestInfo
	err = json.Unmarshal(test.userInfo, &userInfo)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.NewContext()
	err = ctx.AddRequest(request)
	if err != nil {
		t.Fatal(err)
	}

	err = ctx.AddUserInfo(userInfo)
	if err != nil {
		t.Fatal(err)
	}

	err = ctx.AddServiceAccount(userInfo.AdmissionUserInfo.Username)
	if err != nil {
		t.Fatal(err)
	}

	newR, oldR, err := utils2.ExtractResources(nil, request)
	if err != nil {
		t.Fatal(err)
	}

	pc := &PolicyContext{
		Policy:        &policy,
		NewResource:   newR,
		OldResource:   oldR,
		AdmissionInfo: userInfo,
		JSONContext:   ctx,
	}

	resp := Validate(pc)
	if resp.IsSuccessful() && test.requestDenied {
		t.Errorf("Testcase has failed, policy: %v", policy.Name)
	}
}

func TestValidate_context_variable_substitution_CLI(t *testing.T) {
	rawPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "restrict-pod-count"
		},
		"spec": {
		  "validationFailureAction": "enforce",
		  "background": false,
		  "rules": [
			{
			  "name": "restrict-pod-count",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "context": [
				{
				  "name": "podcounts",
				  "apiCall": {
					"urlPath": "/api/v1/pods",
					"jmesPath": "items[?spec.nodeName=='minikube'] | length(@)"
				  }
				}
			  ],
			  "validate": {
				"message": "restrict pod counts to be no more than 10 on node minikube",
				"deny": {
				  "conditions": [
					{
					  "key": "{{ podcounts }}",
					  "operator": "GreaterThanOrEquals",
					  "value": 10
					}
				  ]
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
		  "name": "nginx-config-test"
		},
		"spec": {
		  "containers": [
			{
			  "image": "nginx:latest",
			  "name": "test-nginx"
			}
		  ]
		}
	  }
	`)

	configMapVariableContext := store.Context{
		Policies: []store.Policy{
			{
				Name: "restrict-pod-count",
				Rules: []store.Rule{
					{
						Name: "restrict-pod-count",
						Values: map[string]interface{}{
							"podcounts": "12",
						},
					},
				},
			},
		},
	}

	store.SetContext(configMapVariableContext)
	store.SetMock(true)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	msgs := []string{
		"restrict pod counts to be no more than 10 on node minikube",
	}
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	for index, r := range er.PolicyResponse.Rules {
		assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, !er.IsSuccessful())
}

func Test_EmptyStringInDenyCondition(t *testing.T) {
	policyRaw := []byte(`{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
	  "annotations": {
		"meta.helm.sh/release-name": "kyverno-policies",
		"meta.helm.sh/release-namespace": "kyverno",
		"pod-policies.kyverno.io/autogen-controllers": "none"
	  },
	  "labels": {
		"app.kubernetes.io/managed-by": "Helm"
	  },
	  "name": "if-baltic-restrict-external-load-balancer"
	},
	"spec": {
	  "background": true,
	  "rules": [
		{
		  "match": {
			"resources": {
			  "kinds": [
				"Service"
			  ]
			}
		  },
		  "name": "match-service-type",
		  "preconditions": [
			{
			  "key": "{{request.object.spec.type}}",
			  "operator": "Equals",
			  "value": "LoadBalancer"
			}
		  ],
		  "validate": {
			"deny": {
			  "conditions": [
				{
				  "key": "{{ request.object.metadata.annotations.\"service.beta.kubernetes.io/azure-load-balancer-internal\"}}",
				  "operator": "NotEquals",
				  "value": "true"
				}
			  ]
			}
		  }
		}
	  ],
	  "validationFailureAction": "enforce"
	}
  }`)

	resourceRaw := []byte(`{
	"apiVersion": "v1",
	"kind": "Service",
	"metadata": {
	  "name": "example-service"
	},
	"spec": {
	  "selector": {
		"app": "example"
	  },
	  "ports": [
		{
		  "port": 8765,
		  "targetPort": 9376
		}
	  ],
	  "type": "LoadBalancer"
	}
  }`)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(policyRaw, &policy)
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = context.AddResource(ctx, resourceRaw)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: ctx})
	assert.Assert(t, !er.IsSuccessful())
}

func Test_StringInDenyCondition(t *testing.T) {
	policyRaw := []byte(`{
	"apiVersion": "kyverno.io/v1",
	"kind": "ClusterPolicy",
	"metadata": {
	  "annotations": {
		"meta.helm.sh/release-name": "kyverno-policies",
		"meta.helm.sh/release-namespace": "kyverno",
		"pod-policies.kyverno.io/autogen-controllers": "none"
	  },
	  "labels": {
		"app.kubernetes.io/managed-by": "Helm"
	  },
	  "name": "if-baltic-restrict-external-load-balancer"
	},
	"spec": {
	  "background": true,
	  "rules": [
		{
		  "match": {
			"resources": {
			  "kinds": [
				"Service"
			  ]
			}
		  },
		  "name": "match-service-type",
		  "preconditions": [
			{
			  "key": "{{request.object.spec.type}}",
			  "operator": "Equals",
			  "value": "LoadBalancer"
			}
		  ],
		  "validate": {
			"deny": {
			  "conditions": [
				{
				  "key": "{{ request.object.metadata.annotations.\"service.beta.kubernetes.io/azure-load-balancer-internal\"}}",
				  "operator": "NotEquals",
				  "value": "true"
				}
			  ]
			}
		  }
		}
	  ],
	  "validationFailureAction": "enforce"
	}
  }`)

	resourceRaw := []byte(`{
	"apiVersion": "v1",
	"kind": "Service",
	"metadata": {
	  "name": "example-service",
	  "annotations": {
		"service.beta.kubernetes.io/azure-load-balancer-internal": "true"
	  }
	},
	"spec": {
	  "selector": {
		"app": "example"
	  },
	  "ports": [
		{
		  "port": 8765,
		  "targetPort": 9376
		}
	  ],
	  "type": "LoadBalancer"
	}
  }`)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(policyRaw, &policy)
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = context.AddResource(ctx, resourceRaw)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: ctx})
	assert.Assert(t, er.IsSuccessful())
}

func Test_foreach_container_pass(t *testing.T) {
	resourceRaw := []byte(`{
		"apiVersion": "v1",
		"kind": "Deployment",
		"metadata": {"name": "test"},
		"spec": { "template": { "spec": {
			"containers": [
				{"name": "pod1-valid", "image": "nginx/nginx:v1"},
				{"name": "pod2-valid", "image": "nginx/nginx:v2"},
				{"name": "pod3-valid", "image": "nginx/nginx:v3"}
			]
		}}}}`)

	policyraw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "test"
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
				"foreach": [
				  {
					"list": "request.object.spec.template.spec.containers",
					"pattern": {
					  "name": "*-valid"
					}
				  }
				]
			  }
			}
		  ]
		}
	  }`)

	testForEach(t, policyraw, resourceRaw, "", response.RuleStatusPass)
}

func Test_foreach_container_fail(t *testing.T) {
	resourceRaw := []byte(`{
		"apiVersion": "v1",
		"kind": "Deployment",
		"metadata": {"name": "test"},
		"spec": { "template": { "spec": {
			"containers": [
				{"name": "pod1-valid", "image": "nginx/nginx:v1"},
				{"name": "pod2-invalid", "image": "nginx/nginx:v2"},
				{"name": "pod3-valid", "image": "nginx/nginx:v3"}
			]
		}}}}`)

	policyraw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {"name": "test"},
		"spec": {
		  "rules": [
			{
			  "name": "test",
			  "match": {"resources": { "kinds": [ "Deployment" ] } },
			  "validate": {
				"foreach": [
				  {
					"list": "request.object.spec.template.spec.containers",
					"pattern": {
					  "name": "*-valid"
					}
				  }
				]
			}}]}}`)

	testForEach(t, policyraw, resourceRaw, "", response.RuleStatusFail)
}

func Test_foreach_container_deny_fail(t *testing.T) {
	resourceRaw := []byte(`{
		"apiVersion": "v1",
		"kind": "Deployment",
		"metadata": {"name": "test"},
		"spec": { "template": { "spec": {
			"containers": [
				{"name": "pod1-valid", "image": "nginx/nginx:v1"},
				{"name": "pod2-invalid", "image": "docker.io/nginx/nginx:v2"},
				{"name": "pod3-valid", "image": "nginx/nginx:v3"}
			]
		}}}}`)

	policyraw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "test"
		},
		"spec": {
		  "rules": [
			{
			  "name": "test",
			  "match": {
				"resources": {
				  "kinds": [
					"Deployment"
				  ]
				}
			  },
			  "validate": {
				"foreach": [
				  {
					"list": "request.object.spec.template.spec.containers",
					"deny": {
					  "conditions": [
						{
						  "key": "{{ regex_match('{{element.image}}', 'docker.io') }}",
						  "operator": "Equals",
						  "value": false
						}
					  ]
					}
				  }
				]
			  }
			}
		  ]
		}
	  }`)

	testForEach(t, policyraw, resourceRaw, "", response.RuleStatusFail)
}

func Test_foreach_container_deny_success(t *testing.T) {
	resourceRaw := []byte(`{
		"apiVersion": "v1",
		"kind": "Deployment",
		"metadata": {"name": "test"},
		"spec": { "template": { "spec": {
			"containers": [
				{"name": "pod1-valid", "image": "nginx/nginx:v1"},
				{"name": "pod2-invalid", "image": "nginx/nginx:v2"},
				{"name": "pod3-valid", "image": "nginx/nginx:v3"}
			]
		}}}}`)

	policyraw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {"name": "test"},
		"spec": {
		  "rules": [
			{
			  "name": "test",
			  "match": {"resources": { "kinds": [ "Deployment" ] } },
			  "validate": {
				"foreach": [
				  {
					"list": "request.object.spec.template.spec.containers",
					"deny": {
					  "conditions": [
						{
						  "key": "{{ regex_match('{{element.image}}', 'docker.io') }}",
						  "operator": "Equals",
						  "value": false
						}
					  ]
					}
				  }
				]
			}}]}}`)

	testForEach(t, policyraw, resourceRaw, "", response.RuleStatusFail)
}

func Test_foreach_container_deny_error(t *testing.T) {
	resourceRaw := []byte(`{
		"apiVersion": "v1",
		"kind": "Deployment",
		"metadata": {"name": "test"},
		"spec": { "template": { "spec": {
			"containers": [
				{"name": "pod1-valid", "image": "nginx/nginx:v1"},
				{"name": "pod2-invalid", "image": "nginx/nginx:v2"},
				{"name": "pod3-valid", "image": "nginx/nginx:v3"}
			]
		}}}}`)

	policyraw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "test"
		},
		"spec": {
		  "rules": [
			{
			  "name": "test",
			  "match": {
				"resources": {
				  "kinds": [
					"Deployment"
				  ]
				}
			  },
			  "validate": {
				"foreach": [
				  {
					"list": "request.object.spec.template.spec.containers",
					"deny": {
					  "conditions": [
						{
						  "key": "{{ regex_match_INVALID('{{request.object.image}}', 'docker.io') }}",
						  "operator": "Equals",
						  "value": false
						}
					  ]
					}
				  }
				]
			  }
			}
		  ]
		}
	  }`)

	testForEach(t, policyraw, resourceRaw, "", response.RuleStatusError)
}

func Test_foreach_context_preconditions(t *testing.T) {

	resourceRaw := []byte(`{
		"apiVersion": "v1",
		"kind": "Deployment",
		"metadata": {"name": "test"},
		"spec": { "template": { "spec": {
			"containers": [
				{"name": "podvalid", "image": "nginx/nginx:v1"},
				{"name": "podinvalid", "image": "nginx/nginx:v2"}
			]
		}}}}`)

	policyraw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "test"
		},
		"spec": {
		  "rules": [
			{
			  "name": "test",
			  "match": {
				"resources": {
				  "kinds": [
					"Deployment"
				  ]
				}
			  },
			  "validate": {
				"foreach": [
				  {
					"list": "request.object.spec.template.spec.containers",
					"context": [
					  {
						"name": "img",
						"configMap": {
						  "name": "mycmap",
						  "namespace": "default"
						}
					  }
					],
					"preconditions": {
					  "all": [
						{
						  "key": "{{element.name}}",
						  "operator": "In",
						  "value": [
							"podvalid"
						  ]
						}
					  ]
					},
					"deny": {
					  "conditions": [
						{
						  "key": "{{ element.image }}",
						  "operator": "NotEquals",
						  "value": "{{ img.data.{{ element.name }} }}"
						}
					  ]
					}
				  }
				]
			  }
			}
		  ]
		}
	  }`)

	configMapVariableContext := store.Context{
		Policies: []store.Policy{
			{
				Name: "test",
				Rules: []store.Rule{
					{
						Name: "test",
						Values: map[string]interface{}{
							"img.data.podvalid":   "nginx/nginx:v1",
							"img.data.podinvalid": "nginx/nginx:v2",
						},
					},
				},
			},
		},
	}

	store.SetContext(configMapVariableContext)
	store.SetMock(true)

	testForEach(t, policyraw, resourceRaw, "", response.RuleStatusPass)
}

func Test_foreach_context_preconditions_fail(t *testing.T) {

	resourceRaw := []byte(`{
		"apiVersion": "v1",
		"kind": "Deployment",
		"metadata": {"name": "test"},
		"spec": { "template": { "spec": {
			"containers": [
				{"name": "podvalid", "image": "nginx/nginx:v1"},
				{"name": "podinvalid", "image": "nginx/nginx:v2"}
			]
		}}}}`)

	policyraw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "test"
		},
		"spec": {
		  "rules": [
			{
			  "name": "test",
			  "match": {
				"resources": {
				  "kinds": [
					"Deployment"
				  ]
				}
			  },
			  "validate": {
				"foreach": [
				  {
					"list": "request.object.spec.template.spec.containers",
					"context": [
					  {
						"name": "img",
						"configMap": {
						  "name": "mycmap",
						  "namespace": "default"
						}
					  }
					],
					"preconditions": {
					  "all": [
						{
						  "key": "{{element.name}}",
						  "operator": "In",
						  "value": [
							"podvalid",
							"podinvalid"
						  ]
						}
					  ]
					},
					"deny": {
					  "conditions": [
						{
						  "key": "{{ element.image }}",
						  "operator": "NotEquals",
						  "value": "{{ img.data.{{ element.name }} }}"
						}
					  ]
					}
				  }
				]
			  }
			}
		  ]
		}
	  }`)

	configMapVariableContext := store.Context{
		Policies: []store.Policy{
			{
				Name: "test",
				Rules: []store.Rule{
					{
						Name: "test",
						Values: map[string]interface{}{
							"img.data.podvalid":   "nginx/nginx:v1",
							"img.data.podinvalid": "nginx/nginx:v1",
						},
					},
				},
			},
		},
	}

	store.SetContext(configMapVariableContext)
	store.SetMock(true)

	testForEach(t, policyraw, resourceRaw, "", response.RuleStatusFail)
}

func Test_foreach_element_validation(t *testing.T) {

	resourceRaw := []byte(`{
        "apiVersion": "v1",
        "kind": "Pod",
        "metadata": {
          "name": "nginx"
        },
        "spec": {
          "containers": [
            {
              "name": "nginx1",
              "image": "nginx"
            },
            {
              "name": "nginx2",
              "image": "nginx"
            }
          ]
        }
	}`)

	policyraw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {"name": "check-container-names"},
		"spec": {
		  "validationFailureAction": "enforce",
		  "background": false,
		  "rules": [
			{
			  "name": "test",
			  "match": {"resources": { "kinds": [ "Pod" ] } },
			  "validate": {
			  	"message": "Invalid name",
				"foreach": [
					{
					  "list": "request.object.spec.containers",
					  "pattern": {
						"name": "{{ element.name }}"
					  }
					}
				]
			}}]}}`)

	testForEach(t, policyraw, resourceRaw, "", response.RuleStatusPass)
}

func Test_outof_foreach_element_validation(t *testing.T) {

	resourceRaw := []byte(`{
        "apiVersion": "v1",
        "kind": "Pod",
        "metadata": {
          "name": "nginx"
        },
        "spec": {
          "containers": [
            {
              "name": "nginx1",
              "image": "nginx"
            },
            {
              "name": "nginx2",
              "image": "nginx"
            }
          ]
        }
	}`)

	policyraw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {"name": "check-container-names"},
		"spec": {
		  "validationFailureAction": "enforce",
		  "background": false,
		  "rules": [
			{
			  "name": "test",
			  "match": {"resources": { "kinds": [ "Pod" ] } },
			  "validate": {
			  	"message": "Invalid name",
				"pattern": {
				  "name": "{{ element.name }}"
				}
			}}]}}`)

	testForEach(t, policyraw, resourceRaw, "", response.RuleStatusError)
}

func Test_foreach_skip_initContainer_pass(t *testing.T) {

	resourceRaw := []byte(`{"apiVersion": "v1",
	"kind": "Deployment",
	"metadata": {"name": "test"},
	"spec": { "template": { "spec": {
		"containers": [
			{"name": "podvalid", "image": "nginx"}
		]
	}}}}`)

	policyraw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "check-images"
		},
		"spec": {
		  "validationFailureAction": "enforce",
		  "background": false,
		  "rules": [
			{
			  "name": "check-registry",
			  "match": {
				"resources": {
				  "kinds": [
					"Deployment"
				  ]
				}
			  },
			  "validate": {
				"message": "unknown registry",
				"foreach": [
				  {
					"list": "request.object.spec.template.spec.containers",
					"pattern": {
					  "image": "nginx"
					}
				  },
				  {
					"list": "request.object.spec.template.spec..initContainers",
					"pattern": {
					  "image": "trusted-registry.io/*"
					}
				  }
				]
			  }
			}
		  ]
		}
	  }`)

	testForEach(t, policyraw, resourceRaw, "", response.RuleStatusPass)
}

func testForEach(t *testing.T, policyraw []byte, resourceRaw []byte, msg string, status response.RuleStatus) {
	var policy kyverno.ClusterPolicy
	assert.NilError(t, json.Unmarshal(policyraw, &policy))
	resourceUnstructured, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = context.AddResource(ctx, resourceRaw)
	assert.NilError(t, err)

	policyContext := &PolicyContext{
		Policy:      &policy,
		JSONContext: ctx,
		NewResource: *resourceUnstructured}
	er := Validate(policyContext)

	assert.Equal(t, er.PolicyResponse.Rules[0].Status, status)
	if msg != "" {
		assert.Equal(t, er.PolicyResponse.Rules[0].Message, msg)
	}
}

func Test_delete_ignore_pattern(t *testing.T) {

	resourceRaw := []byte(`{
        "apiVersion": "v1",
        "kind": "Pod",
        "metadata": {
          "name": "nginx",
          "labels": {"app.kubernetes.io/foo" : "myapp-pod"}
        },
        "spec": {
          "containers": [
            {
              "name": "nginx1",
              "image": "nginx"
            },
            {
              "name": "nginx2",
              "image": "nginx"
            }
          ]
        }
	}`)

	policyRaw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {"name": "check-container-labels"},
		"spec": {
		  "validationFailureAction": "enforce",
		  "background": false,
		  "rules": [
			{
			  "name": "test",
			  "match": {"resources": { "kinds": [ "Pod" ] } },
			  "validate": {
			  	"message": "Invalid label",
				"pattern": {
				  "metadata" : {
                      "labels": {"app.kubernetes.io/name" : "myapp-pod"}
                  }
				}
			}}]}}`)

	var policy kyverno.ClusterPolicy
	assert.NilError(t, json.Unmarshal(policyRaw, &policy))
	resourceUnstructured, err := utils.ConvertToUnstructured(resourceRaw)
	assert.NilError(t, err)

	ctx := context.NewContext()
	err = context.AddResource(ctx, resourceRaw)
	assert.NilError(t, err)

	policyContextCreate := &PolicyContext{
		Policy:      &policy,
		JSONContext: ctx,
		NewResource: *resourceUnstructured}
	engineResponseCreate := Validate(policyContextCreate)
	assert.Equal(t, len(engineResponseCreate.PolicyResponse.Rules), 1)
	assert.Equal(t, engineResponseCreate.PolicyResponse.Rules[0].Status, response.RuleStatusFail)

	policyContextDelete := &PolicyContext{
		Policy:      &policy,
		JSONContext: ctx,
		OldResource: *resourceUnstructured}
	engineResponseDelete := Validate(policyContextDelete)
	assert.Equal(t, len(engineResponseDelete.PolicyResponse.Rules), 0)
}

// Pod security admission

// ====== Baseline ======

// === Control: "HostPath Volumes", check.ID: "hostPathVolumes"

// pod-level:
// - spec.volumes[*].hostPath

func TestValidate_pod_security_admission_enforce_baseline_exclude_all_host_path_volumes(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "HostPath Volumes"
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
			"volumes": [
				{
					"name": "hostpath-directory",
					"hostPath": {
						"path": "/var/local/aaa",
						"type": "DirectoryOrCreate"
					}
				},
				{
					"name": "hostpath-file",
					"hostPath": {
						"path": "/var/local/aaa/1.txt",
						"type": "FileOrCreate"
					}
				}
			],
		   "hostNetwork": false,
		   "securityContext": {
				"seccompProfile": {
					"type": "RuntimeDefault"
				},
				"runAsNonRoot": true,
				"allowPrivilegeEscalation": false
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_all_host_path_volumes_with_restrictedFields(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "HostPath Volumes",
								"restrictedField": "spec.volumes[*].hostPath",
								"values": [
									"/var/local/aaa",
									"/var/local/aaa/1.txt"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
			"volumes": [
				{
					"name": "hostpath-directory",
					"hostPath": {
						"path": "/var/local/aaa",
						"type": "DirectoryOrCreate"
					}
				},
				{
					"name": "hostpath-file",
					"hostPath": {
						"path": "/var/local/aaa/1.txt",
						"type": "FileOrCreate"
					}
				}
			],
		   "hostNetwork": false,
		   "securityContext": {
				"seccompProfile": {
					"type": "RuntimeDefault"
				},
				"runAsNonRoot": true,
				"allowPrivilegeEscalation": false
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_all_host_path_volume_missing_exclude_value(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "HostPath Volumes",
								"restrictedField": "spec.volumes[*].hostPath",
								"values": [
									"/var/local/aaa"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
			"volumes": [
				{
					"name": "hostpath-directory",
					"hostPath": {
						"path": "/var/local/aaa",
						"type": "DirectoryOrCreate"
					}
				},
				{
					"name": "hostpath-file",
					"hostPath": {
						"path": "/var/local/aaa/1.txt",
						"type": "FileOrCreate"
					}
				}
			],
		   "hostNetwork": false,
		   "securityContext": {
				"seccompProfile": {
					"type": "RuntimeDefault"
				},
				"runAsNonRoot": true,
				"allowPrivilegeEscalation": false
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_all_host_path_volume_missing_restrictedField(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Sysctls"
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
			"volumes": [
				{
					"name": "hostpath-directory",
					"hostPath": {
						"path": "/var/local/aaa",
						"type": "DirectoryOrCreate"
					}
				},
				{
					"name": "hostpath-file",
					"hostPath": {
						"path": "/var/local/aaa/1.txt",
						"type": "FileOrCreate"
					}
				}
			],
		   "hostNetwork": false,
		   "securityContext": {
				"seccompProfile": {
					"type": "RuntimeDefault"
				},
				"runAsNonRoot": true,
				"allowPrivilegeEscalation": false
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

// === Control: "AppArmor", check.ID: "appArmorProfile"

// metadata-level:
// - metadata.annotations['container.apparmor.security.beta.kubernetes.io/*']

func TestValidate_pod_security_admission_enforce_baseline_exclude_all_app_armor(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "AppArmor"
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging",
		   "annotations": {
				"container.apparmor.security.beta.kubernetes.io/":  "bogus",
				"container.apparmor.security.beta.kubernetes.io/a": "",
				"container.apparmor.security.beta.kubernetes.io/b": "runtime/default",
				"container.apparmor.security.beta.kubernetes.io/c": "localhost/",
				"container.apparmor.security.beta.kubernetes.io/d": "localhost/foo",
				"container.apparmor.security.beta.kubernetes.io/e": "unconfined",
				"container.apparmor.security.beta.kubernetes.io/f": "unknown"
			}
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx"
			  },
			{
				"name": "nodejs",
				"image": "nodejs"
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx"
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_all_app_armor_with_restrictedFields(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "AppArmor",
								"restrictedField": "metadata.annotations",
								"values": [
									"bogus",
									"unconfined",
									"unknown"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging",
		   "annotations": {
				"container.apparmor.security.beta.kubernetes.io/":  "bogus",
				"container.apparmor.security.beta.kubernetes.io/a": "",
				"container.apparmor.security.beta.kubernetes.io/b": "runtime/default",
				"container.apparmor.security.beta.kubernetes.io/c": "localhost/",
				"container.apparmor.security.beta.kubernetes.io/d": "localhost/foo",
				"container.apparmor.security.beta.kubernetes.io/e": "unconfined",
				"container.apparmor.security.beta.kubernetes.io/f": "unknown"
			}
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx"
			  },
			{
				"name": "nodejs",
				"image": "nodejs"
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx"
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_all_app_armor_missing_exclude_value(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "AppArmor",
								"restrictedField": "metadata.annotations",
								"values": [
									"bogus",
									"unconfined"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging",
		   "annotations": {
				"container.apparmor.security.beta.kubernetes.io/":  "bogus",
				"container.apparmor.security.beta.kubernetes.io/a": "",
				"container.apparmor.security.beta.kubernetes.io/b": "runtime/default",
				"container.apparmor.security.beta.kubernetes.io/c": "localhost/",
				"container.apparmor.security.beta.kubernetes.io/d": "localhost/foo",
				"container.apparmor.security.beta.kubernetes.io/e": "unconfined",
				"container.apparmor.security.beta.kubernetes.io/f": "unknown"
			}
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx"
			  },
			{
				"name": "nodejs",
				"image": "nodejs"
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx"
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

// === Control: "Sysctls", check.ID: "sysctls"

// pod-level:
// - spec.securityContext.sysctls[*].name

func TestValidate_pod_security_admission_enforce_baseline_exclude_all_sysctls(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Sysctls"
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"sysctls": [
					{
						"name": "a"
					},
					{
						"name": "b"
					} 
				]
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx"
			  },
			{
				"name": "nodejs",
				"image": "nodejs"
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx"
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_all_sysctls_with_restrictedFields(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Sysctls",
								"restrictedField": "spec.securityContext.sysctls[*].name",
								"values": [
									"a",
									"b"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"sysctls": [
					{
						"name": "a"
					},
					{
						"name": "b"
					} 
				]
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx"
			  },
			{
				"name": "nodejs",
				"image": "nodejs"
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx"
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_all_sysctls_with_missing_exclude_value(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Sysctls",
								"restrictedField": "spec.securityContext.sysctls[*].name",
								"values": [
									"fdsfds",
									"fdfdsdddd"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"sysctls": [
					{
						"name": "kernel.shm_rmid_forced"
					},
					{
						"name": "b"
					} 
				]
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx"
			  },
			{
				"name": "nodejs",
				"image": "nodejs"
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx"
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

// === Control: "Seccomp", check.ID: "seccompProfile_baseline"

// pod-level:
// - spec.securityContext.seccompProfile.type

// container-level:
// - spec.containers[*].securityContext.seccompProfile.type
// - spec.initContainers[*].securityContext.seccompProfile.type
// - spec.ephemeralContainers[*].securityContext.seccompProfile.type

func TestValidate_pod_security_admission_enforce_baseline_exclude_all_seccomp(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Seccomp",
								"images": [
									"nginx",
									"nodejs"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"seccompProfile": {
					"type": "Unconfined"
				}
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "Unconfined"
					}
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "Unconfined"
					}
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "Unconfined"
						}
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "Unconfined"
						}
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_seccomp_with_restrictedField(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Seccomp",
								"restrictedField": "spec.securityContext.seccompProfile.type",
								"values": [
									"randomValue1"
								]
							},
							{
								"controlName": "Seccomp",
								"restrictedField":  "spec.containers[*].securityContext.seccompProfile.type",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"randomValue2"
								]
							},
							{
								"controlName": "Seccomp",
								"restrictedField":  "spec.initContainers[*].securityContext.seccompProfile.type",
								"images": [
									"nginx"
								],
								"values": [
									"randomValue3"
								]
							},
							{
								"controlName": "Seccomp",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.seccompProfile.type",
								"images": [
									"nginx"
								],
								"values": [
									"randomValue4"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"seccompProfile": {
					"type": "randomValue1"
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
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "Localhost"
					}
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "randomValue3"
						}
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "randomValue4"
						}
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_seccomp_missing_exclude_value(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Seccomp",
								"restrictedField": "spec.securityContext.seccompProfile.type",
								"values": [
									"randomValue1"
								]
							},
							{
								"controlName": "Seccomp",
								"restrictedField":  "spec.containers[*].securityContext.seccompProfile.type",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"NotMatchingValue"
								]
							},
							{
								"controlName": "Seccomp",
								"restrictedField":  "spec.initContainers[*].securityContext.seccompProfile.type",
								"images": [
									"nginx"
								],
								"values": [
									"randomValue3"
								]
							},
							{
								"controlName": "Seccomp",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.seccompProfile.type",
								"images": [
									"nginx"
								],
								"values": [
									"randomValue4"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"seccompProfile": {
					"type": "randomValue1"
				}
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "randomValue2"
					}
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "Localhost"
					}
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "randomValue3"
						}
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "randomValue4"
						}
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_seccomp_missing_restrictedField(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Seccomp",
								"restrictedField": "spec.securityContext.seccompProfile.type",
								"values": [
									"randomValue1"
								]
							},
							{
								"controlName": "Seccomp",
								"restrictedField":  "spec.containers[*].securityContext.seccompProfile.type",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"NotMatchingValue"
								]
							},
							{
								"controlName": "Seccomp",
								"restrictedField":  "spec.initContainers[*].securityContext.seccompProfile.type",
								"images": [
									"nginx"
								],
								"values": [
									"randomValue3"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"seccompProfile": {
					"type": "randomValue1"
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
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "Localhost"
					}
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "randomValue3"
						}
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "randomValue4"
						}
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

// pod-level:
// - spec.securityContext.seLinuxOptions.type

// container-level:
// Type
// - spec.containers[*].securityContext.seLinuxOptions.type
// - spec.initContainers[*].securityContext.seLinuxOptions.type
// - spec.ephemeralContainers[*].securityContext.seLinuxOptions.type

// User
// - spec.securityContext.seLinuxOptions.user
// - spec.containers[*].securityContext.seLinuxOptions.user
// - spec.initContainers[*].securityContext.seLinuxOptions.user
// - spec.ephemeralContainers[*].securityContext.seLinuxOptions.user

// Role
// - spec.securityContext.seLinuxOptions.role
// - spec.containers[*].securityContext.seLinuxOptions.role
// - spec.initContainers[*].securityContext.seLinuxOptions.role
// - spec.ephemeralContainers[*].securityContext.seLinuxOptions.role

func TestValidate_pod_security_admission_enforce_baseline_exclude_all_SELinuxOptions(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "SELinux",
								"images": [
									"nginx",
									"nodejs"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"seLinuxOptions": {
					"type": "foo",
					"user": "bar",
					"role": "baz"
				}
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seLinuxOptions": {
						"type": "foo"
					}
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seLinuxOptions": {
						"user": "bar"
					}
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seLinuxOptions": {
							"role": "baz"
						}
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seLinuxOptions": {
							"role": "baz"
						}
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

// TO DO
// func TestValidate_pod_security_admission_enforce_baseline_exclude_SELinuxOptions_pod_level_restrictedFields(t *testing.T) {
// 	rawPolicy := []byte(`
// 	{
// 		"apiVersion": "kyverno.io/v1",
// 		"kind": "ClusterPolicy",
// 		"metadata": {
// 		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
// 		},
// 		"spec": {
// 			"validationFailureAction": "enforce",
// 			"rules": [
// 				{
// 				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
// 				"match": {
// 					"resources": {
// 					   "kinds": [
// 						  "Pod"
// 						],
// 						"namespaces": [
// 							"staging"
// 						]
// 					}
// 				 },
// 				 "validate": {
// 					"podSecurity": {
// 						"level": "baseline",
// 						"version": "v1.24",
// 						"exclude": [
// 							{
// 								"controlName": "SELinux",
// 								"restrictedField": "spec.securityContext.seLinuxOptions.type",
// 								"values": [
// 									"foo"
// 								]
// 							},
// 							{
// 								"controlName": "SELinux",
// 								"restrictedField": "spec.securityContext.seLinuxOptions.user",
// 								"values": [
// 									"bar"
// 								]
// 							},
// 							{
// 								"controlName": "SELinux",
// 								"restrictedField": "spec.containers[*].securityContext.seLinuxOptions.type",
// 								"images": [
// 									"nginx",
// 									"nodejs"
// 								],
// 								"values": [
// 									"foo"
// 								]
// 							},
// 							{
// 								"controlName": "SELinux",
// 								"restrictedField": "spec.initContainers[*].securityContext.seLinuxOptions.role",
// 								"images": [
// 									"nginx"
// 								],
// 								"values": [
// 									"baz"
// 								]
// 							},
// 							{
// 								"controlName": "SELinux",
// 								"restrictedField": "spec.ephemeralContainers[*].securityContext.seLinuxOptions.role",
// 								"images": [
// 									"nginx"
// 								],
// 								"values": [
// 									"baz"
// 								]
// 							}
// 						]
// 					}
// 				 }
// 			  }
// 		   ]
// 		}
// 	 }
// 	 `)

// 	rawResource := []byte(`
// 	 {
// 		"apiVersion": "v1",
// 		"kind": "Pod",
// 		"metadata": {
// 		   "name": "nginx-baseline-privileged-container",
// 		   "namespace": "staging"
// 		},
// 		"spec": {
// 		   "hostNetwork": false,
// 		   "securityContext": {
// 				"seLinuxOptions": {
// 					"type": "foo",
// 					"user": "bar"
// 				}
// 		   },
// 		   "containers": [
// 			{
// 				 "name": "nginx",
// 				 "image": "nginx",
// 				 "securityContext": {
// 					"seLinuxOptions": {
// 						"type": "foo"
// 					}
// 				 }
// 			  },
// 			{
// 				"name": "nodejs",
// 				"image": "nodejs",
// 				"securityContext": {
// 					"seLinuxOptions": {
// 						"user": "foo"
// 					}
// 				}
// 			 }
// 			],
// 			"initContainers": [
// 				{
// 				   "name": "init-nginx",
// 				   "image": "nginx",
// 				   "securityContext": {
// 						"seLinuxOptions": {
// 							"role": "baz"
// 						}
// 					}
// 				}
// 			],
// 			"ephemeralContainers": [
// 				{
// 				   "name": "ephemeral-nginx",
// 				   "image": "nginx",
// 				   "securityContext": {
// 						"seLinuxOptions": {
// 							"role": "baz"
// 						}
// 					}
// 				}
// 			]
// 		}
// 	 }
// 	 `)

// 	var policy kyverno.ClusterPolicy
// 	err := json.Unmarshal(rawPolicy, &policy)
// 	assert.NilError(t, err)

// 	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
// 	assert.NilError(t, err)
// 	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

// 	fmt.Println(er)
// 	// msgs := []string{""}

// 	for _, r := range er.PolicyResponse.Rules {
// 		fmt.Printf("== Response: %+v\n", r.Message)
// 		// assert.Equal(t, r.Message, msgs[index])
// 	}
// 	assert.Assert(t, er.IsSuccessful())
// }

func TestValidate_pod_security_admission_enforce_baseline_exclude_SELinuxOptions_with_restrictedFields(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "SELinux",
								"restrictedField": "spec.securityContext.seLinuxOptions.type",
								"values": [
									"foo"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.securityContext.seLinuxOptions.user",
								"values": [
									"bar"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.securityContext.seLinuxOptions.role",
								"values": [
									"baz"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.containers[*].securityContext.seLinuxOptions.type",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"foo"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.initContainers[*].securityContext.seLinuxOptions.role",
								"images": [
									"nginx"
								],
								"values": [
									"baz"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.seLinuxOptions.role",
								"images": [
									"nginx"
								],
								"values": [
									"baz"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"seLinuxOptions": {
					"type": "foo",
					"user": "bar",
					"role": "baz"
				}
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seLinuxOptions": {
						"type": "foo"
					}
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seLinuxOptions": {
						"user": "foo"
					}
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seLinuxOptions": {
							"role": "baz"
						}
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seLinuxOptions": {
							"role": "baz"
						}
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_SELinuxOptions_with_restrictedFields_only_containers(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "SELinux",
								"restrictedField": "spec.containers[*].securityContext.seLinuxOptions.type",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"foo"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.initContainers[*].securityContext.seLinuxOptions.role",
								"images": [
									"nginx"
								],
								"values": [
									"baz"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.seLinuxOptions.role",
								"images": [
									"nginx"
								],
								"values": [
									"baz"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seLinuxOptions": {
						"type": "foo"
					}
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seLinuxOptions": {
						"user": "foo"
					}
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seLinuxOptions": {
							"role": "baz"
						}
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seLinuxOptions": {
							"role": "baz"
						}
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_SELinuxOptions_with_missing_exclude(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "SELinux",
								"restrictedField": "spec.securityContext.seLinuxOptions.type",
								"values": [
									"randomValue"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.securityContext.seLinuxOptions.user",
								"values": [
									"bar"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.securityContext.seLinuxOptions.role",
								"values": [
									"baz"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.containers[*].securityContext.seLinuxOptions.type",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"foo"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.initContainers[*].securityContext.seLinuxOptions.role",
								"images": [
									"nginx"
								],
								"values": [
									"baz"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.seLinuxOptions.role",
								"images": [
									"nginx"
								],
								"values": [
									"baz"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"seLinuxOptions": {
					"type": "foo",
					"user": "bar",
					"role": "baz"
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
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seLinuxOptions": {
						"user": "bar"
					}
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seLinuxOptions": {
							"role": "baz"
						}
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seLinuxOptions": {
							"role": "baz"
						}
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_SELinuxOptions_with_missing_restrictedField(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "SELinux",
								"restrictedField": "spec.securityContext.seLinuxOptions.type",
								"values": [
									"foo"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.securityContext.seLinuxOptions.user",
								"values": [
									"bar"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.securityContext.seLinuxOptions.role",
								"values": [
									"baz"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.containers[*].securityContext.seLinuxOptions.type",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"foo"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.initContainers[*].securityContext.seLinuxOptions.role",
								"images": [
									"nginx"
								],
								"values": [
									"baz"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.seLinuxOptions.role",
								"images": [
									"nginx"
								],
								"values": [
									"baz"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"seLinuxOptions": {
					"type": "foo",
					"user": "bar",
					"role": "baz"
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
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seLinuxOptions": {
						"user": "bar"
					}
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seLinuxOptions": {
							"role": "baz"
						}
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seLinuxOptions": {
							"role": "baz"
						}
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_SELinuxOptions_missing_exclude_value_deployment_autogen(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "SELinux",
								"restrictedField": "spec.containers[*].securityContext.seLinuxOptions.type",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"baz"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.initContainers[*].securityContext.seLinuxOptions.role",
								"images": [
									"nodejs"
								],
								"values": [
									"randomValue"
								]
							}
						]
					}
				 }
			  },
			  {
				"name": "autogen-enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
							"DaemonSet",
							"Deployment",
							"Job",
							"StatefulSet"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "SELinux",
								"restrictedField": "spec.template.spec.containers[*].securityContext.seLinuxOptions.type",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"baz"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.template.spec.initContainers[*].securityContext.seLinuxOptions.role",
								"images": [
									"nodejs"
								],
								"values": [
									"randomValue"
								]
							}
						]
					}
				 }
			  },
			  {
				"name": "autogen-cronjob-enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
							"CronJob"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "SELinux",
								"restrictedField": "spec.jobTemplate.spec.template.spec.containers[*].securityContext.seLinuxOptions.type",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"baz"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.jobTemplate.spec.template.spec.initContainers[*].securityContext.seLinuxOptions.role",
								"images": [
									"nodejs"
								],
								"values": [
									"randomValue"
								]
							}
						]
					}
				 }
			  }
		   ]
		}
	 }
	 `)

	rawResource := []byte(`
	{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
		  "name": "nginx-deployment",
		  "namespace": "staging",
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
				  "image": "nginx",
				  "name": "nginx",
				  "resources": {},
				  "securityContext": {
					"seLinuxOptions": {
					  "role": "baz"
					}
				  }
				}
			  ],
			  "initContainers": [
				{
				  "image": "nodejs",
				  "name": "init-nodejs",
				  "resources": {},
				  "securityContext": {
					"seLinuxOptions": {
					  "role": "init-baz"
					}
				  }
				}
			  ]
			}
		  }
		}
	  }
	 `)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	if err != nil {
		fmt.Printf("=== Error: %+v\n", er)
	}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
	}
	assert.Assert(t, er.IsFailed())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_SELinuxOptions_missing_exclude_value_daemonset_autogen(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "SELinux",
								"restrictedField": "spec.containers[*].securityContext.seLinuxOptions.type",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"baz"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.initContainers[*].securityContext.seLinuxOptions.role",
								"images": [
									"nodejs"
								],
								"values": [
									"randomValue"
								]
							}
						]
					}
				 }
			  },
			  {
				"name": "autogen-enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
							"DaemonSet",
							"Deployment",
							"Job",
							"StatefulSet"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "SELinux",
								"restrictedField": "spec.template.spec.containers[*].securityContext.seLinuxOptions.type",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"baz"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.template.spec.initContainers[*].securityContext.seLinuxOptions.role",
								"images": [
									"nodejs"
								],
								"values": [
									"randomValue"
								]
							}
						]
					}
				 }
			  },
			  {
				"name": "autogen-cronjob-enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
							"CronJob"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "SELinux",
								"restrictedField": "spec.jobTemplate.spec.template.spec.containers[*].securityContext.seLinuxOptions.type",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"baz"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.jobTemplate.spec.template.spec.initContainers[*].securityContext.seLinuxOptions.role",
								"images": [
									"nodejs"
								],
								"values": [
									"randomValue"
								]
							}
						]
					}
				 }
			  }
		   ]
		}
	 }
	 `)

	rawResource := []byte(`
	{
		"apiVersion": "apps/v1",
		"kind": "DaemonSet",
		"metadata": {
		  "name": "nginx-daemonset",
		  "namespace": "staging",
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
				  "image": "nginx",
				  "name": "nginx",
				  "resources": {},
				  "securityContext": {
					"seLinuxOptions": {
					  "role": "baz"
					}
				  }
				}
			  ],
			  "initContainers": [
				{
				  "image": "nodejs",
				  "name": "init-nodejs",
				  "resources": {},
				  "securityContext": {
					"seLinuxOptions": {
					  "role": "init-baz"
					}
				  }
				}
			  ]
			}
		  }
		}
	  }
	 `)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	if err != nil {
		fmt.Printf("=== Error: %+v\n", er)
	}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
	}
	assert.Assert(t, er.IsFailed())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_SELinuxOptions_missing_exclude_value_job_autogen(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "SELinux",
								"restrictedField": "spec.containers[*].securityContext.seLinuxOptions.type",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"baz"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.initContainers[*].securityContext.seLinuxOptions.role",
								"images": [
									"nodejs"
								],
								"values": [
									"randomValue"
								]
							}
						]
					}
				 }
			  },
			  {
				"name": "autogen-enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
							"DaemonSet",
							"Deployment",
							"Job",
							"StatefulSet"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "SELinux",
								"restrictedField": "spec.template.spec.containers[*].securityContext.seLinuxOptions.type",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"baz"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.template.spec.initContainers[*].securityContext.seLinuxOptions.role",
								"images": [
									"nodejs"
								],
								"values": [
									"randomValue"
								]
							}
						]
					}
				 }
			  },
			  {
				"name": "autogen-cronjob-enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
							"CronJob"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "SELinux",
								"restrictedField": "spec.jobTemplate.spec.template.spec.containers[*].securityContext.seLinuxOptions.type",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"baz"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.jobTemplate.spec.template.spec.initContainers[*].securityContext.seLinuxOptions.role",
								"images": [
									"nodejs"
								],
								"values": [
									"randomValue"
								]
							}
						]
					}
				 }
			  }
		   ]
		}
	 }
	 `)

	rawResource := []byte(`
	{
		"apiVersion": "batch/v1",
		"kind": "Job",
		"metadata": {
		  "name": "nginx-daemonset",
		  "namespace": "staging",
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
				  "image": "nginx",
				  "name": "nginx",
				  "resources": {},
				  "securityContext": {
					"seLinuxOptions": {
					  "role": "baz"
					}
				  }
				}
			  ],
			  "initContainers": [
				{
				  "image": "nodejs",
				  "name": "init-nodejs",
				  "resources": {},
				  "securityContext": {
					"seLinuxOptions": {
					  "role": "init-baz"
					}
				  }
				}
			  ]
			}
		  }
		}
	  }
	 `)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	if err != nil {
		fmt.Printf("=== Error: %+v\n", er)
	}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
	}
	assert.Assert(t, er.IsFailed())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_SELinuxOptions_missing_exclude_value_statefulset_autogen(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "SELinux",
								"restrictedField": "spec.containers[*].securityContext.seLinuxOptions.type",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"baz"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.initContainers[*].securityContext.seLinuxOptions.role",
								"images": [
									"nodejs"
								],
								"values": [
									"init-randomValue"
								]
							}
						]
					}
				 }
			  },
			  {
				"name": "autogen-enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
							"DaemonSet",
							"Deployment",
							"Job",
							"StatefulSet"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "SELinux",
								"restrictedField": "spec.template.spec.containers[*].securityContext.seLinuxOptions.type",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"baz"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.template.spec.initContainers[*].securityContext.seLinuxOptions.role",
								"images": [
									"nodejs"
								],
								"values": [
									"randomValue"
								]
							}
						]
					}
				 }
			  },
			  {
				"name": "autogen-cronjob-enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
							"CronJob"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "SELinux",
								"restrictedField": "spec.jobTemplate.spec.template.spec.containers[*].securityContext.seLinuxOptions.type",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"baz"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.jobTemplate.spec.template.spec.initContainers[*].securityContext.seLinuxOptions.role",
								"images": [
									"nodejs"
								],
								"values": [
									"randomValue"
								]
							}
						]
					}
				 }
			  }
		   ]
		}
	 }
	 `)

	rawResource := []byte(`
	{
		"apiVersion": "apps/v1",
		"kind": "StatefulSet",
		"metadata": {
		  "name": "nginx-daemonset",
		  "namespace": "staging",
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
				  "image": "nginx",
				  "name": "nginx",
				  "resources": {},
				  "securityContext": {
					"seLinuxOptions": {
					  "role": "baz"
					}
				  }
				}
			  ],
			  "initContainers": [
				{
				  "image": "nodejs",
				  "name": "init-nodejs",
				  "resources": {},
				  "securityContext": {
					"seLinuxOptions": {
					  "role": "init-baz"
					}
				  }
				}
			  ]
			}
		  }
		}
	  }
	 `)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	if err != nil {
		fmt.Printf("=== Error: %+v\n", er)
	}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
	}
	assert.Assert(t, er.IsFailed())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_SELinuxOptions_missing_exclude_value_cronjob_autogen(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "SELinux",
								"restrictedField": "spec.containers[*].securityContext.seLinuxOptions.type",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"baz"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.initContainers[*].securityContext.seLinuxOptions.role",
								"images": [
									"nodejs"
								],
								"values": [
									"init-randomValue"
								]
							}
						]
					}
				 }
			  },
			  {
				"name": "autogen-enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
							"DaemonSet",
							"Deployment",
							"Job",
							"StatefulSet"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "SELinux",
								"restrictedField": "spec.template.spec.containers[*].securityContext.seLinuxOptions.type",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"baz"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.template.spec.initContainers[*].securityContext.seLinuxOptions.role",
								"images": [
									"nodejs"
								],
								"values": [
									"randomValue"
								]
							}
						]
					}
				 }
			  },
			  {
				"name": "autogen-cronjob-enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
							"CronJob"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "SELinux",
								"restrictedField": "spec.jobTemplate.spec.template.spec.containers[*].securityContext.seLinuxOptions.type",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"baz"
								]
							},
							{
								"controlName": "SELinux",
								"restrictedField": "spec.jobTemplate.spec.template.spec.initContainers[*].securityContext.seLinuxOptions.role",
								"images": [
									"nodejs"
								],
								"values": [
									"randomValue"
								]
							}
						]
					}
				 }
			  }
		   ]
		}
	 }
	 `)

	rawResource := []byte(`
	{
		"apiVersion": "batch/v1",
		"kind": "CronJob",
		"metadata": {
		  "name": "cronjob-nginx",
		  "namespace": "staging"
		},
		"spec": {
		  "schedule": "* * * * *",
		  "jobTemplate": {
			"spec": {
			  "template": {
				"spec": {
					"containers": [
						{
						  "image": "nginx",
						  "name": "nginx",
						  "resources": {},
						  "securityContext": {
							"seLinuxOptions": {
							  "role": "baz"
							}
						  }
						}
					  ],
					  "initContainers": [
						{
						  "image": "nodejs",
						  "name": "init-nodejs",
						  "resources": {},
						  "securityContext": {
							"seLinuxOptions": {
							  "role": "init-baz"
							}
						  }
						}
					  ],
				  "restartPolicy": "OnFailure"
				}
			  }
			}
		  }
		}
	  }
	 `)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	if err != nil {
		fmt.Printf("=== Error: %+v\n", er)
	}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
	}
	assert.Assert(t, er.IsFailed())
}

// === Control: "HostProcess", check.ID: "windowsHostProcess"
// pod-level:
// - spec.securityContext.windowsOptions.hostProcess

// container-level:
// - spec.containers[*].securityContext.windowsOptions.hostProcess
// - spec.initContainers[*].securityContext.windowsOptions.hostProcess
// - spec.ephemeralContainers[*].securityContext.windowsOptions.hostProcess
func TestValidate_pod_security_admission_enforce_baseline_exclude_all_hostProcesses(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "HostProcess",
								"images": [
									"nginx",
									"nodejs"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"windowsOptions": {
					"hostProcess": true
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
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
				   "windowsOptions": {
					   "hostProcess": true
				   }
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"windowsOptions": {
							"hostProcess": true
						}
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_hostProcesses_with_restrictedFields(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "HostProcess",
								"restrictedField": "spec.securityContext.windowsOptions.hostProcess",
								"values": [
									"true"
								]
							},
							{
								"controlName": "HostProcess",
								"restrictedField": "spec.containers[*].securityContext.windowsOptions.hostProcess",
								"images": [
									"nginx:1.2.3",
									"nodejs:1.2.3"
								],
								"values": [
									"true"
								]
							},
							{
								"controlName": "HostProcess",
								"restrictedField": "spec.initContainers[*].securityContext.windowsOptions.hostProcess",
								"images": [
									"nginx:1.2.3"
								],
								"values": [
									"true"
								]
							},
							{
								"controlName": "HostProcess",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.windowsOptions.hostProcess",
								"images": [
									"nginx:1.2.3"
								],
								"values": [
									"true"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"windowsOptions": {
					"hostProcess": true
				}
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx:1.2.3",
				 "securityContext": {
					"windowsOptions": {
						"hostProcess": true
					}
				 }
			},
			{
				"name": "nodejs",
				"image": "nodejs:1.2.3",
				"securityContext": {
				   "windowsOptions": {
					   "hostProcess": true
				   }
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx:1.2.3",
				   "securityContext": {
						"windowsOptions": {
							"hostProcess": true
						}
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx:1.2.3",
				   "securityContext": {
						"windowsOptions": {
							"hostProcess": true
						}
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_hostProcesses_missing_exlude_value(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "HostProcess",
								"restrictedField": "spec.securityContext.windowsOptions.hostProcess",
								"values": [
									"RandomValue"
								]
							},
							{
								"controlName": "HostProcess",
								"restrictedField": "spec.containers[*].securityContext.windowsOptions.hostProcess",
								"images": [
									"nginx:1.2.3",
									"nodejs:1.2.3"
								],
								"values": [
									"RandomValue"
								]
							},
							{
								"controlName": "HostProcess",
								"restrictedField": "spec.initContainers[*].securityContext.windowsOptions.hostProcess",
								"images": [
									"nginx:1.2.3"
								],
								"values": [
									"true"
								]
							},
							{
								"controlName": "HostProcess",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.windowsOptions.hostProcess",
								"images": [
									"nginx:1.2.3"
								],
								"values": [
									"true"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"windowsOptions": {
					"hostProcess": true
				}
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx:1.2.3",
				 "securityContext": {
					"windowsOptions": {
						"hostProcess": true
					}
				 }
			},
			{
				"name": "nodejs",
				"image": "nodejs:1.2.3",
				"securityContext": {
				   "windowsOptions": {
					   "hostProcess": true
				   }
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx:1.2.3",
				   "securityContext": {
						"windowsOptions": {
							"hostProcess": true
						}
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx:1.2.3",
				   "securityContext": {
						"windowsOptions": {
							"hostProcess": true
						}
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_hostProcesses_missing_restrictedField(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "HostProcess",
								"restrictedField": "spec.containers[*].securityContext.windowsOptions.hostProcess",
								"images": [
									"nginx:1.2.3"
								],
								"values": [
									"true"
								]
							},
							{
								"controlName": "HostProcess",
								"restrictedField": "spec.initContainers[*].securityContext.windowsOptions.hostProcess",
								"images": [
									"nginx:1.2.3"
								],
								"values": [
									"true"
								]
							},
							{
								"controlName": "HostProcess",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.windowsOptions.hostProcess",
								"images": [
									"nginx:1.2.3"
								],
								"values": [
									"true"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"windowsOptions": {
					"hostProcess": true
				}
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx:1.2.3",
				 "securityContext": {
					"windowsOptions": {
						"hostProcess": true
					}
				 }
			},
			{
				"name": "nodejs",
				"image": "nodejs:1.2.3",
				"securityContext": {
				   "windowsOptions": {
					   "hostProcess": true
				   }
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx:1.2.3",
				   "securityContext": {
						"windowsOptions": {
							"hostProcess": true
						}
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx:1.2.3",
				   "securityContext": {
						"windowsOptions": {
							"hostProcess": true
						}
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

// === Control: "Host Namespaces", check.ID: "hostNamespaces"

// pod-level:
// - spec.securityContext.seLinuxOptions.type

// container-level:
// - spec.hostNetwork
// - spec.hostPID
// - spec.hostIPC
func TestValidate_pod_security_admission_enforce_baseline_exclude_all_hostNamespaces(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostNamespaces"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostNamespaces",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Host Namespaces"
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": true,
		   "hostIPC":     true,
		   "hostPID":     true
		}
	 }
	 `)

	var policy kyverno.ClusterPolicy
	err := json.Unmarshal(rawPolicy, &policy)
	assert.NilError(t, err)

	resourceUnstructured, err := utils.ConvertToUnstructured(rawResource)
	assert.NilError(t, err)
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_hostNamespaces_with_restrictedFields_and_containers(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostNamespaces"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostNamespaces",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Host Namespaces",
								"restrictedField": "spec.hostNetwork",
								"values": [
									"true"
								]
							},
							{
								"controlName": "Host Namespaces",
								"restrictedField": "spec.hostIPC",
								"values": [
									"true"
								]
							},
							{
								"controlName": "Host Namespaces",
								"restrictedField": "spec.hostPID",
								"values": [
									"true"
								]
							},
							{
								"controlName": "HostProcess",
								"images": [
									"nginx:1.2.3",
									"nodejs:1.2.3"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": true,
		   "hostIPC":     true,
		   "hostPID":     true,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx:1.2.3",
				 "securityContext": {
					"windowsOptions": {
						"hostProcess": true
					}
				 }
			},
			{
				"name": "nodejs",
				"image": "nodejs:1.2.3",
				"securityContext": {
				   "windowsOptions": {
					   "hostProcess": true
				   }
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx:1.2.3",
				   "securityContext": {
						"windowsOptions": {
							"hostProcess": true
						}
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx:1.2.3",
				   "securityContext": {
						"windowsOptions": {
							"hostProcess": true
						}
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_hostNamespaces_with_restrictedFields_and_forbidden_containers(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostNamespaces"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostNamespaces",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Host Namespaces",
								"restrictedField": "spec.hostNetwork",
								"values": [
									"true"
								]
							},
							{
								"controlName": "Host Namespaces",
								"restrictedField": "spec.hostIPC",
								"values": [
									"true"
								]
							},
							{
								"controlName": "Host Namespaces",
								"restrictedField": "spec.hostPID",
								"values": [
									"true"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": true,
		   "hostIPC":     true,
		   "hostPID":     true,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx:1.2.3",
				 "securityContext": {
					"windowsOptions": {
						"hostProcess": true
					}
				 }
			},
			{
				"name": "nodejs",
				"image": "nodejs:1.2.3",
				"securityContext": {
				   "windowsOptions": {
					   "hostProcess": true
				   }
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx:1.2.3",
				   "securityContext": {
						"windowsOptions": {
							"hostProcess": true
						}
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx:1.2.3",
				   "securityContext": {
						"windowsOptions": {
							"hostProcess": true
						}
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_hostNamespaces_missing_exclude_value(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostNamespaces"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostNamespaces",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Host Namespaces",
								"restrictedField": "spec.hostNetwork",
								"values": [
									"true"
								]
							},
							{
								"controlName": "Host Namespaces",
								"restrictedField": "spec.hostIPC",
								"values": [
									"true"
								]
							},
							{
								"controlName": "Host Namespaces",
								"restrictedField": "spec.hostPID",
								"values": [
									"randomValue"
								]
							},
							{
								"controlName": "HostProcess",
								"images": [
									"nginx:1.2.3",
									"nodejs:1.2.3"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": true,
		   "hostIPC":     true,
		   "hostPID":     true,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx:1.2.3",
				 "securityContext": {
					"windowsOptions": {
						"hostProcess": true
					}
				 }
			},
			{
				"name": "nodejs",
				"image": "nodejs:1.2.3",
				"securityContext": {
				   "windowsOptions": {
					   "hostProcess": true
				   }
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx:1.2.3",
				   "securityContext": {
						"windowsOptions": {
							"hostProcess": true
						}
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx:1.2.3",
				   "securityContext": {
						"windowsOptions": {
							"hostProcess": true
						}
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_hostNamespaces_some_pod_level(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostNamespaces"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostNamespaces",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Host Namespaces",
								"restrictedField": "spec.hostNetwork",
								"values": [
									"true"
								]
							},
							{
								"controlName": "Host Namespaces",
								"restrictedField": "spec.hostIPC",
								"values": [
									"true"
								]
							},
							{
								"controlName": "HostProcess",
								"images": [
									"nginx:1.2.3",
									"nodejs:1.2.3"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": true,
		   "hostIPC":     true,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx:1.2.3",
				 "securityContext": {
					"windowsOptions": {
						"hostProcess": true
					}
				 }
			},
			{
				"name": "nodejs",
				"image": "nodejs:1.2.3",
				"securityContext": {
				   "windowsOptions": {
					   "hostProcess": true
				   }
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx:1.2.3",
				   "securityContext": {
						"windowsOptions": {
							"hostProcess": true
						}
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx:1.2.3",
				   "securityContext": {
						"windowsOptions": {
							"hostProcess": true
						}
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_hostNamespaces_missing_restrictedField(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostNamespaces"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostNamespaces",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Host Namespaces",
								"restrictedField": "spec.hostNetwork",
								"values": [
									"true"
								]
							},
							{
								"controlName": "Host Namespaces",
								"restrictedField": "spec.hostIPC",
								"values": [
									"true"
								]
							},
							{
								"controlName": "HostProcess",
								"images": [
									"nginx:1.2.3",
									"nodejs:1.2.3"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": true,
		   "hostIPC":     true,
		   "hostPID":     true,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx:1.2.3",
				 "securityContext": {
					"windowsOptions": {
						"hostProcess": true
					}
				 }
			},
			{
				"name": "nodejs",
				"image": "nodejs:1.2.3",
				"securityContext": {
				   "windowsOptions": {
					   "hostProcess": true
				   }
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx:1.2.3",
				   "securityContext": {
						"windowsOptions": {
							"hostProcess": true
						}
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx:1.2.3",
				   "securityContext": {
						"windowsOptions": {
							"hostProcess": true
						}
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

// === Control: "Capabilities", check.ID: "capabilities_baseline"
// pod-level restrictedFields:
// - spec.containers[*].securityContext.capabilities.add
// - spec.initContainers[*].securityContext.capabilities.add
// - spec.ephemeralContainers[*].securityContext.capabilities.add

// Only ControlName: exclude all restrictedFields for `Capabilities` control for all containers (containers, initContainers, ephemeralContainers) running with images `nginx`
// 1 * Container: nginx
// 1 * InitContainer: nginx
// 1 * EphemeralContainer: nginx
// Pod creation allowed
func TestValidate_pod_security_admission_enforce_baseline_exclude_all_capabilities(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-capabilities-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-capabilities-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"capabilities": { 
						"add": [
							"SYS_ADMIN"
						]
					}
				 }
			  }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
					  "capabilities": { 
						  "add": [
							  "SYS_ADMIN"
						  ]
					  }
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
					  "capabilities": { 
						  "add": [
							  "SYS_ADMIN"
						  ]
					  }
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

// Exclude `SYS_ADMIN` and `SYS_TIME` values for `Capabilites` control for all containers (containers, initContainers, ephemeralContainers) running with images `nginx`
// 1 * Container: nginx
// 1 * InitContainer: nginx
// 1 * EphemeralContainer: nginx
// Pod creation allowed
func TestValidate_pod_security_admission_enforce_baseline_exclude_capabilities_with_restrictedFields(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-some-capabilities-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-some-capabilities-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Capabilities",
								"restrictedField": "spec.containers[*].securityContext.capabilities.add",
								"images": [
									"nginx"
								],
								"values": [
									"SYS_ADMIN",
									"SYS_TIME"
								]
							},
							{
								"controlName": "Capabilities",
								"restrictedField": "spec.initContainers[*].securityContext.capabilities.add",
								"images": [
									"nginx"
								],
								"values": [
									"SYS_ADMIN",
									"SYS_TIME"
								]
							},
							{
								"controlName": "Capabilities",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.capabilities.add",
								"images": [
									"nginx"
								],
								"values": [
									"SYS_ADMIN",
									"SYS_TIME"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			  {
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"capabilities": { 
						"add": [
							"SYS_ADMIN",
							"SYS_TIME"
						]
					}
				 }
			  }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
					  "capabilities": { 
						  "add": [
							  "SYS_ADMIN",
							  "SYS_TIME"
						  ]
					  }
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
					  "capabilities": { 
						  "add": [
							  "SYS_ADMIN",
							  "SYS_TIME"
						  ]
					  }
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

// Exclude `SYS_ADMIN` value for `Capabilites` control for all containers (containers, initContainers, ephemeralContainers) running with images `nginx`
// 1 * Container: nginx
// 1 * InitContainer: nginx
// 1 * EphemeralContainer: nginx
// Pod creation forbidden: missing `SYS_TIME` value in exclude
func TestValidate_pod_security_admission_enforce_restricted_exclude_capabilities_missing_exclude_value(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name":  "enforce-baseline-exclude-some-capabilities-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-some-capabilities-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Capabilities",
								"restrictedField": "spec.containers[*].securityContext.capabilities.add",
								"images": [
									"nginx"
								],
								"values": [
									"SYS_ADMIN"
								]
							},
							{
								"controlName": "Capabilities",
								"restrictedField": "spec.initContainers[*].securityContext.capabilities.add",
								"images": [
									"nginx"
								],
								"values": [
									"SYS_ADMIN"
								]
							},
							{
								"controlName": "Capabilities",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.capabilities.add",
								"images": [
									"nginx"
								],
								"values": [
									"SYS_ADMIN"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			  {
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"capabilities": { 
						"add": [
							"SYS_ADMIN",
							"SYS_TIME"
						]
					}
				 }
			  }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
					  "capabilities": { 
						  "add": [
							  "SYS_ADMIN",
							  "SYS_TIME"
						  ]
					  }
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
					  "capabilities": { 
						  "add": [
							  "SYS_ADMIN",
							  "SYS_TIME"
						  ]
					  }
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

// Exclude `SYS_ADMIN`, `SYS_TIME` values for `Capabilites` control for all containers (containers, initContainers, ephemeralContainers) running with images `nginx`
// 1 * Container: nginx
// 1 * InitContainer: nginx
// 1 * EphemeralContainer: nginx
// Pod creation forbidden: missing exclude block for ephemeralContainers:
//
//	{
//		"controlName": "Capabilities",
//		"restrictedField": "spec.ephemeralContainers[*].securityContext.capabilities.add",
//		"images": [
//			"nginx"
//		],
//		"values": [
//			"SYS_ADMIN",
//			"SYS_TIME"
//		]
//	}
func TestValidate_pod_security_admission_enforce_restricted_exclude_capabilities_missing_exclude_restrictedField(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name":  "enforce-baseline-exclude-some-capabilities-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-some-capabilities-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Capabilities",
								"restrictedField": "spec.containers[*].securityContext.capabilities.add",
								"images": [
									"nginx"
								],
								"values": [
									"SYS_ADMIN",
									"SYS_TIME"
								]
							},
							{
								"controlName": "Capabilities",
								"restrictedField": "spec.initContainers[*].securityContext.capabilities.add",
								"images": [
									"nginx"
								],
								"values": [
									"SYS_ADMIN",
									"SYS_TIME"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			  {
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"capabilities": { 
						"add": [
							"SYS_ADMIN",
							"SYS_TIME"
						]
					}
				 }
			  }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
					  "capabilities": { 
						  "add": [
							  "SYS_ADMIN",
							  "SYS_TIME"
						  ]
					  }
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
					  "capabilities": { 
						  "add": [
							  "SYS_ADMIN",
							  "SYS_TIME"
						  ]
					  }
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

// === Control: "Privileged Containers", check.ID: "privileged"

// container-level:
// - spec.containers[*].securityContext.securityContext.privileged
// - spec.initContainers[*].securityContext.securityContext.privileged
// - spec.ephemeralContainers[*].securityContext.securityContext.privileged

// Only ControlName: exclude all restrictedFields for `Privileged Containers` control running with images `nginx` and `nodejs`
// 2 * Container: nginx / nodejs
// 1 * InitContainer: nginx
// 1 * EphemeralContainer: nginx
// Pod creation allowed
func TestValidate_pod_security_admission_enforce_baseline_exclude_all_privileged_containers(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-privileged-containers-nginx-nodejs"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-privileged-containers-nginx-nodejs",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Privileged Containers",
								"images": [
									"nginx",
									"nodejs"
								]
							}
						]
					}
				 }
			  }
		   ]
		}
	 }
	 `)

	// Restricted
	rawResource := []byte(`
	{
	   "apiVersion": "v1",
	   "kind": "Pod",
	   "metadata": {
		  "name": "nginx-baseline-privileged-container",
		  "namespace": "staging"
	   },
	   "spec": {
		  "hostNetwork": false,
		  "containers": [
		   {
			   "name": "nginx",
			   "image": "nginx",
			   "securityContext": {
					"privileged": true,
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false,
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					}
				}
			}
			],
			"initContainers": [
				{
					"name": "init-nginx",
					"image": "nginx",
					"securityContext": {
						 "privileged": true,
						 "runAsNonRoot": true,
						 "allowPrivilegeEscalation": false,
						 "seccompProfile": {
							 "type": "RuntimeDefault"
						 },
						 "capabilities": {
							 "drop": [
								 "ALL"
							 ]
						 }
					 }
				 }
			],
			"ephemeralContainers": [
				{
					"name": "ephemeral-nginx",
					"image": "nginx",
					"securityContext": {
						 "privileged": true,
						 "runAsNonRoot": true,
						 "allowPrivilegeEscalation": false,
						 "seccompProfile": {
							 "type": "RuntimeDefault"
						 },
						 "capabilities": {
							 "drop": [
								 "ALL"
							 ]
						 }
					 }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_privileged_containers_with_restrictedFields(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-some-privileged-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-some-privileged-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Privileged Containers",
								"restrictedField": "spec.containers[*].securityContext.privileged",
								"values": [
									"true"
								],
								"images": [
									"nginx",
									"nodejs"
								]
							},
							{
								"controlName": "Privileged Containers",
								"restrictedField": "spec.initContainers[*].securityContext.privileged",
								"values": [
									"true"
								],
								"images": [
									"nginx"
								]
							},
							{
								"controlName": "Privileged Containers",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.privileged",
								"values": [
									"true"
								],
								"images": [
									"nginx"
								]
							}
						]
					}
				 }
			  }
		   ]
		}
	 }
	 `)

	// Restricted
	rawResource := []byte(`
	{
	   "apiVersion": "v1",
	   "kind": "Pod",
	   "metadata": {
		  "name": "nginx-baseline-privileged-container",
		  "namespace": "staging"
	   },
	   "spec": {
		  "hostNetwork": false,
		  "containers": [
		   {
			   "name": "nginx",
			   "image": "nginx",
			   "securityContext": {
					"privileged": true,
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false,
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					}
				}
			},
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					 "privileged": true,
					 "runAsNonRoot": true,
					 "allowPrivilegeEscalation": false,
					 "seccompProfile": {
						 "type": "RuntimeDefault"
					 },
					 "capabilities": {
						 "drop": [
							 "ALL"
						 ]
					 }
				 }
			 }
			],
			"initContainers": [
				{
					"name": "init-nginx",
					"image": "nginx",
					"securityContext": {
						 "privileged": true,
						 "runAsNonRoot": true,
						 "allowPrivilegeEscalation": false,
						 "seccompProfile": {
							 "type": "RuntimeDefault"
						 },
						 "capabilities": {
							 "drop": [
								 "ALL"
							 ]
						 }
					 }
				 }
			],
			"ephemeralContainers": [
				{
					"name": "ephemeral-nginx",
					"image": "nginx",
					"securityContext": {
						 "privileged": true,
						 "runAsNonRoot": true,
						 "allowPrivilegeEscalation": false,
						 "seccompProfile": {
							 "type": "RuntimeDefault"
						 },
						 "capabilities": {
							 "drop": [
								 "ALL"
							 ]
						 }
					 }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
	}
	assert.Assert(t, er.IsSuccessful())
}

// Only ControlName: exclude all restrictedFields for `privileged containers` control running with images `nginx`
// 2 * Container: nginx / nodejs
// 1 * InitContainer: nginx
// 1 * EphemeralContainer: nginx
// Pod creation forbidden: missing exclude for container running with `nodejs` image
func TestValidate_pod_security_admission_enforce_baseline_exclude_privileged_containers_missing_exclude_value(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-some-privileged-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-some-privileged-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Privileged Containers",
								"restrictedField": "spec.containers[*].securityContext.privileged",
								"values": [
									"ForbiddenValue"
								],
								"images": [
									"nginx",
									"nodejs"
								]
							},
							{
								"controlName": "Privileged Containers",
								"restrictedField": "spec.initContainers[*].securityContext.privileged",
								"values": [
									"true"
								],
								"images": [
									"nginx"
								]
							},
							{
								"controlName": "Privileged Containers",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.privileged",
								"values": [
									"true"
								],
								"images": [
									"nginx"
								]
							}
						]
					}
				 }
			  }
		   ]
		}
	 }
	 `)

	// Restricted
	rawResource := []byte(`
	{
	   "apiVersion": "v1",
	   "kind": "Pod",
	   "metadata": {
		  "name": "nginx-baseline-privileged-container",
		  "namespace": "staging"
	   },
	   "spec": {
		  "hostNetwork": false,
		  "containers": [
		   {
			   "name": "nginx",
			   "image": "nginx",
			   "securityContext": {
					"privileged": true,
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false,
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					}
				}
			},
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					 "privileged": true,
					 "runAsNonRoot": true,
					 "allowPrivilegeEscalation": false,
					 "seccompProfile": {
						 "type": "RuntimeDefault"
					 },
					 "capabilities": {
						 "drop": [
							 "ALL"
						 ]
					 }
				 }
			 }
			],
			"initContainers": [
				{
					"name": "init-nginx",
					"image": "nginx",
					"securityContext": {
						 "privileged": true,
						 "runAsNonRoot": true,
						 "allowPrivilegeEscalation": false,
						 "seccompProfile": {
							 "type": "RuntimeDefault"
						 },
						 "capabilities": {
							 "drop": [
								 "ALL"
							 ]
						 }
					 }
				 }
			],
			"ephemeralContainers": [
				{
					"name": "ephemeral-nginx",
					"image": "nginx",
					"securityContext": {
						 "privileged": true,
						 "runAsNonRoot": true,
						 "allowPrivilegeEscalation": false,
						 "seccompProfile": {
							 "type": "RuntimeDefault"
						 },
						 "capabilities": {
							 "drop": [
								 "ALL"
							 ]
						 }
					 }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
	}
	assert.Assert(t, er.IsFailed())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_privileged_containers_missing_restrictedField(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-some-privileged-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-some-privileged-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Privileged Containers",
								"restrictedField": "spec.containers[*].securityContext.privileged",
								"values": [
									"true"
								],
								"images": [
									"nginx",
									"nodejs"
								]
							},
							{
								"controlName": "Privileged Containers",
								"restrictedField": "spec.initContainers[*].securityContext.privileged",
								"values": [
									"true"
								],
								"images": [
									"nginx"
								]
							}
						]
					}
				 }
			  }
		   ]
		}
	 }
	 `)

	// Restricted
	rawResource := []byte(`
	{
	   "apiVersion": "v1",
	   "kind": "Pod",
	   "metadata": {
		  "name": "nginx-baseline-privileged-container",
		  "namespace": "staging"
	   },
	   "spec": {
		  "hostNetwork": false,
		  "containers": [
		   {
			   "name": "nginx",
			   "image": "nginx",
			   "securityContext": {
					"privileged": true,
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false,
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					}
				}
			},
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					 "privileged": true,
					 "runAsNonRoot": true,
					 "allowPrivilegeEscalation": false,
					 "seccompProfile": {
						 "type": "RuntimeDefault"
					 },
					 "capabilities": {
						 "drop": [
							 "ALL"
						 ]
					 }
				 }
			 }
			],
			"initContainers": [
				{
					"name": "init-nginx",
					"image": "nginx",
					"securityContext": {
						 "privileged": true,
						 "runAsNonRoot": true,
						 "allowPrivilegeEscalation": false,
						 "seccompProfile": {
							 "type": "RuntimeDefault"
						 },
						 "capabilities": {
							 "drop": [
								 "ALL"
							 ]
						 }
					 }
				 }
			],
			"ephemeralContainers": [
				{
					"name": "ephemeral-nginx",
					"image": "nginx",
					"securityContext": {
						 "privileged": true,
						 "runAsNonRoot": true,
						 "allowPrivilegeEscalation": false,
						 "seccompProfile": {
							 "type": "RuntimeDefault"
						 },
						 "capabilities": {
							 "drop": [
								 "ALL"
							 ]
						 }
					 }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
	}
	assert.Assert(t, er.IsFailed())
}

// Exclude spec.containers[*].securityContext.privileged for containers running `nginx` image
// 2 * Container: nginx / nodejs
// 1 * InitContainer: nginx
// 1 * EphemeralContainer: nginx
// Fail -> We have to exclude restrictedFields for initContainers and ephemeralContainers:
// - spec.initContainers[*].securityContext.privileged
// - spec.ephemeralContainers[*].securityContext.privileged

func TestValidate_pod_security_admission_enforce_restricted_exclude_privileged_containers_missing_restrictedFields(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-restricted-exclude-privileged-container-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-restricted-exclude-privileged-container-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "privileged",
								"restrictedField": "spec.containers[*].securityContext.privileged",
								"values": [
									"true"
								],
								"images": [
									"nginx"
								]
							}
						]
					}
				 }
			  }
		   ]
		}
	 }
	 `)

	// Restricted
	rawResource := []byte(`
	{
	   "apiVersion": "v1",
	   "kind": "Pod",
	   "metadata": {
		  "name": "nginx-baseline-privileged-container",
		  "namespace": "staging"
	   },
	   "spec": {
		  "hostNetwork": false,
		  "containers": [
		   {
			   "name": "nginx",
			   "image": "nginx",
			   "securityContext": {
					"privileged": true,
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false,
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					}
				}
			},
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					 "privileged": true,
					 "runAsNonRoot": true,
					 "allowPrivilegeEscalation": false,
					 "seccompProfile": {
						 "type": "RuntimeDefault"
					 },
					 "capabilities": {
						 "drop": [
							 "ALL"
						 ]
					 }
				 }
			 }
			],
			"initContainers": [
				{
					"name": "init-nginx",
					"image": "nginx",
					"securityContext": {
						 "privileged": true,
						 "runAsNonRoot": true,
						 "allowPrivilegeEscalation": false,
						 "seccompProfile": {
							 "type": "RuntimeDefault"
						 },
						 "capabilities": {
							 "drop": [
								 "ALL"
							 ]
						 }
					 }
				 }
			],
			"ephemeralContainers": [
				{
					"name": "ephemeral-nginx",
					"image": "nginx",
					"securityContext": {
						 "privileged": true,
						 "runAsNonRoot": true,
						 "allowPrivilegeEscalation": false,
						 "seccompProfile": {
							 "type": "RuntimeDefault"
						 },
						 "capabilities": {
							 "drop": [
								 "ALL"
							 ]
						 }
					 }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
	}
	assert.Assert(t, er.IsFailed())
}

// Exclude spec.containers[*].securityContext.privileged for containers running `nginx` image
// Exclude spec.initContainers[*].securityContext.privileged for initContainers running `nginx` image
// Exclude spec.ephemeralContainers[*].securityContext.privileged for ephemeralContainers running `nginx` image
// 2 * Container: nginx / nodejs
// 1 * InitContainer: nginx
// 1 * EphemeralContainer: nginx
// Fail -> We have to exclude restrictedFields for containers running with `nodejs` image:
// - spec.containers[*].securityContext.privileged
func TestValidate_pod_security_admission_enforce_restricted_exclude_privileged_containers(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-restricted-exclude-privileged-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-restricted-exclude-privileged-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "privileged",
								"restrictedField": "spec.containers[*].securityContext.privileged",
								"values": [
									"true"
								],
								"images": [
									"nginx"
								]
							},
							{
								"controlName": "privileged",
								"restrictedField": "spec.initContainers[*].securityContext.privileged",
								"values": [
									"true"
								],
								"images": [
									"nginx"
								]
							},
							{
								"controlName": "privileged",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.privileged",
								"values": [
									"true"
								],
								"images": [
									"nginx"
								]
							}
						]
					}
				 }
			  }
		   ]
		}
	 }
	 `)

	// Restricted
	rawResource := []byte(`
	{
	   "apiVersion": "v1",
	   "kind": "Pod",
	   "metadata": {
		  "name": "nginx-baseline-privileged-container",
		  "namespace": "staging"
	   },
	   "spec": {
		  "hostNetwork": false,
		  "containers": [
		   {
			   "name": "nginx",
			   "image": "nginx",
			   "securityContext": {
					"privileged": true,
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false,
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					}
				}
			},
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					 "privileged": true,
					 "runAsNonRoot": true,
					 "allowPrivilegeEscalation": false,
					 "seccompProfile": {
						 "type": "RuntimeDefault"
					 },
					 "capabilities": {
						 "drop": [
							 "ALL"
						 ]
					 }
				 }
			 }
			],
			"initContainers": [
				{
					"name": "init-nginx",
					"image": "nginx",
					"securityContext": {
						 "privileged": true,
						 "runAsNonRoot": true,
						 "allowPrivilegeEscalation": false,
						 "seccompProfile": {
							 "type": "RuntimeDefault"
						 },
						 "capabilities": {
							 "drop": [
								 "ALL"
							 ]
						 }
					 }
				 }
			],
			"ephemeralContainers": [
				{
					"name": "ephemeral-nginx",
					"image": "nginx",
					"securityContext": {
						 "privileged": true,
						 "runAsNonRoot": true,
						 "allowPrivilegeEscalation": false,
						 "seccompProfile": {
							 "type": "RuntimeDefault"
						 },
						 "capabilities": {
							 "drop": [
								 "ALL"
							 ]
						 }
					 }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
	}
	assert.Assert(t, er.IsFailed())
}

// === Control: "Host Ports", check.ID: "hostPorts"

// container-level:
// - spec.containers[*].securityContext.ports[*].hostPort
// - spec.initContainers[*].securityContext.ports[*].hostPort
// - spec.ephemeralContainers[*].securityContext.ports[*].hostPort

func TestValidate_pod_security_admission_enforce_baseline_exclude_all_hostPorts(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostPorts-all-containers-nginx-nodejs"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostPorts-all-containers-nginx-nodejs",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Host Ports",
								"images": [
									"nginx",
									"nodejs"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "ports": [
					{
						"hostPort": 8080
					},
					{
						"hostPort": 9000
					}
				 ]
			  },
			  {
				"name": "nodejs",
				"image": "nodejs",
				"ports": [
					{
						"hostPort": 8080
					},
					{
						"hostPort": 9000
					}
				]
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "ports": [
						{
							"hostPort": 8080
						},
						{
							"hostPort": 9000
						}
					]
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "ports": [
						{
							"hostPort": 8080
						},
						{
							"hostPort": 9000
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_hostPorts_with_restrictedFields(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostPorts-all-containers-nginx-nodejs"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostPorts-all-containers-nginx-nodejs",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Host Ports",
								"restrictedField": "spec.containers[*].ports[*].hostPort",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"8080",
									"9000"
								]
							},
							{
								"controlName": "Host Ports",
								"restrictedField": "spec.initContainers[*].ports[*].hostPort",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"8080",
									"9000"
								]
							},
							{
								"controlName": "Host Ports",
								"restrictedField": "spec.ephemeralContainers[*].ports[*].hostPort",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"8080",
									"9000"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "ports": [
					{
						"hostPort": 8080
					},
					{
						"hostPort": 9000
					}
				 ]
			  },
			  {
				"name": "nodejs",
				"image": "nodejs",
				"ports": [
					{
						"hostPort": 8080
					},
					{
						"hostPort": 9000
					}
				]
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "ports": [
						{
							"hostPort": 8080
						},
						{
							"hostPort": 9000
						}
					]
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "ports": [
						{
							"hostPort": 8080
						},
						{
							"hostPort": 9000
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_hostPorts_missing_exclude_value(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostPorts-all-containers-nginx-nodejs"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostPorts-all-containers-nginx-nodejs",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Host Ports",
								"restrictedField": "spec.containers[*].ports[*].hostPort",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"8080"
								]
							},
							{
								"controlName": "Host Ports",
								"restrictedField": "spec.initContainers[*].ports[*].hostPort",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"8080",
									"9000"
								]
							},
							{
								"controlName": "Host Ports",
								"restrictedField": "spec.ephemeralContainers[*].ports[*].hostPort",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"8080",
									"9000"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "ports": [
					{
						"hostPort": 8080
					},
					{
						"hostPort": 9000
					}
				 ]
			  },
			  {
				"name": "nodejs",
				"image": "nodejs",
				"ports": [
					{
						"hostPort": 8080
					},
					{
						"hostPort": 9000
					}
				]
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "ports": [
						{
							"hostPort": 8080
						},
						{
							"hostPort": 9000
						}
					]
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "ports": [
						{
							"hostPort": 8080
						},
						{
							"hostPort": 9000
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

func TestValidate_pod_security_admission_enforce_restricted_exclude_hostPorts_missing_exclude_restrictedField(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostPorts-all-containers-nginx-nodejs"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostPorts-all-containers-nginx-nodejs",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Host Ports",
								"restrictedField": "spec.containers[*].ports[*].hostPort",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"8080",
									"9000"
								]
							},
							{
								"controlName": "Host Ports",
								"restrictedField": "spec.initContainers[*].ports[*].hostPort",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"8080",
									"9000"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "ports": [
					{
						"hostPort": 8080
					},
					{
						"hostPort": 9000
					}
				 ]
			  },
			  {
				"name": "nodejs",
				"image": "nodejs",
				"ports": [
					{
						"hostPort": 8080
					},
					{
						"hostPort": 9000
					}
				]
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "ports": [
						{
							"hostPort": 8080
						},
						{
							"hostPort": 9000
						}
					]
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "ports": [
						{
							"hostPort": 8080
						},
						{
							"hostPort": 9000
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

// === Control: "/proc Mount Type", check.ID: "procMount"

// container-level:
// - spec.containers[*].securityContext.procMount
// - spec.initContainers[*].securityContext.procMount
// - spec.ephemeralContainers[*].securityContext.procMount

func TestValidate_pod_security_admission_enforce_baseline_exclude_all_procMounts(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-procMounts-all-containers-nginx-nodejs"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-procMounts-all-containers-nginx-nodejs",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "/proc Mount Type",
								"images": [
									"nginx",
									"nodejs"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"ProcMount": "Unmasked" 
				 }
			  },
			  {
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"ProcMount": "Other" 
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"ProcMount": "Unmasked" 
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"ProcMount": "Unmasked" 
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_procMounts_with_restrictedFields(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-procMounts-all-containers-nginx-nodejs"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-procMounts-all-containers-nginx-nodejs",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "/proc Mount Type",
								"restrictedField": "spec.containers[*].securityContext.procMount",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"Unmasked",
									"Other"
								]
							},
							{
								"controlName": "/proc Mount Type",
								"restrictedField": "spec.initContainers[*].securityContext.procMount",
								"images": [
									"nginx"
								],
								"values": [
									"Unmasked"
								]
							},
							{
								"controlName": "/proc Mount Type",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.procMount",
								"images": [
									"nginx"
								],
								"values": [
									"Unmasked"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"ProcMount": "Unmasked" 
				 }
			  },
			  {
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"ProcMount": "Other" 
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"ProcMount": "Unmasked" 
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"ProcMount": "Unmasked" 
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_procMounts_missing_exclude_value(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-procMounts-all-containers-nginx-nodejs"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-procMounts-all-containers-nginx-nodejs",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "/proc Mount Type",
								"restrictedField": "spec.containers[*].securityContext.procMount",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"Unmasked"
								]
							},
							{
								"controlName": "/proc Mount Type",
								"restrictedField": "spec.initContainers[*].securityContext.procMount",
								"images": [
									"nginx"
								],
								"values": [
									"Unmasked"
								]
							},
							{
								"controlName": "/proc Mount Type",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.procMount",
								"images": [
									"nginx"
								],
								"values": [
									"Unmasked"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"ProcMount": "Unmasked" 
				 }
			  },
			  {
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"ProcMount": "Other" 
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"ProcMount": "Unmasked" 
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"ProcMount": "Unmasked" 
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

func TestValidate_pod_security_admission_enforce_baseline_exclude_procMounts_missing_exclude_RestrictedField(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-procMounts-all-containers-nginx-nodejs"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-procMounts-all-containers-nginx-nodejs",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "baseline",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "/proc Mount Type",
								"restrictedField": "spec.containers[*].securityContext.procMount",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"Unmasked",
									"Other"
								]
							},
							{
								"controlName": "/proc Mount Type",
								"restrictedField": "spec.initContainers[*].securityContext.procMount",
								"images": [
									"nginx"
								],
								"values": [
									"Unmasked"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"ProcMount": "Unmasked" 
				 }
			  },
			  {
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"ProcMount": "Other" 
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"ProcMount": "Unmasked" 
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"ProcMount": "Unmasked" 
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

// ====== Restricted ======

// === Control: "Volumes Types", check.ID: "restrictedVolumes"

// pod-level:
// - spec.volumes[*]

func TestValidate_pod_security_admission_enforce_restricted_exclude_all_volume_types(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Volume Types"
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
			"volumes": [
				{
					"name": "aws-volume",
					"AWSElasticBlockStore": {
						"volumeID": "id",
						"fsType": "ext4"
					}
				},
				{
					"name": "gcp-volume",
					"GCEPersistentDisk": {
						"pdName": "my-data-disk",
						"fsType": "ext4"
					}
				}
			],
		   "hostNetwork": false,
		   "securityContext": {
				"seccompProfile": {
					"type": "RuntimeDefault"
				},
				"runAsNonRoot": true,
				"allowPrivilegeEscalation": false
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_restricted_exclude_all_volume_types_with_restrictedFields(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Volume Types",
								"restrictedField": "spec.volumes[*]",
								"values": [
									"spec.volumes[*].awsElasticBlockStore",
									"spec.volumes[*].gcePersistentDisk"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
			"volumes": [
				{
					"name": "aws-volume",
					"AWSElasticBlockStore": {
						"volumeID": "id",
						"fsType": "ext4"
					}
				},
				{
					"name": "gcp-volume",
					"GCEPersistentDisk": {
						"pdName": "my-data-disk",
						"fsType": "ext4"
					}
				}
			],
		   "hostNetwork": false,
		   "securityContext": {
				"seccompProfile": {
					"type": "RuntimeDefault"
				},
				"runAsNonRoot": true,
				"allowPrivilegeEscalation": false
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_restricted_exclude_all_volume_types_missing_exclude_value(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Volume Types",
								"restrictedField": "spec.volumes[*]",
								"values": [
									"spec.volumes[*].awsElasticBlockStore",
									"spec.volumes[*].cephfs"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
			"volumes": [
				{
					"name": "aws-volume",
					"AWSElasticBlockStore": {
						"volumeID": "id",
						"fsType": "ext4"
					}
				},
				{
					"name": "gcp-volume",
					"GCEPersistentDisk": {
						"pdName": "my-data-disk",
						"fsType": "ext4"
					}
				}
			],
		   "hostNetwork": false,
		   "securityContext": {
				"seccompProfile": {
					"type": "RuntimeDefault"
				},
				"runAsNonRoot": true,
				"allowPrivilegeEscalation": false
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

func TestValidate_pod_security_admission_enforce_restricted_exclude_all_volume_types_missing_restrictedField(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Privilege Containers",
								"images": [
									"nginx"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
			"volumes": [
				{
					"name": "aws-volume",
					"AWSElasticBlockStore": {
						"volumeID": "id",
						"fsType": "ext4"
					}
				},
				{
					"name": "gcp-volume",
					"GCEPersistentDisk": {
						"pdName": "my-data-disk",
						"fsType": "ext4"
					}
				}
			],
		   "hostNetwork": false,
		   "securityContext": {
				"seccompProfile": {
					"type": "RuntimeDefault"
				},
				"runAsNonRoot": true,
				"allowPrivilegeEscalation": false
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

// === Control: "Running as Non-root user", check.ID: "runAsUser"
// pod-level:
// - spec.securityContext.runAsUser

// container-level:
// - spec.containers[*].securityContext.runAsUser
// - spec.initContainers[*].securityContext.runAsUser
// - spec.ephemeralContainers[*].securityContext.runAsUser

func TestValidate_pod_security_admission_enforce_restricted_exclude_all_running_as_non_root_user(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Running as Non-root user",
								"images": [
									"nginx",
									"nodejs"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"seccompProfile": {
					"type": "RuntimeDefault"
				},
				"runAsNonRoot": true,
				"runAsUser": 1,
				"allowPrivilegeEscalation": false
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"runAsUser": 0,
					"allowPrivilegeEscalation": false
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"runAsUser": 0,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"runAsUser": 0,
						"allowPrivilegeEscalation": false
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"runAsUser": 0,
						"allowPrivilegeEscalation": false
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_restricted_exclude_all_running_as_non_root_user_with_restrictedField(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.23",
						"exclude": [
							{
								"controlName": "Running as Non-root user",
								"restrictedField": "spec.securityContext.runAsUser",
								"values": [
									"0"
								]
							},
							{
								"controlName": "Running as Non-root user",
								"restrictedField": "spec.containers[*].securityContext.runAsUser",
								"values": [
									"0"
								],
								"images": [
									"nginx",
									"nodejs"
								]
							},
							{
								"controlName": "Running as Non-root user",
								"restrictedField": "spec.initContainers[*].securityContext.runAsUser",
								"values": [
									"0"
								],
								"images": [
									"nginx"
								]
							},
							{
								"controlName": "Running as Non-root user",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.runAsUser",
								"values": [
									"0"
								],
								"images": [
									"nginx"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"seccompProfile": {
					"type": "RuntimeDefault"
				},
				"runAsNonRoot": true,
				"runAsUser": 0,
				"allowPrivilegeEscalation": false
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"runAsUser": 0,
					"allowPrivilegeEscalation": false
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"runAsUser": 0,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"runAsUser": 0,
						"allowPrivilegeEscalation": false
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"runAsUser": 0,
						"allowPrivilegeEscalation": false
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_restricted_exclude_all_running_as_non_root_user_missing_exclude_value(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.23",
						"exclude": [
							{
								"controlName": "Running as Non-root user",
								"restrictedField": "spec.securityContext.runAsUser",
								"values": [
									"0"
								]
							},
							{
								"controlName": "Running as Non-root user",
								"restrictedField": "spec.containers[*].securityContext.runAsUser",
								"values": [
									"1"
								],
								"images": [
									"nginx",
									"nodejs"
								]
							},
							{
								"controlName": "Running as Non-root user",
								"restrictedField": "spec.initContainers[*].securityContext.runAsUser",
								"values": [
									"0"
								],
								"images": [
									"nginx",
									"nodejs"
								]
							},
							{
								"controlName": "Running as Non-root user",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.runAsUser",
								"values": [
									"0"
								],
								"images": [
									"nginx"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"seccompProfile": {
					"type": "RuntimeDefault"
				},
				"runAsNonRoot": true,
				"runAsUser": 0,
				"allowPrivilegeEscalation": false
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"runAsUser": 0,
					"allowPrivilegeEscalation": false
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"runAsUser": 0,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"runAsUser": 0,
						"allowPrivilegeEscalation": false
					}
				},
				{
					"name": "init-nodejs",
					"image": "nodejs",
					"securityContext": {
						 "seccompProfile": {
							 "type": "RuntimeDefault"
						 },
						 "capabilities": {
							 "drop": [
								 "ALL"
							 ]
						 },
						 "runAsNonRoot": true,
						 "runAsUser": 1,
						 "allowPrivilegeEscalation": false
					 }
				 }
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"runAsUser": 0,
						"allowPrivilegeEscalation": false
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

func TestValidate_pod_security_admission_enforce_restricted_exclude_all_running_as_non_root_user_missing_restrictedField(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.23",
						"exclude": [
							{
								"controlName": "Running as Non-root user",
								"restrictedField": "spec.securityContext.runAsUser",
								"values": [
									"0"
								]
							},
							{
								"controlName": "Running as Non-root user",
								"restrictedField": "spec.initContainers[*].securityContext.runAsUser",
								"values": [
									"0"
								],
								"images": [
									"nginx"
								]
							},
							{
								"controlName": "Running as Non-root user",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.runAsUser",
								"values": [
									"0"
								],
								"images": [
									"nginx"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"seccompProfile": {
					"type": "RuntimeDefault"
				},
				"runAsNonRoot": true,
				"runAsUser": 0,
				"allowPrivilegeEscalation": false
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"runAsUser": 0,
					"allowPrivilegeEscalation": false
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"runAsUser": 0,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"runAsUser": 0,
						"allowPrivilegeEscalation": false
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"runAsUser": 0,
						"allowPrivilegeEscalation": false
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

// === Control: "Running as Non-root", check.ID: "runAsNonRoot"

// pod-level:
// - spec.securityContext.runAsNonRoot

// container-level:
// - spec.containers[*].securityContext.runAsNonRoot
// - spec.initContainers[*].securityContext.runAsNonRoot
// - spec.ephemeralContainers[*].securityContext.runAsNonRoot

func TestValidate_pod_security_admission_enforce_restricted_exclude_all_running_as_non_root(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Running as Non-root",
								"images": [
									"nginx",
									"nodejs"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"seccompProfile": {
					"type": "RuntimeDefault"
				},
				"runAsNonRoot": false,
				"allowPrivilegeEscalation": false
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": false,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": false,
						"allowPrivilegeEscalation": false
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": false,
						"allowPrivilegeEscalation": false
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_restricted_exclude_all_running_as_non_root_with_restrictedField(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Running as Non-root",
								"restrictedField": "spec.securityContext.runAsNonRoot",
								"values": [
									"false"
								]
							},
							{
								"controlName": "Running as Non-root",
								"restrictedField": "spec.containers[*].securityContext.runAsNonRoot",
								"values": [
									"false"
								],
								"images": [
									"nginx",
									"nodejs"
								]
							},
							{
								"controlName": "Running as Non-root",
								"restrictedField": "spec.initContainers[*].securityContext.runAsNonRoot",
								"values": [
									"false"
								],
								"images": [
									"nginx"
								]
							},
							{
								"controlName": "Running as Non-root",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.runAsNonRoot",
								"values": [
									"false"
								],
								"images": [
									"nginx"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"seccompProfile": {
					"type": "RuntimeDefault"
				},
				"runAsNonRoot": false,
				"allowPrivilegeEscalation": false
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": false,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": false,
						"allowPrivilegeEscalation": false
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": false,
						"allowPrivilegeEscalation": false
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_restricted_exclude_all_running_as_non_root_missing_exclude_value(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Running as Non-root",
								"restrictedField": "spec.securityContext.runAsNonRoot",
								"values": [
									"false"
								]
							},
							{
								"controlName": "Running as Non-root",
								"restrictedField": "spec.containers[*].securityContext.runAsNonRoot",
								"values": [
									"false"
								],
								"images": [
									"nginx"
								]
							},
							{
								"controlName": "Running as Non-root",
								"restrictedField": "spec.initContainers[*].securityContext.runAsNonRoot",
								"values": [
									"false"
								],
								"images": [
									"nginx"
								]
							},
							{
								"controlName": "Running as Non-root",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.runAsNonRoot",
								"values": [
									"false"
								],
								"images": [
									"nginx"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"seccompProfile": {
					"type": "RuntimeDefault"
				},
				"runAsNonRoot": false,
				"allowPrivilegeEscalation": false
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": false,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": false,
						"allowPrivilegeEscalation": false
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": false,
						"allowPrivilegeEscalation": false
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

func TestValidate_pod_security_admission_enforce_restricted_exclude_all_running_as_non_root_missing_restrictedField(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Running as Non-root",
								"restrictedField": "spec.securityContext.runAsNonRoot",
								"values": [
									"false"
								]
							},
							{
								"controlName": "Running as Non-root",
								"restrictedField": "spec.containers[*].securityContext.runAsNonRoot",
								"values": [
									"false"
								],
								"images": [
									"nodejs"
								]
							},
							{
								"controlName": "Running as Non-root",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.runAsNonRoot",
								"values": [
									"false"
								],
								"images": [
									"nginx"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"seccompProfile": {
					"type": "RuntimeDefault"
				},
				"runAsNonRoot": false,
				"allowPrivilegeEscalation": false
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": false,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": false,
						"allowPrivilegeEscalation": false
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": false,
						"allowPrivilegeEscalation": false
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

// === Control: "Seccomp", check.ID: "seccompProfile_restricted"
// pod-level:
// - spec.securityContext.seccompProfile.type

// container-level:
// - spec.containers[*].securityContext.seccompProfile.type
// - spec.initContainers[*].securityContext.seccompProfile.type
// - spec.ephemeralContainers[*].securityContext.seccompProfile.type

func TestValidate_pod_security_admission_enforce_restricted_exclude_all_seccomp(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Seccomp",
								"images": [
									"nginx",
									"nodejs"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"runAsNonRoot": true,
				"allowPrivilegeEscalation": false
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "Unconfined"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "Unconfined"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "Unconfined"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_restricted_exclude_seccomp_with_exclude(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Seccomp",
								"restrictedField": "spec.securityContext.seccompProfile.type",
								"values": [
									"Unconfined"
								]
							},
							{
								"controlName": "Seccomp",
								"restrictedField": "spec.containers[*].securityContext.seccompProfile.type",
								"values": [
									"Unconfined"
								],
								"images": [
									"nginx",
									"nodejs"
								]
							},
							{
								"controlName": "Seccomp",
								"restrictedField": "spec.initContainers[*].securityContext.seccompProfile.type",
								"values": [
									"Unconfined"
								],
								"images": [
									"nginx"
								]
							},
							{
								"controlName": "Seccomp",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.seccompProfile.type",
								"values": [
									"Unconfined"
								],
								"images": [
									"nginx"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"seccompProfile": {
					"type": "Unconfined"
				},
				"runAsNonRoot": true,
				"allowPrivilegeEscalation": false
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "Unconfined"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "Unconfined"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "Unconfined"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "Unconfined"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_restricted_exclude_seccomp_missing_exclude_value(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Seccomp",
								"restrictedField": "spec.securityContext.seccompProfile.type",
								"values": [
									"Unconfined"
								]
							},
							{
								"controlName": "Seccomp",
								"restrictedField": "spec.containers[*].securityContext.seccompProfile.type",
								"values": [
									"Unconfined"
								],
								"images": [
									"nginx",
									"nodejs"
								]
							},
							{
								"controlName": "Seccomp",
								"restrictedField": "spec.initContainers[*].securityContext.seccompProfile.type",
								"values": [
									"Unconfined"
								],
								"images": [
									"nginxImageNotMatching"
								]
							},
							{
								"controlName": "Seccomp",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.seccompProfile.type",
								"values": [
									"Unconfined"
								],
								"images": [
									"nginx"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"seccompProfile": {
					"type": "Unconfined"
				},
				"runAsNonRoot": true,
				"allowPrivilegeEscalation": false
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "Unconfined"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "Unconfined"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "Unconfined"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "Unconfined"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

func TestValidate_pod_security_admission_enforce_restricted_exclude_seccomp_missing_restrictedField(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-hostProcesses-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Seccomp",
								"restrictedField": "spec.securityContext.seccompProfile.type",
								"values": [
									"Unconfined"
								]
							},
							{
								"controlName": "Seccomp",
								"restrictedField": "spec.containers[*].securityContext.seccompProfile.type",
								"values": [
									"Unconfined"
								],
								"images": [
									"nginx",
									"nodejs"
								]
							},
							{
								"controlName": "Seccomp",
								"restrictedField": "spec.initContainers[*].securityContext.seccompProfile.type",
								"values": [
									"Unconfined"
								],
								"images": [
									"nginx"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "securityContext": {
				"seccompProfile": {
					"type": "Unconfined"
				},
				"runAsNonRoot": true,
				"allowPrivilegeEscalation": false
		   },
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "Unconfined"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				 }
			  },
			{
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "Unconfined"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "Unconfined"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
				}
			],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "Unconfined"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
					}
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

// === Control: "Privilege Escalation", check.ID: "allowPrivilegeEscalation"

// container-level:
// - spec.containers[*].securityContext.allowPrivilegeEscalation
// - spec.initContainers[*].securityContext.allowPrivilegeEscalation
// - spec.ephemeralContainers[*].securityContext.allowPrivilegeEscalation

func TestValidate_pod_security_admission_enforce_restricted_exclude_all_privilege_escalations(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-privilege_escalations-all-containers-nginx-nodejs"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-privilege_escalations-all-containers-nginx-nodejs",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Privilege Escalation",
								"images": [
									"nginx",
									"nodejs"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": true
				 }
			  },
			  {
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": true
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": true
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": true
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_restricted_exclude_all_privilege_escalations_with_restrictedFields(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-privilege_escalations-all-containers-nginx-nodejs"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-privilege_escalations-all-containers-nginx-nodejs",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Privilege Escalation",
								"restrictedField": "spec.containers[*].securityContext.allowPrivilegeEscalation",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"true"
								]
							},
							{
								"controlName": "Privilege Escalation",
								"restrictedField": "spec.initContainers[*].securityContext.allowPrivilegeEscalation",
								"images": [
									"nginx"
								],
								"values": [
									"true"
								]
							},
							{
								"controlName": "Privilege Escalation",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.allowPrivilegeEscalation",
								"images": [
									"nginx"
								],
								"values": [
									"true"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": true
				 }
			  },
			  {
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": true
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": true
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": true
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_restricted_exclude_all_privilege_escalations_missing_exclude_value(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-privilege_escalations-all-containers-nginx-nodejs"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-privilege_escalations-all-containers-nginx-nodejs",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Privilege Escalation",
								"restrictedField": "spec.containers[*].securityContext.allowPrivilegeEscalation",
								"images": [
									"nginx"
								],
								"values": [
									"true"
								]
							},
							{
								"controlName": "Privilege Escalation",
								"restrictedField": "spec.initContainers[*].securityContext.allowPrivilegeEscalation",
								"images": [
									"nginx"
								],
								"values": [
									"true"
								]
							},
							{
								"controlName": "Privilege Escalation",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.allowPrivilegeEscalation",
								"images": [
									"nginx"
								],
								"values": [
									"true"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": true
				 }
			  },
			  {
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": true
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": true
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": true
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

func TestValidate_pod_security_admission_enforce_restricted_exclude_all_privilege_escalations_missing_restrictedField(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-baseline-exclude-all-privilege_escalations-all-containers-nginx-nodejs"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-baseline-exclude-all-privilege_escalations-all-containers-nginx-nodejs",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Privilege Escalation",
								"restrictedField": "spec.initContainers[*].securityContext.allowPrivilegeEscalation",
								"images": [
									"nginx"
								],
								"values": [
									"true"
								]
							},
							{
								"controlName": "Privilege Escalation",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.allowPrivilegeEscalation",
								"images": [
									"nginx"
								],
								"values": [
									"true"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": true
				 }
			  },
			  {
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"ALL"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": true
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": true
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"ALL"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": true
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

// === Control: "Capabilities", check.ID: "capabilities_restricted"
// container-level:
// - spec.containers[*].securityContext.capabilities.drop
// - spec.initContainers[*].securityContext.capabilities.drop
// - spec.ephemeralContainers[*].securityContext.capabilities.drop
// - spec.containers[*].securityContext.capabilities.add
// - spec.initContainers[*].securityContext.capabilities.add
// - spec.ephemeralContainers[*].securityContext.capabilities.add

func TestValidate_pod_security_admission_enforce_restricted_exclude_all_capabilities(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-restricted-exclude-all-capabilities-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-restricted-exclude-all-capabilities-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Capabilities",
								"images": [
									"nginx",
									"nodejs"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"add": [
							"SYS_TIME"
						],
						"drop": [
							"SYS_ADMIN"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				 }
			  },
			  {
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"add": [
							"SYS_TIME"
						],
						"drop": [
							"SYS_ADMIN"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"add": [
								"SYS_TIME"
							],
							"drop": [
								"SYS_ADMIN"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"add": [
								"SYS_TIME"
							],
							"drop": [
								"SYS_ADMIN"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_restricted_exclude_all_capabilities_with_exclude_value(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-restricted-exclude-all-capabilities-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-restricted-exclude-all-capabilities-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Capabilities",
								"restrictedField": "spec.containers[*].securityContext.capabilities.drop",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"SYS_ADMIN"
								]
							},
							{
								"controlName": "Capabilities",
								"restrictedField": "spec.initContainers[*].securityContext.capabilities.drop",
								"images": [
									"nginx"
								],
								"values": [
									"SYS_ADMIN"
								]
							},
							{
								"controlName": "Capabilities",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.capabilities.drop",
								"images": [
									"nginx"
								],
								"values": [
									"SYS_ADMIN"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"SYS_ADMIN"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				 }
			  },
			  {
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"SYS_ADMIN"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"SYS_ADMIN"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"SYS_ADMIN"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsSuccessful())
}

func TestValidate_pod_security_admission_enforce_restricted_exclude_all_capabilities_missing_exclude_value(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-restricted-exclude-all-capabilities-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-restricted-exclude-all-capabilities-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Capabilities",
								"restrictedField": "spec.containers[*].securityContext.capabilities.drop",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"SYS_TIME"
								]
							},
							{
								"controlName": "Capabilities",
								"restrictedField": "spec.initContainers[*].securityContext.capabilities.drop",
								"images": [
									"nginx"
								],
								"values": [
									"SYS_ADMIN"
								]
							},
							{
								"controlName": "Capabilities",
								"restrictedField": "spec.ephemeralContainers[*].securityContext.capabilities.drop",
								"images": [
									"nginx"
								],
								"values": [
									"SYS_ADMIN"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"SYS_ADMIN"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				 }
			  },
			  {
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"SYS_ADMIN"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"SYS_ADMIN"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"SYS_ADMIN"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

func TestValidate_pod_security_admission_enforce_restricted_exclude_all_capabilities_missing_restrictedField(t *testing.T) {
	rawPolicy := []byte(`
	{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "enforce-restricted-exclude-all-capabilities-all-containers-nginx"
		},
		"spec": {
			"validationFailureAction": "enforce",
			"rules": [
				{
				"name": "enforce-restricted-exclude-all-capabilities-all-containers-nginx",
				"match": {
					"resources": {
					   "kinds": [
						  "Pod"
						],
						"namespaces": [
							"staging"
						]
					}
				 },
				 "validate": {
					"podSecurity": {
						"level": "restricted",
						"version": "v1.24",
						"exclude": [
							{
								"controlName": "Capabilities",
								"restrictedField": "spec.containers[*].securityContext.capabilities.drop",
								"images": [
									"nginx",
									"nodejs"
								],
								"values": [
									"SYS_ADMIN"
								]
							},
							{
								"controlName": "Capabilities",
								"restrictedField": "spec.initContainers[*].securityContext.capabilities.drop",
								"images": [
									"nginx"
								],
								"values": [
									"SYS_ADMIN"
								]
							}
						]
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
		   "name": "nginx-baseline-privileged-container",
		   "namespace": "staging"
		},
		"spec": {
		   "hostNetwork": false,
		   "containers": [
			{
				 "name": "nginx",
				 "image": "nginx",
				 "securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"SYS_ADMIN"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				 }
			  },
			  {
				"name": "nodejs",
				"image": "nodejs",
				"securityContext": {
					"seccompProfile": {
						"type": "RuntimeDefault"
					},
					"capabilities": {
						"drop": [
							"SYS_ADMIN"
						]
					},
					"runAsNonRoot": true,
					"allowPrivilegeEscalation": false
				}
			 }
			],
			"initContainers": [
				{
				   "name": "init-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"SYS_ADMIN"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
				   }
				}
			  ],
			"ephemeralContainers": [
				{
				   "name": "ephemeral-nginx",
				   "image": "nginx",
				   "securityContext": {
						"seccompProfile": {
							"type": "RuntimeDefault"
						},
						"capabilities": {
							"drop": [
								"SYS_ADMIN"
							]
						},
						"runAsNonRoot": true,
						"allowPrivilegeEscalation": false
				   }
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
	er := Validate(&PolicyContext{Policy: &policy, NewResource: *resourceUnstructured, JSONContext: context.NewContext()})

	fmt.Println(er)
	// msgs := []string{""}

	for _, r := range er.PolicyResponse.Rules {
		fmt.Printf("== Response: %+v\n", r.Message)
		// assert.Equal(t, r.Message, msgs[index])
	}
	assert.Assert(t, er.IsFailed())
}

func Test_block_bypass(t *testing.T) {
	testcases := []testCase{
		{
			description:   "Blocks bypass of policy by manipulating pre-conditions",
			policy:        []byte(`{"apiVersion":"kyverno.io/v1","kind":"Policy","metadata":{"name":"configmap-policy"},"spec":{"rules":[{"match":{"resources":{"kinds":["ConfigMap"]}},"name":"key-abc","preconditions":{"any":[{"key":"admin","operator":"Equals","value":"{{request.object.data.lock}}"}]},"validate":{"anyPattern":[{"data":{"key":"abc"}}],"message":"Configmap key must be \"abc\""}}]}}`),
			request:       []byte(`{"uid":"7b0600b7-0258-4ecb-9666-c2839bd19612","kind":{"group":"","version":"v1","kind":"ConfigMap"},"resource":{"group":"","version":"v1","resource":"configmaps"},"subResource":"status","requestKind":{"group":"","version":"v1","kind":"configmaps"},"requestResource":{"group":"","version":"v1","resource":"configmaps"},"name":"test-configmap","namespace":"default","operation":"UPDATE","userInfo":{"username":"system:node:kind-control-plane","groups":["system:authenticated"]},"object":{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"test-configmap"},"data":{"key":"xyz","lock":"admin"}},"oldObject":{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"test-configmap"},"data":{"key":"xyz"}},"dryRun":false,"options":{"kind":"UpdateOptions","apiVersion":"meta.k8s.io/v1"}}`),
			userInfo:      []byte(`{"roles":["kube-system:kubeadm:kubelet-config-1.17","kube-system:kubeadm:nodes-kubeadm-config"],"clusterRoles":["system:basic-user","system:certificates.k8s.io:certificatesigningrequests:selfnodeclient","system:public-info-viewer","system:discovery"],"userInfo":{"username":"kubernetes-admin","groups":["system:authenticated"]}}`),
			requestDenied: true,
		},
	}

	for _, testcase := range testcases {
		executeTest(t, testcase)
	}
}
