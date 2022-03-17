package v1

import (
	"testing"

	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_Validate_UniqueRuleName(t *testing.T) {
	subject := Spec{
		Rules: []Rule{{
			Name: "deny-privileged-disallowpriviligedescalation",
			MatchResources: MatchResources{
				ResourceDescription: ResourceDescription{
					Kinds: []string{
						"Pod",
					},
				},
			},
		}, {
			Name: "deny-privileged-disallowpriviligedescalation",
			MatchResources: MatchResources{
				ResourceDescription: ResourceDescription{
					Kinds: []string{
						"Pod",
					},
				},
			},
		}},
	}
	path := field.NewPath("dummy")
	errs := subject.Validate(path)
	assert.Assert(t, len(errs) == 1)
	assert.Equal(t, errs[0].Field, "dummy.rules[1].name")
	assert.Equal(t, errs[0].Type, field.ErrorTypeInvalid)
	assert.Equal(t, errs[0].Detail, "Duplicate rule name: 'deny-privileged-disallowpriviligedescalation'")
}
