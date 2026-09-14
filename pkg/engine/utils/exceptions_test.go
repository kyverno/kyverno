package utils

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
	"github.com/stretchr/testify/assert"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// countingClient records GetNamespace calls; the other engineapi.Client methods
// are inherited from the embedded nil interface and panic if called.
type countingClient struct {
	engineapi.Client
	getNamespaceCalls int
	namespaceLabels   map[string]string
	err               error
}

func (c *countingClient) GetNamespace(_ context.Context, name string, _ metav1.GetOptions) (*corev1.Namespace, error) {
	c.getNamespaceCalls++
	if c.err != nil {
		return nil, c.err
	}
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: c.namespaceLabels},
	}, nil
}

func newExceptionTestPolicyContext(t *testing.T, resource unstructured.Unstructured) engineapi.PolicyContext {
	t.Helper()
	cfg := config.NewDefaultConfiguration(false)
	jp := jmespath.New(cfg)
	pc, err := policycontext.NewPolicyContext(jp, resource, kyvernov1.Create, nil, cfg)
	assert.NoError(t, err)
	return pc.WithNewResource(resource)
}

func newPodResource(name, namespace string) unstructured.Unstructured {
	resource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
		},
	}
	resource.SetName(name)
	resource.SetNamespace(namespace)
	return resource
}

func polexMatchingNamespaces(namespaces ...string) *kyvernov2.PolicyException {
	return &kyvernov2.PolicyException{
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{
				{PolicyName: "disallow-capabilities", RuleNames: []string{"*"}},
			},
			Match: kyvernov2beta1.MatchResources{
				Any: kyvernov1.ResourceFilters{
					{
						ResourceDescription: kyvernov1.ResourceDescription{
							Namespaces: namespaces,
						},
					},
				},
			},
		},
	}
}

func polexMatchingNamespaceSelector(matchLabels map[string]string) *kyvernov2.PolicyException {
	return &kyvernov2.PolicyException{
		Spec: kyvernov2.PolicyExceptionSpec{
			Exceptions: []kyvernov2.Exception{
				{PolicyName: "disallow-capabilities", RuleNames: []string{"*"}},
			},
			Match: kyvernov2beta1.MatchResources{
				Any: kyvernov1.ResourceFilters{
					{
						ResourceDescription: kyvernov1.ResourceDescription{
							NamespaceSelector: &metav1.LabelSelector{MatchLabels: matchLabels},
						},
					},
				},
			},
		},
	}
}

func TestMatchesException_NoNamespaceSelector_SkipsNamespaceGet(t *testing.T) {
	client := &countingClient{}
	polexs := []*kyvernov2.PolicyException{polexMatchingNamespaces("kube-system")}

	matched := MatchesException(client, polexs, newExceptionTestPolicyContext(t, newPodResource("test-pod", "kube-system")), true, logr.Discard())
	assert.Len(t, matched, 1, "exception matching by namespace list should still match")
	assert.Equal(t, 0, client.getNamespaceCalls, "no exception defines a namespaceSelector, namespace labels must not be fetched")

	matched = MatchesException(client, polexs, newExceptionTestPolicyContext(t, newPodResource("test-pod", "default")), true, logr.Discard())
	assert.Empty(t, matched, "exception should not match a namespace outside the list")
	assert.Equal(t, 0, client.getNamespaceCalls, "no exception defines a namespaceSelector, namespace labels must not be fetched")
}

func TestMatchesException_NamespaceSelector_FetchesNamespaceLabels(t *testing.T) {
	client := &countingClient{namespaceLabels: map[string]string{"env": "test"}}
	polexs := []*kyvernov2.PolicyException{polexMatchingNamespaceSelector(map[string]string{"env": "test"})}

	matched := MatchesException(client, polexs, newExceptionTestPolicyContext(t, newPodResource("test-pod", "default")), true, logr.Discard())
	assert.Len(t, matched, 1, "exception should match via namespaceSelector")
	assert.Equal(t, 1, client.getNamespaceCalls, "namespace labels must be fetched when a namespaceSelector is defined")

	client = &countingClient{namespaceLabels: map[string]string{"env": "prod"}}
	matched = MatchesException(client, polexs, newExceptionTestPolicyContext(t, newPodResource("test-pod", "default")), true, logr.Discard())
	assert.Empty(t, matched, "exception should not match when namespace labels do not satisfy the selector")
	assert.Equal(t, 1, client.getNamespaceCalls)
}

func TestMatchesException_NamespaceSelector_GetNamespaceError(t *testing.T) {
	client := &countingClient{err: errors.New("connection refused")}
	polexs := []*kyvernov2.PolicyException{polexMatchingNamespaceSelector(map[string]string{"env": "test"})}

	matched := MatchesException(client, polexs, newExceptionTestPolicyContext(t, newPodResource("test-pod", "default")), true, logr.Discard())
	assert.Nil(t, matched, "no exceptions should match when the namespace cannot be fetched")
	assert.Equal(t, 1, client.getNamespaceCalls)
}

func TestMatchesException_ClusterScopedResource_SkipsNamespaceGet(t *testing.T) {
	client := &countingClient{}
	polexs := []*kyvernov2.PolicyException{polexMatchingNamespaceSelector(map[string]string{"env": "test"})}
	resource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "ClusterRole",
		},
	}
	resource.SetName("test-cluster-role")

	matched := MatchesException(client, polexs, newExceptionTestPolicyContext(t, resource), true, logr.Discard())
	assert.Empty(t, matched)
	assert.Equal(t, 0, client.getNamespaceCalls, "cluster-scoped resources have no namespace to fetch")
}
