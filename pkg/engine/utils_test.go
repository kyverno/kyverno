package engine

import (
	"encoding/json"
	"testing"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/utils"
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
	}

	for i, tc := range tcs {
		var policy kyverno.Policy
		err := json.Unmarshal(tc.Policy, &policy)
		if err != nil {
			t.Errorf("Testcase %d invalid policy raw", i+1)
		}
		resource, _ := utils.ConvertToUnstructured(tc.Resource)

		for _, rule := range policy.Spec.Rules {
			err := MatchesResourceDescription(*resource, rule, tc.AdmissionInfo, []string{})
			if err != nil {
				if !tc.areErrorsExpected {
					t.Errorf("Testcase %d Unexpected error: %v", i+1, err)
				}
			} else {
				if tc.areErrorsExpected {
					t.Errorf("Testcase %d Expected Error but recieved no error", i+1)
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

	if err := MatchesResourceDescription(*resource, rule, kyverno.RequestInfo{}, []string{}); err != nil {
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

	if err := MatchesResourceDescription(*resource, rule, kyverno.RequestInfo{}, []string{}); err != nil {
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

	if err := MatchesResourceDescription(*resource, rule, kyverno.RequestInfo{}, []string{}); err != nil {
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

	if err := MatchesResourceDescription(*resource, rule, kyverno.RequestInfo{}, []string{}); err != nil {
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

	if err := MatchesResourceDescription(*resource, rule, kyverno.RequestInfo{}, []string{}); err != nil {
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

	if err := MatchesResourceDescription(*resource, rule, kyverno.RequestInfo{}, []string{}); err == nil {
		t.Errorf("Testcase has failed due to the following:\n Function has returned no error, even though it was suposed to fail")
	}
}
