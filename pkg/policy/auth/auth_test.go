package auth

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// newCountingFakeClient returns a dclient.Interface backed by a fake kube
// clientset whose SubjectAccessReview creations are counted, so tests can
// assert how many times the API server was actually contacted.
func newCountingFakeClient(t *testing.T) (dclient.Interface, *atomic.Int64) {
	t.Helper()
	var calls atomic.Int64

	kubeClient := kubefake.NewSimpleClientset()
	kubeClient.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
		calls.Add(1)
		return true, &authorizationv1.SubjectAccessReview{
			Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true},
		}, nil
	})

	dynClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	disco := dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{{Version: "v1", Resource: "pods"}})
	client := dclient.NewFakeClientWithDisco(dynClient, kubeClient, disco)
	return client, &calls
}

func TestAuth_CanI_CacheDedupesAccessReviews(t *testing.T) {
	client, calls := newCountingFakeClient(t)
	cache := NewCache()

	// Simulate several rules in the same policy all checking the same
	// (user, verb, kind) tuple, as happens when multiple rules match the
	// same resource kind - each rule gets its own Auth instance, but they
	// share the Cache the way validateActions does across a policy's rules.
	for i := 0; i < 5; i++ {
		a := NewAuth(client, "system:serviceaccount:kyverno:reports-controller", logr.Discard(), cache)
		ok, msg, err := a.CanI(context.Background(), []string{"get", "list", "watch"}, "Pod", "", "", "")
		assert.NoError(t, err)
		assert.True(t, ok)
		assert.Empty(t, msg)
	}

	// 3 verbs checked once each and then reused from cache for the other 4 rules.
	assert.Equal(t, int64(3), calls.Load())
}

func TestAuth_CanI_NoCacheRepeatsAccessReviews(t *testing.T) {
	client, calls := newCountingFakeClient(t)

	for i := 0; i < 5; i++ {
		a := NewAuth(client, "system:serviceaccount:kyverno:reports-controller", logr.Discard(), nil)
		ok, _, err := a.CanI(context.Background(), []string{"get", "list", "watch"}, "Pod", "", "", "")
		assert.NoError(t, err)
		assert.True(t, ok)
	}

	// without a cache, every rule repeats all 3 verb checks
	assert.Equal(t, int64(15), calls.Load())
}

func TestNewAuth_DifferentUsers(t *testing.T) {
	tests := []struct {
		name string
		user string
	}{
		{
			name: "non-empty user",
			user: "test-user",
		},
		{
			name: "empty user",
			user: "",
		},
		{
			name: "admin user",
			user: "admin",
		},
		{
			name: "service account",
			user: "system:serviceaccount:default:test",
		},
		{
			name: "anonymous user",
			user: "system:anonymous",
		},
		{
			name: "email user",
			user: "developer@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := NewAuth(nil, tt.user, logr.Discard(), nil)

			assert.NotNil(t, auth)
			assert.Equal(t, tt.user, auth.user)
			assert.Equal(t, tt.user, auth.User())
			assert.Nil(t, auth.client)
		})
	}
}

func TestBuildMessage_Scenarios(t *testing.T) {
	tests := []struct {
		name        string
		gvk         string
		subresource string
		verbs       []string
		user        string
		namespace   string
		wantParts   []string
	}{
		{
			name:        "basic case",
			gvk:         "pods",
			subresource: "",
			verbs:       []string{"create"},
			user:        "test-user",
			namespace:   "",
			wantParts:   []string{"test-user", "create", "pods"},
		},
		{
			name:        "with subresource",
			gvk:         "pods",
			subresource: "status",
			verbs:       []string{"update"},
			user:        "test-user",
			namespace:   "",
			wantParts:   []string{"pods/status", "update"},
		},
		{
			name:        "with namespace",
			gvk:         "pods",
			subresource: "",
			verbs:       []string{"get"},
			user:        "test-user",
			namespace:   "default",
			wantParts:   []string{"default", "test-user"},
		},
		{
			name:        "multiple verbs",
			gvk:         "pods",
			subresource: "",
			verbs:       []string{"create", "delete", "update"},
			user:        "test-user",
			namespace:   "",
			wantParts:   []string{"create", "delete", "update"},
		},
		{
			name:        "full case",
			gvk:         "deployments",
			subresource: "scale",
			verbs:       []string{"get", "update"},
			user:        "system:serviceaccount:kyverno:kyverno",
			namespace:   "production",
			wantParts:   []string{"deployments/scale", "get", "update", "system:serviceaccount:kyverno:kyverno", "production"},
		},
		{
			name:        "empty verbs",
			gvk:         "pods",
			subresource: "",
			verbs:       []string{},
			user:        "test-user",
			namespace:   "",
			wantParts:   []string{"test-user", "pods"},
		},
		{
			name:        "cluster-scoped resource",
			gvk:         "namespaces",
			subresource: "",
			verbs:       []string{"create"},
			user:        "admin",
			namespace:   "",
			wantParts:   []string{"admin", "create", "namespaces"},
		},
		{
			name:        "namespaced resource",
			gvk:         "configmaps",
			subresource: "",
			verbs:       []string{"get", "list"},
			user:        "developer",
			namespace:   "dev",
			wantParts:   []string{"developer", "get", "list", "configmaps", "dev"},
		},
		{
			name:        "with subresource",
			gvk:         "pods",
			subresource: "log",
			verbs:       []string{"get"},
			user:        "viewer",
			namespace:   "monitoring",
			wantParts:   []string{"viewer", "get", "pods/log", "monitoring"},
		},
		{
			name:        "service account user",
			gvk:         "secrets",
			subresource: "",
			verbs:       []string{"get", "watch"},
			user:        "system:serviceaccount:default:my-sa",
			namespace:   "default",
			wantParts:   []string{"system:serviceaccount:default:my-sa", "get", "watch", "secrets", "default"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := buildMessage(tt.gvk, tt.subresource, tt.verbs, tt.user, tt.namespace)

			for _, part := range tt.wantParts {
				assert.Contains(t, msg, part)
			}
		})
	}
}
