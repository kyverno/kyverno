package engine

import (
	"encoding/json"
	"fmt"
	"testing"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	context "github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/utils"
	"gotest.tools/assert"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

	if err := MatchesResourceDescription(*resource, rule, kyverno.RequestInfo{}); err != nil {
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

	if err := MatchesResourceDescription(*resource, rule, kyverno.RequestInfo{}); err != nil {
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

	if err := MatchesResourceDescription(*resource, rule, kyverno.RequestInfo{}); err != nil {
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
				metav1.LabelSelectorRequirement{
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

	if err := MatchesResourceDescription(*resource, rule, kyverno.RequestInfo{}); err != nil {
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
				metav1.LabelSelectorRequirement{
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

	if err := MatchesResourceDescription(*resource, rule, kyverno.RequestInfo{}); err != nil {
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
				metav1.LabelSelectorRequirement{
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

	if err := MatchesResourceDescription(*resource, rule, kyverno.RequestInfo{}); err == nil {
		t.Errorf("Testcase has failed due to the following:\n Function has returned no error, even though it was suposed to fail")
	}
}

func Test_validateGeneralRuleInfoVariables(t *testing.T) {
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

	policyRaw := []byte(`{
		"apiVersion": "kyverno.io/v1",
		"kind": "ClusterPolicy",
		"metadata": {
		  "name": "test-validate-variables"
		},
		"spec": {
		  "rules": [
			{
			  "name": "test-match",
			  "match": {
				"Subjects": [
				  {
					"kind": "User",
					"name": "{{request.userInfo.username1}}}"
				  }
				],
				"resources": {
				  "kind": "{{request.object.kind}}"
				}
			  }
			},
			{
			  "name": "test-exclude",
			  "match": {
				"resources": {
				  "namespaces": [
					"{{request.object.namespace}}"
				  ]
				}
			  }
			},
			{
			  "name": "test-condition",
			  "preconditions": [
				{
				  "key": "{{serviceAccountName}}",
				  "operator": "NotEqual",
				  "value": "testuser"
				}
			  ]
			}
		  ]
		}
	  }`)

	userReqInfo := kyverno.RequestInfo{
		AdmissionUserInfo: authenticationv1.UserInfo{
			Username: "user1",
		},
	}

	var policy kyverno.ClusterPolicy
	assert.NilError(t, json.Unmarshal(policyRaw, &policy))

	ctx := context.NewContext()
	var err error
	err = ctx.AddResource(rawResource)
	if err != nil {
		t.Error(err)
	}
	err = ctx.AddUserInfo(userReqInfo)
	if err != nil {
		t.Error(err)
	}
	err = ctx.AddSA("system:serviceaccount:test:testuser")
	if err != nil {
		t.Error(err)
	}

	expectPaths := []string{"request.userInfo.username1", "request.object.namespace", ""}

	for i, rule := range policy.Spec.Rules {
		invalidPaths := validateGeneralRuleInfoVariables(ctx, rule)
		assert.Assert(t, invalidPaths == expectPaths[i], fmt.Sprintf("result not match, got invalidPaths %s", invalidPaths))
	}
}
