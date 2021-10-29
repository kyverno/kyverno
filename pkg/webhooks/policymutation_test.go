package webhooks

import (
	"encoding/json"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/policymutation"

	assertnew "github.com/stretchr/testify/assert"
	"gotest.tools/assert"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func compareJSONAsMap(t *testing.T, expected, actual []byte) {
	var expectedMap, actualMap map[string]interface{}
	assert.NilError(t, json.Unmarshal(expected, &expectedMap))
	assert.NilError(t, json.Unmarshal(actual, &actualMap))

	if !assertnew.Equal(t, expectedMap, actualMap) {
		t.FailNow()
	}
}

func TestGeneratePodControllerRule_NilAnnotation(t *testing.T) {
	policyRaw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "add-safe-to-evict"
		}
	  }`)

	var policy kyverno.ClusterPolicy
	assert.Assert(t, json.Unmarshal(policyRaw, &policy))
	patches, errs := policymutation.GeneratePodControllerRule(policy, log.Log)
	assert.Assert(t, len(errs) == 0)

	p, err := utils.ApplyPatches(policyRaw, patches)
	assert.NilError(t, err)

	expectedPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "add-safe-to-evict",
		  "annotations": {
			"pod-policies.kyverno.io/autogen-controllers": "DaemonSet,Deployment,Job,StatefulSet,CronJob"
		  }
		}
	  }`)
	compareJSONAsMap(t, p, expectedPolicy)
}

