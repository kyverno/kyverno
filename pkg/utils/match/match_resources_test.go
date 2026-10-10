package match

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestCheckMatchesResources_Any(t *testing.T) {
	resource := unstructured.Unstructured{}
	resource.SetKind("Pod")
	resource.SetName("test-pod")
	resource.SetNamespace("test-ns")
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
	statement := kyvernov2beta1.MatchResources{
		Any: kyvernov1.ResourceFilters{
			kyvernov1.ResourceFilter{
				ResourceDescription: kyvernov1.ResourceDescription{
					Kinds: []string{"Pod"},
					Names: []string{"test-pod"},
				},
			},
		},
	}
	err := CheckMatchesResources(resource, statement, nil, kyvernov2.RequestInfo{}, gvk, "")
	assert.NoError(t, err)
	statementFail := kyvernov2beta1.MatchResources{
		Any: kyvernov1.ResourceFilters{
			kyvernov1.ResourceFilter{
				ResourceDescription: kyvernov1.ResourceDescription{
					Kinds: []string{"Deployment"},
				},
			},
		},
	}
	err = CheckMatchesResources(resource, statementFail, nil, kyvernov2.RequestInfo{}, gvk, "")
	assert.Error(t, err)
}

func TestCheckMatchesResources_All(t *testing.T) {
	resource := unstructured.Unstructured{}
	resource.SetKind("Pod")
	resource.SetName("test-pod")
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
	statement := kyvernov2beta1.MatchResources{
		All: kyvernov1.ResourceFilters{
			kyvernov1.ResourceFilter{
				ResourceDescription: kyvernov1.ResourceDescription{
					Kinds: []string{"Pod"},
				},
			},
			kyvernov1.ResourceFilter{
				ResourceDescription: kyvernov1.ResourceDescription{
					Names: []string{"test-pod"},
				},
			},
		},
	}
	err := CheckMatchesResources(resource, statement, nil, kyvernov2.RequestInfo{}, gvk, "")
	assert.NoError(t, err)
	statementFail := kyvernov2beta1.MatchResources{
		All: kyvernov1.ResourceFilters{
			kyvernov1.ResourceFilter{
				ResourceDescription: kyvernov1.ResourceDescription{
					Kinds: []string{"Pod"},
				},
			},
			kyvernov1.ResourceFilter{
				ResourceDescription: kyvernov1.ResourceDescription{
					Names: []string{"other-pod"},
				},
			},
		},
	}
	err = CheckMatchesResources(resource, statementFail, nil, kyvernov2.RequestInfo{}, gvk, "")
	assert.Error(t, err)
}

func TestCheckUserInfo(t *testing.T) {
	userInfo := kyvernov1.UserInfo{
		Roles:        []string{"admin"},
		ClusterRoles: []string{"cluster-admin"},
	}
	admissionInfo := kyvernov2.RequestInfo{
		Roles:        []string{"admin", "user"},
		ClusterRoles: []string{"cluster-admin"},
	}
	errs := checkUserInfo(userInfo, admissionInfo)
	assert.Len(t, errs, 0)

	userInfoFail := kyvernov1.UserInfo{
		Roles: []string{"superadmin"},
	}
	errs = checkUserInfo(userInfoFail, admissionInfo)
	assert.Len(t, errs, 1)
}
