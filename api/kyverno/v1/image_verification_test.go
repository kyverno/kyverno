package v1

import (
	"fmt"
	"testing"

	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_ImageVerification(t *testing.T) {
	path := field.NewPath("dummy")
	testCases := []struct {
		name    string
		subject ImageVerification
		errors  func(*ImageVerification) field.ErrorList
	}{{
		name: "valid",
		subject: ImageVerification{
			Image:       "bla",
			Key:         "bla",
			Roots:       "bla",
			Subject:     "bla",
			Issuer:      "bla",
			Annotations: map[string]string{"bla": "bla"},
			Repository:  "bla",
		},
	}, {
		name: "only key",
		subject: ImageVerification{
			Image: "bla",
			Key:   "bla",
		},
	}, {
		name: "only roots and subject",
		subject: ImageVerification{
			Image:   "bla",
			Roots:   "bla",
			Subject: "bla",
		},
	}, {
		name: "key roots and subject",
		subject: ImageVerification{
			Image:   "bla",
			Key:     "bla",
			Roots:   "bla",
			Subject: "bla",
		},
	}, {
		name: "empty",
		subject: ImageVerification{
			Image: "bla",
		},
		errors: func(i *ImageVerification) field.ErrorList {
			return field.ErrorList{
				field.Invalid(path, i, "Either a public key, or root certificates and an email, are required"),
			}
		},
	}}
	for _, test := range testCases {
		errs := test.subject.Validate(path)
		var expectedErrs field.ErrorList
		if test.errors != nil {
			expectedErrs = test.errors(&test.subject)
		}
		assert.Equal(t, len(errs), len(expectedErrs), fmt.Sprintf("test %s failed", test.name))
		if len(errs) != 0 {
			assert.DeepEqual(t, errs, expectedErrs)
		}
	}
}