func TestGeneratePodControllerRule_PredefinedAnnotation(t *testing.T) {
	policyRaw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "add-safe-to-evict",
		  "annotations": {
			"pod-policies.kyverno.io/autogen-controllers": "StatefulSet,Pod"
		  }
		}
	  }`)

	var policy kyverno.ClusterPolicy
	assert.Assert(t, json.Unmarshal(policyRaw, &policy))
	patches, errs := policymutation.GeneratePodControllerRule(policy, log.Log)
	assert.Assert(t, len(errs) == 0)
	assert.Assert(t, len(patches) == 0)
}

func TestGeneratePodControllerRule_DisableFeature(t *testing.T) {
	policyRaw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "annotations": {
			"a": "b",
			"pod-policies.kyverno.io/autogen-controllers": "none"
		  },
		  "name": "add-safe-to-evict"
		},
		"spec": {
		  "rules": [
			{
			  "name": "annotate-empty-dir",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "mutate": {
				"patchStrategicMerge": {
				  "metadata": {
					"annotations": {
					  "+(cluster-autoscaler.kubernetes.io/safe-to-evict)": "true"
					}
				  },
				  "spec": {
					"volumes": [
					  {
						"(emptyDir)": {
						}
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
	assert.Assert(t, json.Unmarshal(policyRaw, &policy))
	patches, errs := policymutation.GeneratePodControllerRule(policy, log.Log)
	assert.Assert(t, len(errs) == 0)
	assert.Assert(t, len(patches) == 0)
}

func TestGeneratePodControllerRule_Mutate(t *testing.T) {
	policyRaw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "annotations": {
			"a": "b",
			"pod-policies.kyverno.io/autogen-controllers": "all"
		  },
		  "name": "add-safe-to-evict"
		},
		"spec": {
		  "rules": [
			{
			  "name": "annotate-empty-dir",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "mutate": {
				"patchStrategicMerge": {
				  "metadata": {
					"annotations": {
					  "+(cluster-autoscaler.kubernetes.io/safe-to-evict)": "true"
					}
				  },
				  "spec": {
					"volumes": [
					  {
						"(emptyDir)": {
						}
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
	assert.Assert(t, json.Unmarshal(policyRaw, &policy))
	patches, errs := policymutation.GeneratePodControllerRule(policy, log.Log)
	assert.Assert(t, len(errs) == 0)

	p, err := utils.ApplyPatches(policyRaw, patches)
	assert.NilError(t, err)

	expectedPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "annotations": {
			"a": "b",
			"pod-policies.kyverno.io/autogen-controllers": "all"
		  },
		  "name": "add-safe-to-evict"
		},
		"spec": {
		  "rules": [
			{
			  "name": "annotate-empty-dir",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "mutate": {
				"patchStrategicMerge": {
				  "metadata": {
					"annotations": {
					  "+(cluster-autoscaler.kubernetes.io/safe-to-evict)": "true"
					}
				  },
				  "spec": {
					"volumes": [
					  {
						"(emptyDir)": {
						}
					  }
					]
				  }
				}
			  }
			},
			{
			  "name": "autogen-annotate-empty-dir",
			  "match": {
				"resources": {
				  "kinds": [
					"DaemonSet",
					"Deployment",
					"Job",
					"StatefulSet"
				  ]
				}
			  },
			  "mutate": {
				"patchStrategicMerge": {
				  "spec": {
					"template": {
					  "metadata": {
						"annotations": {
						  "+(cluster-autoscaler.kubernetes.io/safe-to-evict)": "true"
						}
					  },
					  "spec": {
						"volumes": [
						  {
							"(emptyDir)": {
							}
						  }
						]
					  }
					}
				  }
				}
			  }
			},
			{
			  "name": "autogen-cronjob-annotate-empty-dir",
			  "match": {
				"resources": {
				  "kinds": [
					"CronJob"
				  ]
				}
			  },
			  "mutate": {
				"patchStrategicMerge": {
				  "spec": {
					"jobTemplate": {
					  "spec": {
						"template": {
						  "metadata": {
							"annotations": {
							  "+(cluster-autoscaler.kubernetes.io/safe-to-evict)": "true"
							}
						  },
						  "spec": {
							"volumes": [
							  {
								"(emptyDir)": {
								}
							  }
							]
						  }
						}
					  }
					}
				  }
				}
			  }
			}
		  ]
		}
	  }`)

	compareJSONAsMap(t, expectedPolicy, p)
}
func TestGeneratePodControllerRule_ExistOtherAnnotation(t *testing.T) {
	policyRaw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "add-safe-to-evict",
		  "annotations": {
			"test": "annotation"
		  }
		}
	  }`)

	var policy kyverno.ClusterPolicy
	assert.Assert(t, json.Unmarshal(policyRaw, &policy))
	patches, errs := policymutation.GeneratePodControllerRule(policy, log.Log)
	assert.Assert(t, len(errs) == 0)

	p, err := utils.ApplyPatches(policyRaw, patches)
	assert.NilError(t, err)

	expectedPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "add-safe-to-evict",
		  "annotations": {
			"pod-policies.kyverno.io/autogen-controllers": "DaemonSet,Deployment,Job,StatefulSet,CronJob",
			"test": "annotation"
		  }
		}
	  }`)
	compareJSONAsMap(t, p, expectedPolicy)
}

