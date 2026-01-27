package utils

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestCheckMatchesResources_EmptyStatement(t *testing.T) {
	result := checkMatchesResources(unstructured.Unstructured{}, kyvernov2beta1.MatchResources{}, nil, kyvernov2.RequestInfo{}, schema.GroupVersionKind{}, "")
	assert.False(t, result)
}

func TestCheckMatchesResources_AnyMatch(t *testing.T) {
	statement := kyvernov2beta1.MatchResources{
		Any: []kyvernov1.ResourceFilter{{
			ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"Pod"}},
		}},
	}
	gvk := schema.GroupVersionKind{Kind: "Pod"}
	result := checkMatchesResources(unstructured.Unstructured{}, statement, nil, kyvernov2.RequestInfo{}, gvk, "")
	assert.True(t, result)
}

func TestCheckMatchesResources_AllMatch(t *testing.T) {
	statement := kyvernov2beta1.MatchResources{
		All: []kyvernov1.ResourceFilter{{
			ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"Pod"}},
		}},
	}
	gvk := schema.GroupVersionKind{Kind: "Pod"}
	result := checkMatchesResources(unstructured.Unstructured{}, statement, nil, kyvernov2.RequestInfo{}, gvk, "")
	assert.True(t, result)
}

func TestCheckUserInfo_RolesMatch(t *testing.T) {
	userInfo := kyvernov1.UserInfo{Roles: []string{"admin"}}
	admissionInfo := kyvernov2.RequestInfo{Roles: []string{"admin", "user"}}
	assert.True(t, checkUserInfo(userInfo, admissionInfo))
}

func TestCheckUserInfo_RolesNoMatch(t *testing.T) {
	userInfo := kyvernov1.UserInfo{Roles: []string{"superadmin"}}
	admissionInfo := kyvernov2.RequestInfo{Roles: []string{"admin"}}
	assert.False(t, checkUserInfo(userInfo, admissionInfo))
}
