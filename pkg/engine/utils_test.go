package engine

import (
	"testing"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/utils"
	"gotest.tools/assert"
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

	assert.Assert(t, MatchesResourceDescription(*resource, rule))
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

	assert.Assert(t, MatchesResourceDescription(*resource, rule))
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

	assert.Assert(t, MatchesResourceDescription(*resource, rule))
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

	assert.Assert(t, MatchesResourceDescription(*resource, rule))
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

	assert.Assert(t, MatchesResourceDescription(*resource, rule))
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

	assert.Assert(t, !MatchesResourceDescription(*resource, rule))
}

// func Test_validateGeneralRuleInfoVariables(t *testing.T) {
// 	policyRaw := []byte(`{
// 		"apiVersion": "kyverno.io/v1",
// 		"kind": "ClusterPolicy",
// 		"metadata": {
// 		  "name": "test-validate-variables"
// 		},
// 		"spec": {
// 		  "rules": [
// 			{
// 			  "name": "test-match",
// 			  "match": {
// 				"Subjects": [
// 				  {
// 					"kind": "User",
// 					"name": "{{request.userInfo.username1}}}"
// 				  }
// 				]
// 			  }
// 			}
// 		  ]
// 		}
// 	  }`)

// 	userReqInfo := kyverno.RequestInfo{
// 		AdmissionUserInfo: authenticationv1.UserInfo{
// 			Username: "user1",
// 		},
// 	}

// 	var policy interface{}
// 	assert.NilError(t, json.Unmarshal(policyRaw, &policy))

// 	ctx := context.NewContext()
// 	// ctx.AddResource(resourceRaw)
// 	ctx.AddUserInfo(userReqInfo)

// 	invalidPaths := validateGeneralRuleInfoVariables(ctx, policy.Spec.Rules[0])
// 	assert.Assert(t, len(invalidPaths) == 1, fmt.Sprintf("got path len = %d", len(invalidPaths)))
// }
