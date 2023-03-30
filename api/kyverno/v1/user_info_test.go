package v1

import (
	"testing"

	"gotest.tools/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_Validate_ServiceAccount(t *testing.T) {
	subject := UserInfo{
		Subjects: []rbacv1.Subject{{
			Kind: "ServiceAccount",
			Name: "sa-1",
		}, {
			Kind: "ServiceAccount",
			Name: "sa-2",
		}},
	}
	path := field.NewPath("dummy")
	errs := subject.Validate(path)
	assert.Equal(t, len(errs), 2)
	assert.Equal(t, errs[0].Field, "dummy.subjects[0].namespace")
	assert.Equal(t, errs[1].Field, "dummy.subjects[1].namespace")
}

func Test_Validate_EmptyUserInfo(t *testing.T) {
	subject := UserInfo{
		Subjects: nil,
	}
	path := field.NewPath("dummy")
	errs := subject.Validate(path)
	assert.Equal(t, len(errs), 0)
}

func Test_Validate_Roles(t *testing.T) {
	subject := UserInfo{
		Roles: []string{
			"namespace1:name1",
			"name2",
		},
	}
	path := field.NewPath("dummy")
	errs := subject.Validate(path)
	assert.Equal(t, len(errs), 1)
	assert.Equal(t, errs[0].Field, "dummy.roles[1]")
}
