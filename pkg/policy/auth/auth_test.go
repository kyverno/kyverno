package auth

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

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
			auth := NewAuth(nil, tt.user, logr.Discard())

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
