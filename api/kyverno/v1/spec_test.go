package v1

import (
	"testing"
	"time"

	"gotest.tools/assert"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			Validation: &Validation{
				Message: "message",
				RawAnyPattern: &apiextv1.JSON{
					Raw: []byte("{"),
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
			Validation: &Validation{
				Message: "message",
				RawAnyPattern: &apiextv1.JSON{
					Raw: []byte("{"),
				},
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

func Test_GetBackgroundScanInterval(t *testing.T) {
	d := &metav1.Duration{}
	specWithOverride := Spec{
		BackgroundScanInterval: d,
	}
	assert.Equal(t, specWithOverride.GetBackgroundScanInterval(), d)

	specNil := Spec{}
	assert.Equal(t, specNil.GetBackgroundScanInterval(), (*metav1.Duration)(nil))
}

func Test_Validate_BackgroundScanInterval(t *testing.T) {
	path := field.NewPath("spec")

	// Valid duration (> 0)
	validSpec := Spec{
		BackgroundScanInterval: &metav1.Duration{Duration: 5 * time.Minute},
	}
	_, errs := validSpec.Validate(path, false, "", nil)
	assert.Equal(t, len(errs), 0)

	// Invalid duration (<= 0)
	invalidSpec := Spec{
		BackgroundScanInterval: &metav1.Duration{Duration: 0},
	}
	_, errs = invalidSpec.Validate(path, false, "", nil)
	assert.Equal(t, len(errs), 1)
	assert.Equal(t, errs[0].Field, "spec.backgroundScanInterval")
	assert.Equal(t, errs[0].Type, field.ErrorTypeInvalid)
}
