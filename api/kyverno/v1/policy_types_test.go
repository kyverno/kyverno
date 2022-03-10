package v1

import (
	"testing"

	"gotest.tools/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_Validate_ServiceAccount2(t *testing.T) {
	rule := Rule{
		Name: "test",
		ExcludeResources: ExcludeResources{
			UserInfo: UserInfo{
				Subjects: []rbacv1.Subject{{
					Kind: "ServiceAccount",
					Name: "sa-1",
				}, {
					Kind: "ServiceAccount",
					Name: "sa-2",
				}},
			},
		},
	}
	path := field.NewPath("dummy")
	errs := rule.Validate(path)
	assert.Equal(t, len(errs), 2)
	assert.Equal(t, errs[0].Field, "dummy.exclude.subjects[0].namespace")
	assert.Equal(t, errs[1].Field, "dummy.exclude.subjects[1].namespace")
}
