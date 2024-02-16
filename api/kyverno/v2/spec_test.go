package v2

import (
	"testing"

	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"gotest.tools/assert"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_Validate_UniqueRuleName(t *testing.T) {
	subject := Spec{
		Rules: []Rule{{
			Name: "deny-privileged-disallowpriviligedescalation",
			MatchResources: MatchResources{
				Any: kyvernov2beta1.ResourceFilters{{
					ResourceDescription: kyvernov2beta1.ResourceDescription{
						Kinds: []string{
							"Pod",
						},
					},
				}},
			},
			Validation: Validation{
				Message: "message",
				RawAnyPattern: &apiextv1.JSON{
					Raw: []byte("{"),
				},
			},
		}, {
			Name: "deny-privileged-disallowpriviligedescalation",
			MatchResources: MatchResources{
				Any: kyvernov2beta1.ResourceFilters{{
					ResourceDescription: kyvernov2beta1.ResourceDescription{
						Kinds: []string{
							"Pod",
						},
					}},
				}},
			Validation: Validation{
				Message: "message",
				RawAnyPattern: &apiextv1.JSON{
					Raw: []byte("{"),
				},
			},
		}},
	}
	path := field.NewPath("dummy")
	errs := subject.Validate(path, false, "", nil)
	assert.Equal(t, len(errs), 1)
	assert.Equal(t, errs[0].Field, "dummy.rules[1].name")
	assert.Equal(t, errs[0].Type, field.ErrorTypeInvalid)
	assert.Equal(t, errs[0].Detail, "Duplicate rule name: 'deny-privileged-disallowpriviligedescalation'")
}
