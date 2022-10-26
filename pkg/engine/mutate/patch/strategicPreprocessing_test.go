package patch

import (
	"encoding/json"
	"testing"

	"github.com/kyverno/kyverno/pkg/engine/anchor"
	"github.com/kyverno/kyverno/pkg/logging"
	"gotest.tools/assert"
	yaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

func Test_preProcessStrategicMergePatch_multipleAnchors(t *testing.T) {
	testCases := []struct {
		rawPolicy     []byte
		rawResource   []byte
		expectedPatch []byte
	}{
		{
			rawPolicy: []byte(`{
			 "spec": {
      "containers": [
        {
          "(name)": "*",
          "securityContext": {
              "+(allowPrivilegeEscalation)": false,
              "+(capabilities)": {
                  "drop": [
                      "NET_CAP"
                  ]
              },
              "+(privileged)": false
          }
        }
      ],
      "initContainers": [
        {
          "(name)": "*",
          "securityContext": {
              "+(allowPrivilegeEscalation)": false,
              "+(capabilities)": {
                  "drop": [
                      "NET_ADMIN"
                  ]
              },
              "+(privileged)": false
          }
        }
      ]
    }
			}`),
			rawResource: []byte(`{
    "apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
				  "name": "mutation-debug",
      "namespace": "amritapuri"
				},
				"spec": {
     "containers": [
       {
        "name": "sleepy-container-1",
        "image": "docker.io/library/ubuntu"
       },
       {
        "name": "sleepy-container-2",
        "image": "docker.io/library/ubuntu"
       }
     ],
     "initContainers": [
       {
        "name": "init-container-1",
        "image": "docker.io/library/ubuntu"
       },
       {
        "name": "init-container-2",
        "image": "docker.io/library/ubuntu"
       }
     ]
				}
			}`),
			expectedPatch: []byte(`{
				"spec": {
				  "containers": [
        {
          "name": "sleepy-container-1",
          "securityContext": {
            "allowPrivilegeEscalation": false,
            "capabilities": {
              "drop": [
                "NET_CAP"
              ]
            },
            "privileged": false
          }
        },
        {
         "name": "sleepy-container-2",
         "securityContext": {
           "allowPrivilegeEscalation": false,
           "capabilities": {
             "drop": [
               "NET_CAP"
             ]
           },
           "privileged": false
         }
       }
      ],
      "initContainers": [
        {
          "name": "init-container-1",
          "securityContext": {
            "allowPrivilegeEscalation": false,
            "capabilities": {
              "drop": [
                "NET_ADMIN"
              ]
            },
            "privileged": false
          }
        },
        {
         "name": "init-container-2",
         "securityContext": {
           "allowPrivilegeEscalation": false,
           "capabilities": {
             "drop": [
               "NET_ADMIN"
             ]
           },
           "privileged": false
         }
       }
      ]
				}
			  }`),
		},
		{
			rawPolicy: []byte(`{
				"metadata": {
				  "annotations": {
					"+(cluster-autoscaler.kubernetes.io/safe-to-evict)": "true"
				  }
				},
				"spec": {
				  "volumes": [
					{
					  "<(emptyDir)": {}
					}
				  ]
				}
			  }`),
			rawResource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
				  "name": "static-web",
				  "labels": {
					"role": "myrole"
				  }
				},
				"spec": {
				  "containers": [
					{
					  "name": "web",
					  "image": "1nginx"
					}
				  ],
				  "volumes": [
					{
					  "emptyDir": {},
					  "name": "cache-volume"
					},
					{
					  "secret": {
						"secretName": "default-token-6gplg"
					  },
					  "name": "default-token-6gplg"
					}
				  ]
				}
			  }`),
			expectedPatch: []byte(`{
				"metadata": {
				  "annotations": {
					"cluster-autoscaler.kubernetes.io/safe-to-evict": "true"
				  }
				}
			  }`),
		},
		{
			rawPolicy: []byte(`{
				"metadata": null
			  }`),
			rawResource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
				  "name": "hello"
				},
				"spec": {
				  "containers": [
					{
					  "name": "hello",
					  "image": "busybox"
					}
				  ]
				}
			  }`),
			expectedPatch: []byte(`{
				"metadata": null
			  }`),
		},
		{
			rawPolicy: []byte(`{
				"spec": {
				  "containers": [
					{
					  "(name)": "*",
					  "image": "gcr.io/google-containers/busybox:latest"
					}
				  ],
				  "imagePullSecrets": [
					{
					  "name": "regcred"
					}
				  ]
				}
			  }`),
			rawResource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
				  "name": "hello"
				},
				"spec": {
				  "containers": [
					{
					  "name": "hello",
					  "image": "busybox"
					}
				  ]
				}
			  }`),
			expectedPatch: []byte(`{
				"spec": {
				  "containers": [
					{
						"name": "hello",
						"image": "gcr.io/google-containers/busybox:latest"
					}
				  ],
				  "imagePullSecrets": [
					{
					  "name": "regcred"
					}
				  ]
				}
			  }`),
		},
		{
			rawPolicy: []byte(`{
				"spec": {
				  "containers": [
					{
					  "(name)": "*",
					  "(image)": "gcr.io/google-containers/busybox:*",
					  "new_filed": "must be inserted"
					}
				  ],
				  "imagePullSecrets": [
					{
					  "name": "regcred"
					}
				  ]
				}
			  }`),
			rawResource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
				  "name": "hello2"
				},
				"spec": {
				  "containers": [
					{
					  "name": "hello",
					  "image": "gcr.io/google-containers/busybox:latest"
					}
				  ]
				}
			  }`),
			expectedPatch: []byte(`{
				"spec": {
				  "containers": [{
					"name": "hello",
					"new_filed": "must be inserted"
				  }],
				  "imagePullSecrets": [
					{
					  "name": "regcred"
					}
				  ]
				}
			  }`),
		},
		{
			rawPolicy: []byte(`{
				"spec": {
				  "containers": [
					{
					  "(image)": "gcr.io/google-containers/busybox:latest"
					}
				  ],
				  "imagePullSecrets": [
					{
					  "name": "regcred"
					}
				  ]
				}
			  }`),
			rawResource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
				  "name": "hello2"
				},
				"spec": {
				  "containers": [
					{
					  "name": "hello",
					  "image": "gcr.io/google-containers/busybox:latest"
					}
				  ]
				}
			  }`),
			expectedPatch: []byte(`{
				"spec": {
				  "imagePullSecrets": [
					{
					  "name": "regcred"
					}
				  ]
				}
			  }`),
		},
		{
			rawPolicy: []byte(`{
				"spec": {
				  "containers": [
					{
					  "(image)": "gcr.io/google-containers/busybox:*"
					}
				  ],
				  "imagePullSecrets": [
					{
					  "name": "regcred"
					}
				  ]
				}
			  }`),
			rawResource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
				  "name": "hello2"
				},
				"spec": {
				  "containers": [
					{
					  "name": "hello",
					  "image": "gcr.io/google-containers/busybox:latest"
					}
				  ]
				}
			  }`),
			expectedPatch: []byte(`{
				"spec": {
				  "imagePullSecrets": [
					{
					  "name": "regcred"
					}
				  ]
				}
			  }`),
		},
		{
			// only the third container does not match the given condition
			rawPolicy: []byte(`{
				"spec": {
				  "containers": [
					{
					  "(image)": "gcr.io/google-containers/busybox:*"
					}
				  ],
				  "imagePullSecrets": [
					{
					  "name": "regcred"
					}
				  ]
				}
			  }`),
			rawResource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
				  "name": "hello"
				},
				"spec": {
				  "containers": [
					{
					  "name": "hello",
					  "image": "gcr.io/google-containers/busybox:latest"
					},
					{
					  "name": "hello2",
					  "image": "gcr.io/google-containers/busybox:latest"
					},
					{
					  "name": "hello3",
					  "image": "gcr.io/google-containers/nginx:latest"
					}
				  ]
				}
			  }`),
			expectedPatch: []byte(`{
				"spec": {
				  "imagePullSecrets": [
					{
					  "name": "regcred"
					}
				  ]
				}
			  }`),
		},
		{
			rawPolicy: []byte(`{
				"metadata": {
				  "annotations": {
					"+(cluster-autoscaler.kubernetes.io/safe-to-evict)": true
				  },
				  "labels": {
					"+(add-labels)": "add"
				  }
				},
				"spec": {
				  "volumes": [
					{
					  "<(hostPath)": {
						"path": "*"
					  }
					}
				  ]
				}
			  }`),
			rawResource: []byte(`{
				"kind": "Pod",
				"apiVersion": "v1",
				"metadata": {
				  "name": "nginx"
				},
				"spec": {
				  "containers": [
					{
					  "name": "nginx",
					  "image": "nginx:latest",
					  "imagePullPolicy": "Never",
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
					  "hostPath": {
						"path": "/data",
						"type": "Directory"
					  }
					}
				  ]
				}
			  }`),
			expectedPatch: []byte(`{
				"metadata": {
				  "annotations": {
					"cluster-autoscaler.kubernetes.io/safe-to-evict": true
				  },
				  "labels": {
					"add-labels": "add"
				  }
				}
			  }`),
		},
		{
			rawPolicy: []byte(`{
				"metadata": {
				  "annotations": {
					"+(cluster-autoscaler.kubernetes.io/safe-to-evict)": true
				  }
				},
				"spec": {
				  "volumes": [
					{
					  "<(hostPath)": {
						"path": "*"
					  }
					}
				  ]
				}
			  }`),
			rawResource: []byte(`{
				"kind": "Pod",
				"apiVersion": "v1",
				"metadata": {
				  "name": "nginx",
				  "annotations": {
					"cluster-autoscaler.kubernetes.io/safe-to-evict": "false"
				  }
				},
				"spec": {
				  "containers": [
					{
					  "name": "nginx",
					  "image": "nginx:latest",
					  "imagePullPolicy": "Never",
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
					  "hostPath": {
						"path": "/data",
						"type": "Directory"
					  }
					}
				  ]
				}
			  }`),
			expectedPatch: []byte(`{}`),
		},
		{
			rawPolicy: []byte(`{
				"metadata": {
					"annotations": {
						"+(alb.ingress.kubernetes.io/backend-protocol)": "HTTPS",
						"+(alb.ingress.kubernetes.io/healthcheck-protocol)": "HTTPS",
						"+(alb.ingress.kubernetes.io/scheme)": "internal",
						"+(alb.ingress.kubernetes.io/target-type)": "ip",
						"+(kubernetes.io/ingress.class)": "alb"
					}
				}
			}`),
			rawResource: []byte(`{
				"apiVersion": "extensions/v1beta1",
				"kind": "Ingress",
				"metadata": {
				  "annotations": {
					"alb.ingress.kubernetes.io/backend-protocol": "HTTPS",
					"alb.ingress.kubernetes.io/healthcheck-protocol": "HTTPS",
					"alb.ingress.kubernetes.io/scheme": "internal",
					"alb.ingress.kubernetes.io/target-type": "ip",
					"external-dns.alpha.kubernetes.io/hostname": "argo",
					"kubernetes.io/ingress.class": "test"
				  },
				  "labels": {
					"app": "argocd-server",
					"app.kubernetes.io/name": "argocd-server"
				  },
				  "name": "argocd",
				  "namespace": "default"
				}
			  }`),
			expectedPatch: []byte(`{}`),
		},
		{
			rawPolicy: []byte(`{
			"spec": {
				"template": {
				   "spec": {
					  "containers": [
						 {
							"(name)": "*",
							"resources": {
							   "limits": {
								  "+(memory)": "300Mi",
								  "+(cpu)": "100"
							   }
							}
						 }
					  ]
				   }
				}
			 }
			}`),
			rawResource: []byte(`{
				"apiVersion": "apps/v1",
				"kind": "Deployment",
				"metadata": {
				   "name": "qos-demo",
				   "labels": {
					  "test": "qos"
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
							   "resources": {
								  "limits": {
									 "cpu": "50m"
								  }
							   }
							}
						 ]
					  }
				   }
				}
			 }`),
			expectedPatch: []byte(`{
				"spec": {
				  "template": {
					"spec": {
					  "containers": [
						{
						  "resources": {
							"limits": {
							  "memory": "300Mi"
							}
						  },
						  "name": "nginx"
						}
					  ]
					}
				  }
				}
			  }`),
		},
		{
			rawPolicy: []byte(`{
				"metadata": {
				  "annotations": {
					"+(annotation1)": "atest1",
					"+(annotation2)": "atest2"
				  },
				  "labels": {
					"+(label1)": "test1"
				  }
				},
				"spec": {
				  "(volumes)": [
					{
					  "(hostPath)": {
						"path": "/var/run/docker.sock"
					  }
					}
				  ],
				  "containers": [
					{
					  "(image)": "*:latest",
					  "command": [
						"ls"
					  ],
					  "imagePullPolicy": "Always"
					}
				  ]
				}
			  }`),
			rawResource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
				  "annotations": {
					"annotation1": "atest2"
				  },
				  "labels": {
					"label1": "test2",
					"label2": "test2"
				  },
				  "name": "check-root-user"
				},
				"spec": {
				  "containers": [
					{
					  "command": [
						"ll"
					  ],
					  "image": "nginx:latest",
					  "imagePullPolicy": "Never",
					  "name": "nginx"
					},
					{
					  "image": "busybox:latest",
					  "imagePullPolicy": "Never",
					  "name": "busybox"
					}
				  ],
				  "volumes": [
					{
					  "hostPath": {
						"path": "/var/run/docker.sock"
					  },
					  "name": "test-volume"
					}
				  ]
				}
			  }`),
			expectedPatch: []byte(`{
				"metadata": {
				  "annotations": {
					"annotation2": "atest2"
				  }
				},
				"spec": {
				  "containers": [
					{
					  "command": [
						"ls"
					  ],
					  "imagePullPolicy": "Always",
					  "name": "nginx"
					},
					{
					  "command": [
						"ls"
					  ],
					  "imagePullPolicy": "Always",
					  "name": "busybox"
					}
				  ]
				}
			  }`),
		},
		{
			rawPolicy: []byte(`{
				"metadata": {
					"annotations": {
						"+(annotation1)": "atest1",
				  	}
				}
			}`),
			rawResource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"annotations": {
						"annotation1": "atest2"
				  	},
				  	"labels": {
						"label1": "test2",
						"label2": "test2"
				  	},
				  	"name": "check-root-user"
				}
			}`),
			expectedPatch: []byte(`{}`),
		},
		{
			rawPolicy: []byte(`{
				"metadata": {
					"annotations": {
						"annotation1": null
				  	}
				}
			}`),
			rawResource: []byte(`{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"annotations": {
						"annotation1": "atest2"
				  	},
				  	"labels": {
						"label1": "test2",
						"label2": "test2"
				  	},
				  	"name": "check-root-user"
				}
			}`),
			expectedPatch: []byte(`{
				"metadata": {
					"annotations": {
						"annotation1": null
				  	}
				}
			}`),
		},
		{
			rawPolicy: []byte(`{
				"metadata": {
				  "annotations": {
					"+(cluster-autoscaler.kubernetes.io/safe-to-evict)": true
				  }
				},
				"spec": {
				  "volumes": [
					{
					  "hostPath": {
						"<(path)": "*data"
					  }
					}
				  ]
				}
			  }`),
			rawResource: []byte(`{
				"kind": "Pod",
				"apiVersion": "v1",
				"metadata": {
				  "name": "nginx"
				},
				"spec": {
				  "containers": [
					{
					  "name": "nginx",
					  "image": "nginx:latest",
					  "imagePullPolicy": "Never",
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
					  "hostPath": {
						"path": "/data",
						"type": "Directory"
					  }
					}
				  ]
				}
			  }`),
			expectedPatch: []byte(`{
				"metadata": {
				  "annotations": {
					"cluster-autoscaler.kubernetes.io/safe-to-evict": true
				  }
				}
			  }`),
		},
		{
			rawPolicy: []byte(`{
				"spec": {
				  "securityContext": {
					"runAsNonRoot": true
				  },
				  "initContainers": [
					{
					  "(name)": "*",
					  "securityContext": {
						"runAsNonRoot": true
					  }
					}
				  ],
				  "containers": [
					{
					  "(name)": "*",
					  "securityContext": {
						"runAsNonRoot": true
					  }
					}
				  ]
				}
			  }`),
			rawResource: []byte(`{
				"spec":{
				   "initContainers":[
					  {
						 "name":"initbusy",
						 "image":"busybox:1.28",
						 "command":[
							"sleep",
							"9999"
						 ]
					  }
				   ],
				   "containers":[
					  {
						 "image":"busybox:1.28",
						 "name":"busybox",
						 "command":[
							"sleep",
							"9999"
						 ]
					  }
				   ],
				   "affinity":{
					  "podAntiAffinity":{
						 "requiredDuringSchedulingIgnoredDuringExecution":[
							{
							   "labelSelector":{
								  "matchExpressions":[
									 {
										"key":"app",
										"operator":"In",
										"values":[
										   "foo",
										   "bar"
										]
									 }
								  ]
							   },
							   "topologyKey":"kubernetes.io/hostname"
							}
						 ]
					  }
				   }
				}
			 }`),
			expectedPatch: []byte(`{
				"spec": {
				  "securityContext": {
					"runAsNonRoot": true
				  },
				  "initContainers": [
					{
					  "name": "initbusy",
					  "securityContext": {
						"runAsNonRoot": true
					  }
					}
				  ],
				  "containers": [
					{
					  "name": "busybox",
					  "securityContext": {
						"runAsNonRoot": true
					  }
					}
				  ]
				}
			  }`),
		},
	}

	for i, test := range testCases {
		t.Logf("Running test %d...", i)
		preProcessedPolicy, err := preProcessStrategicMergePatch(logging.GlobalLogger(), string(test.rawPolicy), string(test.rawResource))
		assert.NilError(t, err)

		output, err := preProcessedPolicy.MarshalJSON()
		assert.NilError(t, err)

		assert.DeepEqual(t, toJSON(t, []byte(test.expectedPatch)), toJSON(t, output))
	}
}

func toJSON(t *testing.T, b []byte) interface{} {
	var i interface{}
	err := json.Unmarshal(b, &i)
	assert.NilError(t, err)
	return i
}

func Test_FilterKeys_NoConditions(t *testing.T) {
	patternRaw := []byte(`{
		"key1": "value1",
		"key2": "value2"
	}`)

	pattern := yaml.MustParse(string(patternRaw))
	conditions, err := filterKeys(pattern, anchor.IsConditionAnchor)

	assert.NilError(t, err)
	assert.Equal(t, len(conditions), 0)
}

func Test_FilterKeys_ConditionsArePresent(t *testing.T) {
	patternRaw := []byte(`{
		"key1": "value1",
		"(key2)": "value2",
		"(key3)": "value3"
	}`)

	pattern := yaml.MustParse(string(patternRaw))
	conditions, err := filterKeys(pattern, anchor.IsConditionAnchor)

	assert.NilError(t, err)
	assert.Equal(t, len(conditions), 2)
	assert.Equal(t, conditions[0], "(key2)")
	assert.Equal(t, conditions[1], "(key3)")
}

func Test_FilterKeys_EmptyList(t *testing.T) {
	patternRaw := []byte(`{}`)
	pattern := yaml.MustParse(string(patternRaw))
	conditions, err := filterKeys(pattern, anchor.IsConditionAnchor)

	assert.NilError(t, err)
	assert.Equal(t, len(conditions), 0)
}

func Test_CheckConditionAnchor_Matches(t *testing.T) {
	patternRaw := []byte(`{ "key1": "value*" }`)
	resourceRaw := []byte(`{ "key1": "value1" }`)

	pattern := yaml.MustParse(string(patternRaw))
	resource := yaml.MustParse(string(resourceRaw))

	err := checkCondition(logging.GlobalLogger(), pattern, resource)
	assert.Equal(t, err, nil)
}

func Test_CheckConditionAnchor_DoesNotMatch(t *testing.T) {
	patternRaw := []byte(`{ "key1": "value*" }`)
	resourceRaw := []byte(`{ "key1": "sample" }`)

	pattern := yaml.MustParse(string(patternRaw))
	resource := yaml.MustParse(string(resourceRaw))

	err := checkCondition(logging.GlobalLogger(), pattern, resource)
	assert.Error(t, err, "resource value 'sample' does not match 'value*' at path /key1/")
}

func Test_ValidateConditions_MapWithOneCondition_Matches(t *testing.T) {
	patternRaw := []byte(`{
		"(key1)": "value*",
		"key2": "value2"
	}`)

	resourceRaw := []byte(`{
		"key1": "value1",
		"key2": "sample"
	}`)

	pattern := yaml.MustParse(string(patternRaw))
	resource := yaml.MustParse(string(resourceRaw))

	err := validateConditions(logging.GlobalLogger(), pattern, resource)
	assert.NilError(t, err)
}

func Test_ValidateConditions_MapWithOneCondition_DoesNotMatch(t *testing.T) {
	patternRaw := []byte(`{
		"(key1)": "value*",
		"key2": "value2"
	}`)

	resourceRaw := []byte(`{
		"key1": "text",
		"key2": "sample"
	}`)

	pattern := yaml.MustParse(string(patternRaw))
	resource := yaml.MustParse(string(resourceRaw))

	err := validateConditions(logging.GlobalLogger(), pattern, resource)
	_, ok := err.(ConditionError)
	assert.Assert(t, ok)
}

func Test_RenameField(t *testing.T) {
	patternRaw := []byte(`{
		"+(key1)": "value",
	}`)

	pattern := yaml.MustParse(string(patternRaw))
	renameField("+(key1)", "key1", pattern)

	actual := pattern.Field("key1").Value.YNode().Value
	expected := "value"

	fields, err := pattern.Fields()
	assert.NilError(t, err)

	assert.Equal(t, len(fields), 1)
	assert.Equal(t, actual, expected)
}

func Test_RenameField_NonExisting(t *testing.T) {
	patternRaw := []byte(`{
		"+(key1)": "value",
	}`)

	pattern := yaml.MustParse(string(patternRaw))
	renameField("non_existing_field", "key1", pattern)

	actual := pattern.Field("+(key1)").Value.YNode().Value
	expected := "value"

	fields, err := pattern.Fields()
	assert.NilError(t, err)

	assert.Equal(t, len(fields), 1)
	assert.Equal(t, actual, expected)
}

func Test_deleteRNode(t *testing.T) {
	patternRaw := []byte(`{
		"list": [
			"first": {
				"a": "b"
			},
			"second": {
				"a": "b"
			},
			"third": {
				"a": "b"
			},
		],
	}`)

	pattern := yaml.MustParse(string(patternRaw))
	list := pattern.Field("list").Value
	elements, err := list.Elements()
	assert.NilError(t, err)

	assert.Equal(t, len(elements), 3)
	deleteListElement(list, 0)

	elements, err = list.Elements()
	assert.NilError(t, err)
	assert.Equal(t, len(elements), 2)
}

func Test_DeleteConditions(t *testing.T) {
	patternRaw := []byte(`{
		"spec": {
		  "containers": [
			{
			  "(name)": "*",
			  "image": "gcr.io/google-containers/busybox:latest"
			},
			{
			  "image": "gcr.io/google-containers/busybox:latest",
			  "name": "hello"
			}
		  ],
		  "imagePullSecrets": [
			{
			  "name": "regcred"
			}
		  ]
		}
	  }`)

	pattern := yaml.MustParse(string(patternRaw))

	containers, err := pattern.Field("spec").Value.Field("containers").Value.Elements()
	assert.NilError(t, err)
	assert.Equal(t, len(containers), 2)

	_, err = deleteAnchors(pattern, false, false)
	assert.NilError(t, err)

	containers, err = pattern.Field("spec").Value.Field("containers").Value.Elements()
	assert.NilError(t, err)
	assert.Equal(t, len(containers), 1)

	name := containers[0].Field("name").Value.YNode().Value
	assert.Equal(t, name, "hello")
}

func Test_ConditionCheck_SeveralElementsMatchExceptOne(t *testing.T) {
	patternRaw := []byte(`{
		"containers": [
			{
			  "(name)": "hello?",
			  "image": "gcr.io/google-containers/busybox:1"
			}
		]
	}`)

	containersRaw := []byte(`{
		"containers": [
			{
			  "name": "hello",
			  "image": "gcr.io/google-containers/busybox:latest"
			},
			{
			  "name": "hello2",
			  "image": "gcr.io/google-containers/busybox:latest"
			},
			{
			  "name": "hello3",
			  "image": "gcr.io/google-containers/nginx:latest"
			}
		  ]
	}`)

	pattern := yaml.MustParse(string(patternRaw))
	containers := yaml.MustParse(string(containersRaw))

	err := preProcessPattern(logging.GlobalLogger(), pattern, containers)
	assert.NilError(t, err)

	patternContainers := pattern.Field("containers")
	assert.Assert(t, patternContainers != nil)
	assert.Assert(t, patternContainers.Value != nil)

	elements, err := patternContainers.Value.Elements()
	assert.NilError(t, err)

	assert.Equal(t, len(elements), 2)
}

func Test_NonExistingKeyMustFailPreprocessing(t *testing.T) {
	rawPattern := []byte(`{
			"metadata": {
				"labels": {
					"(key1)": "value1",
				}
			},
			"spec": {
			  "containers": [
				{
				  "name": "busybox",
				  "image": "gcr.io/google-containers/busybox:latest"
				}
			  ],
			  "imagePullSecrets": [
				{
				  "name": "regcred"
				}
			  ]
			}
		  }`)

	rawResource := []byte(`{
			"apiVersion": "v1",
			"kind": "Pod",
			"metadata": {
			  "name": "hello"
			},
			"spec": {
			  "containers": [
				{
				  "name": "hello",
				  "image": "busybox"
				}
			  ]
			}
		  }`)

	pattern := yaml.MustParse(string(rawPattern))
	resource := yaml.MustParse(string(rawResource))
	err := preProcessPattern(logging.GlobalLogger(), pattern, resource)
	assert.Error(t, err, "condition failed: could not found \"key1\" key in the resource")
}

func Test_NestedConditionals(t *testing.T) {
	rawPattern := `{"spec":{"template":{"spec":{"volumes":[{"(emptyDir)":{"+(sizeLimit)":"20Mi"},"name":"*"}]}}}}`
	rawResource := `{"spec":{"template":{"spec":{"volumes":[{"emptyDir":{},"name":"vol02"}]}}}}`
	expectedPattern := `{"spec":{"template":{"spec":{"volumes":[{"emptyDir":{"sizeLimit":"20Mi"},"name":"vol02"}]}}}}`

	pattern := yaml.MustParse(rawPattern)
	resource := yaml.MustParse(rawResource)
	err := preProcessPattern(logging.GlobalLogger(), pattern, resource)
	assert.NilError(t, err)
	resultPattern, _ := pattern.String()

	assert.DeepEqual(t, toJSON(t, []byte(expectedPattern)), toJSON(t, []byte(resultPattern)))
}
