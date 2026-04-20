package admissionpolicy

import (
	"testing"

	authenticationv1 "k8s.io/api/authentication/v1"
)

func TestUserInfo_GetName(t *testing.T) {
	user := NewUser(authenticationv1.UserInfo{Username: "test-user"})
	if got := user.GetName(); got != "test-user" {
		t.Errorf("GetName() = %v, want test-user", got)
	}
}

func TestUserInfo_GetUID(t *testing.T) {
	user := NewUser(authenticationv1.UserInfo{UID: "uid-123"})
	if got := user.GetUID(); got != "uid-123" {
		t.Errorf("GetUID() = %v, want uid-123", got)
	}
}

func TestUserInfo_GetGroups(t *testing.T) {
	groups := []string{"group1", "group2", "system:authenticated"}
	user := NewUser(authenticationv1.UserInfo{Groups: groups})
	got := user.GetGroups()

	if len(got) != 3 {
		t.Errorf("GetGroups() len = %v, want 3", len(got))
	}
	if got[0] != "group1" {
		t.Errorf("GetGroups()[0] = %v, want group1", got[0])
	}
}

func TestUserInfo_GetExtra(t *testing.T) {
	extra := map[string]authenticationv1.ExtraValue{
		"key1": {"val1", "val2"},
		"key2": {"val3"},
	}
	user := NewUser(authenticationv1.UserInfo{Extra: extra})
	got := user.GetExtra()

	if len(got) != 2 {
		t.Errorf("GetExtra() len = %v, want 2", len(got))
	}
	if len(got["key1"]) != 2 {
		t.Errorf("GetExtra()[key1] len = %v, want 2", len(got["key1"]))
	}
}

func TestUserInfo_EmptyExtra(t *testing.T) {
	user := NewUser(authenticationv1.UserInfo{})
	got := user.GetExtra()

	if got == nil {
		t.Error("GetExtra() should return non-nil map")
	}
	if len(got) != 0 {
		t.Errorf("GetExtra() len = %v, want 0", len(got))
	}
}

func TestNewUser(t *testing.T) {
	info := authenticationv1.UserInfo{
		Username: "admin",
		UID:      "admin-uid",
		Groups:   []string{"admins"},
	}
	user := NewUser(info)

	if user.GetName() != "admin" {
		t.Error("NewUser did not properly set username")
	}
}