func TestGeneratePodControllerRule_ValidateAnyPattern(t *testing.T) {
	policyRaw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "annotations": {
			"pod-policies.kyverno.io/autogen-controllers": "Deployment"
		  },
		  "name": "add-safe-to-evict"
		},
		"spec": {
		  "rules": [
			{
			  "name": "validate-runAsNonRoot",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "validate": {
				"message": "Running as root user is not allowed. Set runAsNonRoot to true",
				"anyPattern": [
				  {
					"spec": {
					  "securityContext": {
						"runAsNonRoot": true
					  }
					}
				  },
				  {
					"spec": {
					  "containers": [
						{
						  "name": "*",
						  "securityContext": {
							"runAsNonRoot": true
						  }
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

	var policy kyverno.ClusterPolicy
	assert.Assert(t, json.Unmarshal(policyRaw, &policy))
	patches, errs := policymutation.GeneratePodControllerRule(policy, log.Log)
	assert.Assert(t, len(errs) == 0)

	p, err := utils.ApplyPatches(policyRaw, patches)
	assert.NilError(t, err)

	expectedPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "annotations": {
			"pod-policies.kyverno.io/autogen-controllers": "Deployment"
		  },
		  "name": "add-safe-to-evict"
		},
		"spec": {
		  "rules": [
			{
			  "name": "validate-runAsNonRoot",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "validate": {
				"message": "Running as root user is not allowed. Set runAsNonRoot to true",
				"anyPattern": [
				  {
					"spec": {
					  "securityContext": {
						"runAsNonRoot": true
					  }
					}
				  },
				  {
					"spec": {
					  "containers": [
						{
						  "name": "*",
						  "securityContext": {
							"runAsNonRoot": true
						  }
						}
					  ]
					}
				  }
				]
			  }
			},
			{
			  "name": "autogen-validate-runAsNonRoot",
			  "match": {
				"resources": {
				  "kinds": [
					"Deployment"
				  ]
				}
			  },
			  "validate": {
				"message": "Running as root user is not allowed. Set runAsNonRoot to true",
				"anyPattern": [
				  {
					"spec": {
					  "template": {
						"spec": {
						  "securityContext": {
							"runAsNonRoot": true
						  }
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
							  "name": "*",
							  "securityContext": {
								"runAsNonRoot": true
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
		  ]
		}
	  }`)
	compareJSONAsMap(t, p, expectedPolicy)
}

func TestGeneratePodControllerRule_ValidatePattern(t *testing.T) {
	policyRaw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "add-safe-to-evict"
		},
		"spec": {
		  "rules": [
			{
			  "name": "validate-docker-sock-mount",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "validate": {
				"message": "Use of the Docker Unix socket is not allowed",
				"pattern": {
				  "spec": {
					"=(volumes)": [
					  {
						"=(hostPath)": {
						  "path": "!/var/run/docker.sock"
						}
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
	// var policy, generatePolicy unstructured.Unstructured
	assert.Assert(t, json.Unmarshal(policyRaw, &policy))
	patches, errs := policymutation.GeneratePodControllerRule(policy, log.Log)
	assert.Assert(t, len(errs) == 0)

	p, err := utils.ApplyPatches(policyRaw, patches)
	assert.NilError(t, err)

	expectedPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "annotations": {
			"pod-policies.kyverno.io/autogen-controllers": "DaemonSet,Deployment,Job,StatefulSet,CronJob"
		  },
		  "name": "add-safe-to-evict"
		},
		"spec": {
		  "rules": [
			{
			  "name": "validate-docker-sock-mount",
			  "match": {
				"resources": {
				  "kinds": [
					"Pod"
				  ]
				}
			  },
			  "validate": {
				"message": "Use of the Docker Unix socket is not allowed",
				"pattern": {
				  "spec": {
					"=(volumes)": [
					  {
						"=(hostPath)": {
						  "path": "!/var/run/docker.sock"
						}
					  }
					]
				  }
				}
			  }
			},
			{
			  "name": "autogen-validate-docker-sock-mount",
			  "match": {
				"resources": {
				  "kinds": [
					"DaemonSet",
					"Deployment",
					"Job",
					"StatefulSet"
				  ]
				}
			  },
			  "validate": {
				"message": "Use of the Docker Unix socket is not allowed",
				"pattern": {
				  "spec": {
					"template": {
					  "spec": {
						"=(volumes)": [
						  {
							"=(hostPath)": {
							  "path": "!/var/run/docker.sock"
							}
						  }
						]
					  }
					}
				  }
				}
			  }
			},
			{
			  "name": "autogen-cronjob-validate-docker-sock-mount",
			  "match": {
				"resources": {
				  "kinds": [
					"CronJob"
				  ]
				}
			  },
			  "validate": {
				"message": "Use of the Docker Unix socket is not allowed",
				"pattern": {
				  "spec": {
					"jobTemplate": {
					  "spec": {
						"template": {
						  "spec": {
							"=(volumes)": [
							  {
								"=(hostPath)": {
								  "path": "!/var/run/docker.sock"
								}
							  }
							]
						  }
						}
					  }
					}
				  }
				}
			  }
			}
		  ]
		}
	  }`)

	compareJSONAsMap(t, expectedPolicy, p)
}
