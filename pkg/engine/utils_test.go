package engine

import (
	"encoding/json"
	"testing"

	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMatchesResourceDescription(t *testing.T) {
	tcs := []struct {
		Description       string
		AdmissionInfo     kyverno.RequestInfo
		Resource          []byte
		Policy            []byte
		areErrorsExpected bool
	}{
		{
			Description: "Should match pod and not exclude it",
			AdmissionInfo: kyverno.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"name":"hello-world"},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: false,
		},
		{
			Description: "Should exclude resource since it matches the exclude block",
			AdmissionInfo: kyverno.RequestInfo{
				ClusterRoles: []string{"system:node"},
			},
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"name":"hello-world"},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: true,
		},
		{
			Description:       "Should not fail if in sync mode, if admission info is empty it should still match resources with specific clusterRoles",
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"name":"hello-world"},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: false,
		},
		{
			Description:       "Should fail since resource does not match because of names field",
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"],"names": ["dev-*"]},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: true,
		},
		{
			Description:       "Should pass since resource matches a name in the names field",
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"],"names": ["dev-*","hello-world"]},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: false,
		},
		{
			Description:       "Should fail since resource gets excluded because of the names field",
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"names": ["dev-*","hello-*"]}},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: true,
		},
		{
			Description:       "Should pass since resource does not get excluded because of the names field",
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"bye-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"names": ["dev-*","hello-*"]}},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: false,
		},
		{
			Description: "Should fail since resource does not match policy",
			AdmissionInfo: kyverno.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{"apiVersion":"v1","kind":"Service","metadata":{"name":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"name":"hello-world"},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: true,
		},
		{
			Description: "Should not fail since resource does not match exclude block",
			AdmissionInfo: kyverno.RequestInfo{
				ClusterRoles: []string{"system:node"},
			},
			Resource:          []byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"hello-world2","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"name":"hello-world"},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: false,
		},
		{
			Description: "Should pass since group, version, kind match",
			AdmissionInfo: kyverno.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{ "apiVersion": "apps/v1", "kind": "Deployment", "metadata": { "creationTimestamp": "2020-09-21T12:56:35Z", "name": "qos-demo", "labels": { "test": "qos" } }, "spec": { "replicas": 1, "selector": { "matchLabels": { "app": "nginx" } }, "template": { "metadata": { "creationTimestamp": "2020-09-21T12:56:35Z", "labels": { "app": "nginx" } }, "spec": { "containers": [ { "name": "nginx", "image": "nginx:latest", "resources": { "limits": { "cpu": "50m" } } } ]}}}}`),
			Policy:            []byte(`{ "apiVersion": "kyverno.io/v1", "kind": "ClusterPolicy", "metadata": { "name": "policy-qos" }, "spec": { "validationFailureAction": "enforce", "rules": [ { "name": "add-memory-limit", "match": { "resources": { "kinds": [ "apps/v1/Deployment" ], "selector": { "matchLabels": { "test": "qos" } } } }, "mutate": { "overlay": { "spec": { "template": { "spec": { "containers": [ { "(name)": "*", "resources": { "limits": { "+(memory)": "300Mi", "+(cpu)": "100" } } } ] } } } } } }, { "name": "check-cpu-memory-limits", "match": { "resources": { "kinds": [ "apps/v1/Deployment" ], "selector": { "matchLabels": { "test": "qos" } } } }, "validate": { "message": "Resource limits are required for CPU and memory", "pattern": { "spec": { "template": { "spec": { "containers": [ { "(name)": "*", "resources": { "limits": { "memory": "?*", "cpu": "?*" } } } ] } } } } } } ] } }`),
			areErrorsExpected: false,
		},
		{
			Description: "Should pass since version and kind match",
			AdmissionInfo: kyverno.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{ "apiVersion": "v1", "kind": "Pod", "metadata": { "name": "myapp-pod2", "labels": { "app": "myapp2" } }, "spec": { "containers": [ { "name": "nginx", "image": "nginx" } ] } }`),
			Policy:            []byte(`{ "apiVersion": "kyverno.io/v1", "kind": "ClusterPolicy", "metadata": { "name": "disallow-latest-tag", "annotations": { "policies.kyverno.io/category": "Workload Isolation", "policies.kyverno.io/description": "The ':latest' tag is mutable and can lead to unexpected errors if the image changes. A best practice is to use an immutable tag that maps to a specific version of an application pod." } }, "spec": { "validationFailureAction": "enforce", "rules": [ { "name": "require-image-tag", "match": { "resources": { "kinds": [ "v1/Pod" ] } }, "validate": { "message": "An image tag is required", "pattern": { "spec": { "containers": [ { "image": "*:*" } ] } } } } ] } }`),
			areErrorsExpected: false,
		},
		{
			Description: "Should fail since resource does not match ",
			AdmissionInfo: kyverno.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{"apiVersion":"v1","kind":"Service","metadata":{"name":"hello-world","labels":{"name":"hello-world"}},"spec":{"containers":[{"name":"hello-world","image":"hello-world","ports":[{"containerPort":81}],"resources":{"limits":{"memory":"30Mi","cpu":"0.2"},"requests":{"memory":"20Mi","cpu":"0.1"}}}]}}`),
			Policy:            []byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"hello-world-policy"},"spec":{"background":false,"rules":[{"name":"hello-world-policy","match":{"resources":{"kinds":["Pod"]}},"exclude":{"resources":{"name":"hello-world"},"clusterRoles":["system:node"]},"mutate":{"overlay":{"spec":{"containers":[{"(image)":"*","imagePullPolicy":"IfNotPresent"}]}}}}]}}`),
			areErrorsExpected: true,
		},
		{
			Description: "Should fail since version not match",
			AdmissionInfo: kyverno.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{ "apiVersion": "apps/v1beta1", "kind": "Deployment", "metadata": { "creationTimestamp": "2020-09-21T12:56:35Z", "name": "qos-demo", "labels": { "test": "qos" } }, "spec": { "replicas": 1, "selector": { "matchLabels": { "app": "nginx" } }, "template": { "metadata": { "creationTimestamp": "2020-09-21T12:56:35Z", "labels": { "app": "nginx" } }, "spec": { "containers": [ { "name": "nginx", "image": "nginx:latest", "resources": { "limits": { "cpu": "50m" } } } ]}}}}`),
			Policy:            []byte(`{ "apiVersion": "kyverno.io/v1", "kind": "ClusterPolicy", "metadata": { "name": "policy-qos" }, "spec": { "validationFailureAction": "enforce", "rules": [ { "name": "add-memory-limit", "match": { "resources": { "kinds": [ "apps/v1/Deployment" ], "selector": { "matchLabels": { "test": "qos" } } } }, "mutate": { "overlay": { "spec": { "template": { "spec": { "containers": [ { "(name)": "*", "resources": { "limits": { "+(memory)": "300Mi", "+(cpu)": "100" } } } ] } } } } } }, { "name": "check-cpu-memory-limits", "match": { "resources": { "kinds": [ "apps/v1/Deployment" ], "selector": { "matchLabels": { "test": "qos" } } } }, "validate": { "message": "Resource limits are required for CPU and memory", "pattern": { "spec": { "template": { "spec": { "containers": [ { "(name)": "*", "resources": { "limits": { "memory": "?*", "cpu": "?*" } } } ] } } } } } } ] } }`),
			areErrorsExpected: true,
		},
		{
			Description: "Should fail since cluster role version not match",
			AdmissionInfo: kyverno.RequestInfo{
				ClusterRoles: []string{"admin"},
			},
			Resource:          []byte(`{ "kind": "ClusterRole", "apiVersion": "rbac.authorization.k8s.io/v1", "metadata": { "name": "secret-reader-demo", "namespace": "default" }, "rules": [ { "apiGroups": [ "" ], "resources": [ "secrets" ], "verbs": [ "get", "watch", "list" ] } ] }`),
			Policy:            []byte(`{ "apiVersion": "kyverno.io/v1", "kind": "ClusterPolicy", "metadata": { "name": "check-host-path" }, "spec": { "validationFailureAction": "enforce", "background": true, "rules": [ { "name": "check-host-path", "match": { "resources": { "kinds": [ "rbac.authorization.k8s.io/v1beta1/ClusterRole" ] } }, "validate": { "message": "Host path is not allowed", "pattern": { "spec": { "volumes": [ { "name": "*", "hostPath": { "path": "" } } ] } } } } ] } }`),
			areErrorsExpected: true,
		},
	}

	for i, tc := range tcs {
		var policy kyverno.Policy
		err := json.Unmarshal(tc.Policy, &policy)
		if err != nil {
			t.Errorf("Testcase %d invalid policy raw", i+1)
		}
		resource, _ := utils.ConvertToUnstructured(tc.Resource)

		for _, rule := range policy.Spec.Rules {
			err := MatchesResourceDescription(*resource, rule, tc.AdmissionInfo, []string{}, nil)
			if err != nil {
				if !tc.areErrorsExpected {
					t.Errorf("Testcase %d Unexpected error: %v", i+1, err)
				}
			} else {
				if tc.areErrorsExpected {
					t.Errorf("Testcase %d Expected Error but received no error", i+1)
				}
			}
		}
	}
}

// Match multiple kinds
func TestResourceDescriptionMatch_MultipleKind(t *testing.T) {
	rawResource := []byte(`{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
		   "name": "nginx-deployment",
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
					   "name": "nginx",
					   "image": "nginx:1.7.9",
					   "ports": [
						  {
							 "containerPort": 80
						  }
					   ]
					}
				 ]
			  }
		   }
		}
	 }`)
	resource, err := utils.ConvertToUnstructured(rawResource)
	if err != nil {
		t.Errorf("unable to convert raw resource to unstructured: %v", err)

	}
	resourceDescription := kyverno.ResourceDescription{
		Kinds: []string{"Deployment", "Pods"},
		Selector: &metav1.LabelSelector{
			MatchLabels:      nil,
			MatchExpressions: nil,
		},
	}
	rule := kyverno.Rule{MatchResources: kyverno.MatchResources{ResourceDescription: resourceDescription}}

	if err := MatchesResourceDescription(*resource, rule, kyverno.RequestInfo{}, []string{}, nil); err != nil {
		t.Errorf("Testcase has failed due to the following:%v", err)
	}

}

// Match resource name
func TestResourceDescriptionMatch_Name(t *testing.T) {
	rawResource := []byte(`{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
		   "name": "nginx-deployment",
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
					   "name": "nginx",
					   "image": "nginx:1.7.9",
					   "ports": [
						  {
							 "containerPort": 80
						  }
					   ]
					}
				 ]
			  }
		   }
		}
	 }`)
	resource, err := utils.ConvertToUnstructured(rawResource)
	if err != nil {
		t.Errorf("unable to convert raw resource to unstructured: %v", err)

	}
	resourceDescription := kyverno.ResourceDescription{
		Kinds: []string{"Deployment"},
		Name:  "nginx-deployment",
		Selector: &metav1.LabelSelector{
			MatchLabels:      nil,
			MatchExpressions: nil,
		},
	}
	rule := kyverno.Rule{MatchResources: kyverno.MatchResources{ResourceDescription: resourceDescription}}

	if err := MatchesResourceDescription(*resource, rule, kyverno.RequestInfo{}, []string{}, nil); err != nil {
		t.Errorf("Testcase has failed due to the following:%v", err)
	}
}

// Match resource regex
func TestResourceDescriptionMatch_Name_Regex(t *testing.T) {
	rawResource := []byte(`{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
		   "name": "nginx-deployment",
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
					   "name": "nginx",
					   "image": "nginx:1.7.9",
					   "ports": [
						  {
							 "containerPort": 80
						  }
					   ]
					}
				 ]
			  }
		   }
		}
	 }`)
	resource, err := utils.ConvertToUnstructured(rawResource)
	if err != nil {
		t.Errorf("unable to convert raw resource to unstructured: %v", err)

	}
	resourceDescription := kyverno.ResourceDescription{
		Kinds: []string{"Deployment"},
		Name:  "nginx-*",
		Selector: &metav1.LabelSelector{
			MatchLabels:      nil,
			MatchExpressions: nil,
		},
	}
	rule := kyverno.Rule{MatchResources: kyverno.MatchResources{ResourceDescription: resourceDescription}}

	if err := MatchesResourceDescription(*resource, rule, kyverno.RequestInfo{}, []string{}, nil); err != nil {
		t.Errorf("Testcase has failed due to the following:%v", err)
	}
}

// Match expressions for labels to not match
func TestResourceDescriptionMatch_Label_Expression_NotMatch(t *testing.T) {
	rawResource := []byte(`{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
		   "name": "nginx-deployment",
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
					   "name": "nginx",
					   "image": "nginx:1.7.9",
					   "ports": [
						  {
							 "containerPort": 80
						  }
					   ]
					}
				 ]
			  }
		   }
		}
	 }`)
	resource, err := utils.ConvertToUnstructured(rawResource)
	if err != nil {
		t.Errorf("unable to convert raw resource to unstructured: %v", err)

	}
	resourceDescription := kyverno.ResourceDescription{
		Kinds: []string{"Deployment"},
		Name:  "nginx-*",
		Selector: &metav1.LabelSelector{
			MatchLabels: nil,
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "label2",
					Operator: "NotIn",
					Values: []string{
						"sometest1",
					},
				},
			},
		},
	}
	rule := kyverno.Rule{MatchResources: kyverno.MatchResources{ResourceDescription: resourceDescription}}

	if err := MatchesResourceDescription(*resource, rule, kyverno.RequestInfo{}, []string{}, nil); err != nil {
		t.Errorf("Testcase has failed due to the following:%v", err)
	}
}

// Match label expression in matching set
func TestResourceDescriptionMatch_Label_Expression_Match(t *testing.T) {
	rawResource := []byte(`{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
		   "name": "nginx-deployment",
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
					   "name": "nginx",
					   "image": "nginx:1.7.9",
					   "ports": [
						  {
							 "containerPort": 80
						  }
					   ]
					}
				 ]
			  }
		   }
		}
	 }`)
	resource, err := utils.ConvertToUnstructured(rawResource)
	if err != nil {
		t.Errorf("unable to convert raw resource to unstructured: %v", err)

	}
	resourceDescription := kyverno.ResourceDescription{
		Kinds: []string{"Deployment"},
		Name:  "nginx-*",
		Selector: &metav1.LabelSelector{
			MatchLabels: nil,
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "app",
					Operator: "NotIn",
					Values: []string{
						"nginx1",
						"nginx2",
					},
				},
			},
		},
	}
	rule := kyverno.Rule{MatchResources: kyverno.MatchResources{ResourceDescription: resourceDescription}}

	if err := MatchesResourceDescription(*resource, rule, kyverno.RequestInfo{}, []string{}, nil); err != nil {
		t.Errorf("Testcase has failed due to the following:%v", err)
	}
}

// check for exclude conditions
func TestResourceDescriptionExclude_Label_Expression_Match(t *testing.T) {
	rawResource := []byte(`{
		"apiVersion": "apps/v1",
		"kind": "Deployment",
		"metadata": {
		   "name": "nginx-deployment",
		   "labels": {
			  "app": "nginx",
			  "block": "true"
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
					   "name": "nginx",
					   "image": "nginx:1.7.9",
					   "ports": [
						  {
							 "containerPort": 80
						  }
					   ]
					}
				 ]
			  }
		   }
		}
	 }`)
	resource, err := utils.ConvertToUnstructured(rawResource)
	if err != nil {
		t.Errorf("unable to convert raw resource to unstructured: %v", err)

	}
	resourceDescription := kyverno.ResourceDescription{
		Kinds: []string{"Deployment"},
		Name:  "nginx-*",
		Selector: &metav1.LabelSelector{
			MatchLabels: nil,
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "app",
					Operator: "NotIn",
					Values: []string{
						"nginx1",
						"nginx2",
					},
				},
			},
		},
	}

	resourceDescriptionExclude := kyverno.ResourceDescription{
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"block": "true",
			},
		},
	}

	rule := kyverno.Rule{MatchResources: kyverno.MatchResources{ResourceDescription: resourceDescription},
		ExcludeResources: kyverno.ExcludeResources{ResourceDescription: resourceDescriptionExclude}}

	if err := MatchesResourceDescription(*resource, rule, kyverno.RequestInfo{}, []string{}, nil); err == nil {
		t.Errorf("Testcase has failed due to the following:\n Function has returned no error, even though it was supposed to fail")
	}
}

func TestWildCardLabels(t *testing.T) {

	testSelector(t, &metav1.LabelSelector{}, map[string]string{}, true)

	testSelector(t, &metav1.LabelSelector{}, map[string]string{"foo": "bar"}, true)

	testSelector(t, &metav1.LabelSelector{MatchLabels: map[string]string{"test.io/*": "bar"}},
		map[string]string{"foo": "bar"}, false)

	testSelector(t, &metav1.LabelSelector{MatchLabels: map[string]string{"scale.test.io/*": "bar"}},
		map[string]string{"foo": "bar"}, false)

	testSelector(t, &metav1.LabelSelector{MatchLabels: map[string]string{"test.io/*": "bar"}},
		map[string]string{"test.io/scale": "foo", "test.io/functional": "bar"}, true)

	testSelector(t, &metav1.LabelSelector{MatchLabels: map[string]string{"test.io/*": "*"}},
		map[string]string{"test.io/scale": "foo", "test.io/functional": "bar"}, true)

	testSelector(t, &metav1.LabelSelector{MatchLabels: map[string]string{"test.io/*": "a*"}},
		map[string]string{"test.io/scale": "foo", "test.io/functional": "bar"}, false)

	testSelector(t, &metav1.LabelSelector{MatchLabels: map[string]string{"test.io/scale": "f??"}},
		map[string]string{"test.io/scale": "foo", "test.io/functional": "bar"}, true)

	testSelector(t, &metav1.LabelSelector{MatchLabels: map[string]string{"*": "*"}},
		map[string]string{"test.io/scale": "foo", "test.io/functional": "bar"}, true)

	testSelector(t, &metav1.LabelSelector{MatchLabels: map[string]string{"test.io/functional": "foo"}},
		map[string]string{"test.io/scale": "foo", "test.io/functional": "bar"}, false)

	testSelector(t, &metav1.LabelSelector{MatchLabels: map[string]string{"*": "*"}},
		map[string]string{}, false)
}

func testSelector(t *testing.T, s *metav1.LabelSelector, l map[string]string, match bool) {
	res, err := checkSelector(s, l)
	if err != nil {
		t.Errorf("selector %v failed to select labels %v: %v", s.MatchLabels, l, err)
		return
	}

	if res != match {
		t.Errorf("select %v -> labels %v: expected %v received %v", s.MatchLabels, l, match, res)
	}
}

func TestWildCardAnnotation(t *testing.T) {

	// test single annotation values
	testAnnotationMatch(t, map[string]string{}, map[string]string{}, true)
	testAnnotationMatch(t, map[string]string{"test/*": "*"}, map[string]string{}, false)
	testAnnotationMatch(t, map[string]string{"test/*": "*"}, map[string]string{"tes1/test": "*"}, false)
	testAnnotationMatch(t, map[string]string{"test/*": "*"}, map[string]string{"test/test": "*"}, true)
	testAnnotationMatch(t, map[string]string{"test/*": "*"}, map[string]string{"test/bar": "foo"}, true)
	testAnnotationMatch(t, map[string]string{"test/b*": "*"}, map[string]string{"test/bar": "foo"}, true)

	// test multiple annotation values
	testAnnotationMatch(t, map[string]string{"test/b*": "*", "test2/*": "*"},
		map[string]string{"test/bar": "foo"}, false)
	testAnnotationMatch(t, map[string]string{"test/b*": "*", "test2/*": "*"},
		map[string]string{"test/bar": "foo", "test2/123": "bar"}, true)
	testAnnotationMatch(t, map[string]string{"test/b*": "*", "test2/*": "*"},
		map[string]string{"test/bar": "foo", "test2/123": "bar", "test3/123": "bar2"}, true)
}

func testAnnotationMatch(t *testing.T, policy map[string]string, resource map[string]string, match bool) {
	res := checkAnnotations(policy, resource)
	if res != match {
		t.Errorf("annotations %v -> labels %v: expected %v received %v", policy, resource, match, res)
	}
}
