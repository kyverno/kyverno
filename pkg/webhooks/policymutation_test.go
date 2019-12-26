package webhooks

import (
	"encoding/json"
	"reflect"
	"testing"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine"
	"gotest.tools/assert"
)

func compareJSONAsMap(t *testing.T, expected, actual []byte) {
	var expectedMap, actualMap map[string]interface{}
	assert.NilError(t, json.Unmarshal(expected, &expectedMap))
	assert.NilError(t, json.Unmarshal(actual, &actualMap))
	assert.Assert(t, reflect.DeepEqual(expectedMap, actualMap))
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
	patches, errs := generatePodControllerRule(policy)
	assert.Assert(t, len(errs) == 0)

	p, err := engine.ApplyPatches(policyRaw, patches)
	assert.NilError(t, err)

	expectedPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "add-safe-to-evict",
		  "annotations": {
			"pod-policies.kyverno.io/autogen-controllers": "all"
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
	patches, errs := generatePodControllerRule(policy)
	assert.Assert(t, len(errs) == 0)
	assert.Assert(t, len(patches) == 0)
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
	patches, errs := generatePodControllerRule(policy)
	assert.Assert(t, len(errs) == 0)

	p, err := engine.ApplyPatches(policyRaw, patches)
	assert.NilError(t, err)

	expectedPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "add-safe-to-evict",
		  "annotations": {
			"pod-policies.kyverno.io/autogen-controllers": "all",
			"test": "annotation"
		  }
		}
	  }`)
	compareJSONAsMap(t, p, expectedPolicy)
}

func TestGeneratePodControllerRule(t *testing.T) {
	policyRaw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "add-safe-to-evict",
		   "annotations": {
			  "a": "b"
		   }
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
					"overlay": {
					   "metadata": {
						  "annotations": {
							 "+(cluster-autoscaler.kubernetes.io/safe-to-evict)": "true"
						  }
					   },
					   "spec": {
						  "volumes": [
							 {
								"(emptyDir)": {}
							 }
						  ]
					   }
					}
				 }
			  },
			  {
				 "name": "annotate-host-path",
				 "match": {
					"resources": {
					   "kinds": [
						  "Pod"
					   ]
					}
				 },
				 "mutate": {
					"overlay": {
					   "metadata": {
						  "annotations": {
							 "+(cluster-autoscaler.kubernetes.io/safe-to-evict)": "true"
						  }
					   },
					   "spec": {
						  "volumes": [
							 {
								"(hostPath)": {
								   "path": "*"
								}
							 }
						  ]
					   }
					}
				 }
			  },
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
	assert.Assert(t, json.Unmarshal(policyRaw, &policy))
	patches, errs := generatePodControllerRule(policy)
	assert.Assert(t, len(errs) == 0)

	p, err := engine.ApplyPatches(policyRaw, patches)
	assert.NilError(t, err)

	expectPolicy := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		   "name": "add-safe-to-evict",
		   "annotations": {
			  "a": "b",
			  "pod-policies.kyverno.io/autogen-controllers": "all"
		   }
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
					"overlay": {
					   "metadata": {
						  "annotations": {
							 "+(cluster-autoscaler.kubernetes.io/safe-to-evict)": "true"
						  }
					   },
					   "spec": {
						  "volumes": [
							 {
								"(emptyDir)": {}
							 }
						  ]
					   }
					}
				 }
			  },
			  {
				 "name": "annotate-host-path",
				 "match": {
					"resources": {
					   "kinds": [
						  "Pod"
					   ]
					}
				 },
				 "mutate": {
					"overlay": {
					   "metadata": {
						  "annotations": {
							 "+(cluster-autoscaler.kubernetes.io/safe-to-evict)": "true"
						  }
					   },
					   "spec": {
						  "volumes": [
							 {
								"(hostPath)": {
								   "path": "*"
								}
							 }
						  ]
					   }
					}
				 }
			  },
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
					"overlay": {
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
									  "(emptyDir)": {}
								   }
								]
							 }
						  }
					   }
					}
				 }
			  },
			  {
				 "name": "autogen-annotate-host-path",
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
					"overlay": {
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
									  "(hostPath)": {
										 "path": "*"
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
				 "name": "autogen-validate-runAsNonRoot",
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
			  }
		   ]
		}
	 }`)
	compareJSONAsMap(t, expectPolicy, p)
}
