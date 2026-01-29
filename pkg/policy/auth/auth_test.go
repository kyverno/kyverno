package auth

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
)

func TestNewAuth(t *testing.T) {
	auth := NewAuth(nil, "test-user", logr.Discard())

	assert.NotNil(t, auth)
	assert.Equal(t, "test-user", auth.user)
}

func TestNewAuth_EmptyUser(t *testing.T) {
	auth := NewAuth(nil, "", logr.Discard())

	assert.NotNil(t, auth)
	assert.Empty(t, auth.user)
}

func TestUser(t *testing.T) {
	auth := NewAuth(nil, "test-user", logr.Discard())

	user := auth.User()

	assert.Equal(t, "test-user", user)
}

func TestUser_EmptyUser(t *testing.T) {
	auth := NewAuth(nil, "", logr.Discard())

	user := auth.User()

	assert.Empty(t, user)
}

func TestBuildMessage_BasicCase(t *testing.T) {
	msg := buildMessage("pods", "", []string{"create"}, "test-user", "")

	assert.Contains(t, msg, "test-user")
	assert.Contains(t, msg, "create")
	assert.Contains(t, msg, "pods")
}

func TestBuildMessage_WithSubresource(t *testing.T) {
	msg := buildMessage("pods", "status", []string{"update"}, "test-user", "")

	assert.Contains(t, msg, "pods/status")
	assert.Contains(t, msg, "update")
}

func TestBuildMessage_WithNamespace(t *testing.T) {
	msg := buildMessage("pods", "", []string{"get"}, "test-user", "default")

	assert.Contains(t, msg, "default")
	assert.Contains(t, msg, "test-user")
}

func TestBuildMessage_MultipleVerbs(t *testing.T) {
	msg := buildMessage("pods", "", []string{"create", "delete", "update"}, "test-user", "")

	assert.Contains(t, msg, "create")
	assert.Contains(t, msg, "delete")
	assert.Contains(t, msg, "update")
}

func TestBuildMessage_FullCase(t *testing.T) {
	msg := buildMessage("deployments", "scale", []string{"get", "update"}, "system:serviceaccount:kyverno:kyverno", "production")

	assert.Contains(t, msg, "deployments/scale")
	assert.Contains(t, msg, "get")
	assert.Contains(t, msg, "update")
	assert.Contains(t, msg, "system:serviceaccount:kyverno:kyverno")
	assert.Contains(t, msg, "production")
}

func TestBuildMessage_EmptyVerbs(t *testing.T) {
	msg := buildMessage("pods", "", []string{}, "test-user", "")

	// Should still produce a message even with empty verbs
	assert.Contains(t, msg, "test-user")
	assert.Contains(t, msg, "pods")
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

func TestAuthChecksInterface(t *testing.T) {
	// Verify Auth implements AuthChecks interface
	var _ AuthChecks = &Auth{}
}

func TestNewAuth_NilClient(t *testing.T) {
	auth := NewAuth(nil, "test-user", logr.Discard())

	assert.NotNil(t, auth)
	assert.Nil(t, auth.client)
}

func TestNewAuth_DifferentUsers(t *testing.T) {
	users := []string{
		"admin",
		"system:serviceaccount:default:test",
		"system:anonymous",
		"developer@example.com",
	}

	for _, user := range users {
		t.Run(user, func(t *testing.T) {
			auth := NewAuth(nil, user, logr.Discard())

			assert.NotNil(t, auth)
			assert.Equal(t, user, auth.User())
		})
	}
}
