package v2beta1

import (
	"testing"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_Validate_UniqueRuleName(t *testing.T) {
	subject := Spec{
		Rules: []Rule{{
			Name: "deny-privileged-disallowpriviligedescalation",
			MatchResources: MatchResources{
				Any: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds: []string{
							"Pod",
						},
					},
				}},
			},
			Validation: &Validation{
				Message:       "message",
				RawAnyPattern: kyverno.ToAny("{"),
			},
		}, {
			Name: "deny-privileged-disallowpriviligedescalation",
			MatchResources: MatchResources{
				Any: kyvernov1.ResourceFilters{{
					ResourceDescription: kyvernov1.ResourceDescription{
						Kinds: []string{
							"Pod",
						},
					}},
				}},
			Validation: &Validation{
				Message:       "message",
				RawAnyPattern: kyverno.ToAny("{"),
			},
		}},
	}
	path := field.NewPath("dummy")
	_, errs := subject.Validate(path, false, "", nil)
	assert.Equal(t, len(errs), 1)
	assert.Equal(t, errs[0].Field, "dummy.rules[1].name")
	assert.Equal(t, errs[0].Type, field.ErrorTypeInvalid)
	assert.Equal(t, errs[0].Detail, "Duplicate rule name: 'deny-privileged-disallowpriviligedescalation'")
}
