package utils

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/stretchr/testify/assert"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestCheckUserInfo_EmptyUserInfo(t *testing.T) {
	userInfo := kyvernov1.UserInfo{}
	admissionInfo := kyvernov2.RequestInfo{}

	result := checkUserInfo(userInfo, admissionInfo)
	assert.True(t, result, "empty user info should match")
}

func TestCheckUserInfo_MatchingRoles(t *testing.T) {
	userInfo := kyvernov1.UserInfo{
		Roles: []string{"admin", "developer"},
	}
	admissionInfo := kyvernov2.RequestInfo{
		Roles: []string{"admin"},
	}

	result := checkUserInfo(userInfo, admissionInfo)
	assert.True(t, result, "should match when user has required role")
}

func TestCheckUserInfo_NonMatchingRoles(t *testing.T) {
	userInfo := kyvernov1.UserInfo{
		Roles: []string{"admin"},
	}
	admissionInfo := kyvernov2.RequestInfo{
		Roles: []string{"viewer"},
	}

	result := checkUserInfo(userInfo, admissionInfo)
	assert.False(t, result, "should not match when user lacks required role")
}

func TestCheckUserInfo_MatchingClusterRoles(t *testing.T) {
	userInfo := kyvernov1.UserInfo{
		ClusterRoles: []string{"cluster-admin"},
	}
	admissionInfo := kyvernov2.RequestInfo{
		ClusterRoles: []string{"cluster-admin"},
	}

	result := checkUserInfo(userInfo, admissionInfo)
	assert.True(t, result, "should match when user has required cluster role")
}

func TestCheckUserInfo_NonMatchingClusterRoles(t *testing.T) {
	userInfo := kyvernov1.UserInfo{
		ClusterRoles: []string{"cluster-admin"},
	}
	admissionInfo := kyvernov2.RequestInfo{
		ClusterRoles: []string{"viewer"},
	}

	result := checkUserInfo(userInfo, admissionInfo)
	assert.False(t, result, "should not match when user lacks required cluster role")
}

func TestCheckResourceDescription_EmptyConditionBlock(t *testing.T) {
	conditionBlock := kyvernov1.ResourceDescription{}
	resource := unstructured.Unstructured{}
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}

	result := checkResourceDescription(conditionBlock, resource, nil, gvk, "")
	assert.True(t, result, "empty condition block should match")
}

func TestCheckResourceDescription_MatchingKind(t *testing.T) {
	conditionBlock := kyvernov1.ResourceDescription{
		Kinds: []string{"Pod"},
	}
	resource := unstructured.Unstructured{}
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}

	result := checkResourceDescription(conditionBlock, resource, nil, gvk, "")
	assert.True(t, result, "should match when kind matches")
}

func TestCheckResourceDescription_NonMatchingKind(t *testing.T) {
	conditionBlock := kyvernov1.ResourceDescription{
		Kinds: []string{"Deployment"},
	}
	resource := unstructured.Unstructured{}
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}

	result := checkResourceDescription(conditionBlock, resource, nil, gvk, "")
	assert.False(t, result, "should not match when kind differs")
}

func TestCheckResourceDescription_MatchingName(t *testing.T) {
	conditionBlock := kyvernov1.ResourceDescription{
		Name: "test-*",
	}
	resource := unstructured.Unstructured{}
	resource.SetName("test-pod")
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}

	result := checkResourceDescription(conditionBlock, resource, nil, gvk, "")
	assert.True(t, result, "should match when name matches wildcard")
}

func TestCheckResourceDescription_NonMatchingName(t *testing.T) {
	conditionBlock := kyvernov1.ResourceDescription{
		Name: "test-*",
	}
	resource := unstructured.Unstructured{}
	resource.SetName("prod-pod")
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}

	result := checkResourceDescription(conditionBlock, resource, nil, gvk, "")
	assert.False(t, result, "should not match when name doesn't match wildcard")
}

func TestCheckResourceDescription_MatchingNamespace(t *testing.T) {
	conditionBlock := kyvernov1.ResourceDescription{
		Namespaces: []string{"default", "kube-system"},
	}
	resource := unstructured.Unstructured{}
	resource.SetNamespace("default")
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}

	result := checkResourceDescription(conditionBlock, resource, nil, gvk, "")
	assert.True(t, result, "should match when namespace is in list")
}

func TestCheckResourceDescription_NonMatchingNamespace(t *testing.T) {
	conditionBlock := kyvernov1.ResourceDescription{
		Namespaces: []string{"production"},
	}
	resource := unstructured.Unstructured{}
	resource.SetNamespace("default")
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}

	result := checkResourceDescription(conditionBlock, resource, nil, gvk, "")
	assert.False(t, result, "should not match when namespace is not in list")
}

func TestCheckResourceFilter_EmptyStatement(t *testing.T) {
	statement := kyvernov1.ResourceFilter{}
	resource := unstructured.Unstructured{}
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
	admissionInfo := kyvernov2.RequestInfo{}

	result := checkResourceFilter(statement, resource, nil, admissionInfo, gvk, "")
	assert.False(t, result, "empty statement should not match")
}

func TestCheckMatchesResources_EmptyStatement(t *testing.T) {
	statement := kyvernov2beta1.MatchResources{}
	resource := unstructured.Unstructured{}
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
	admissionInfo := kyvernov2.RequestInfo{}

	result := checkMatchesResources(resource, statement, nil, admissionInfo, gvk, "")
	assert.False(t, result, "empty match statement should not match")
}

func TestCheckMatchesResources_AnyMatches(t *testing.T) {
	statement := kyvernov2beta1.MatchResources{
		Any: kyvernov1.ResourceFilters{
			{
				ResourceDescription: kyvernov1.ResourceDescription{
					Kinds: []string{"Pod"},
				},
			},
		},
	}
	resource := unstructured.Unstructured{}
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
	admissionInfo := kyvernov2.RequestInfo{}

	result := checkMatchesResources(resource, statement, nil, admissionInfo, gvk, "")
	assert.True(t, result, "should match when any filter matches")
}

func TestCheckMatchesResources_AllMatchesFail(t *testing.T) {
	statement := kyvernov2beta1.MatchResources{
		All: kyvernov1.ResourceFilters{
			{
				ResourceDescription: kyvernov1.ResourceDescription{
					Kinds: []string{"Deployment"},
				},
			},
		},
	}
	resource := unstructured.Unstructured{}
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
	admissionInfo := kyvernov2.RequestInfo{}

	result := checkMatchesResources(resource, statement, nil, admissionInfo, gvk, "")
	assert.False(t, result, "should not match when all filter doesn't match")
}

func TestCheckUserInfo_MatchingSubjects(t *testing.T) {
	userInfo := kyvernov1.UserInfo{
		Subjects: []rbacv1.Subject{
			{Kind: "User", Name: "admin@example.com"},
		},
	}
	admissionInfo := kyvernov2.RequestInfo{
		AdmissionUserInfo: authenticationv1.UserInfo{
			Username: "admin@example.com",
		},
	}

	result := checkUserInfo(userInfo, admissionInfo)
	assert.True(t, result, "should match when subject matches")
}
