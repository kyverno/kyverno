package v2beta1

import (
	"fmt"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
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
		name: "no attestors",
		subject: ImageVerification{
			ImageReferences: []string{"*"},
			Attestors:       []kyvernov1.AttestorSet{},
		},
	},
		{
			name: "no entries",
			subject: ImageVerification{
				ImageReferences: []string{"*"},
				Attestors: []kyvernov1.AttestorSet{
					{Entries: []kyvernov1.Attestor{}},
				},
			},
			errors: func(i *ImageVerification) field.ErrorList {
				return field.ErrorList{
					field.Invalid(path.Child("attestors").Index(0), &i.Attestors[0], "An entry is required"),
				}
			},
		},
		{
			name: "empty attestor",
			subject: ImageVerification{
				ImageReferences: []string{"*"},
				Attestors: []kyvernov1.AttestorSet{
					{Entries: []kyvernov1.Attestor{{}}},
				},
			},
			errors: func(i *ImageVerification) field.ErrorList {
				return field.ErrorList{
					field.Invalid(path.Child("attestors").Index(0).Child("entries").Index(0),
						&i.Attestors[0].Entries[0], "keys, certificates, keyless, or a nested attestor is required"),
				}
			},
		},
		{
			name: "empty static key attestor",
			subject: ImageVerification{
				ImageReferences: []string{"*"},
				Attestors: []kyvernov1.AttestorSet{
					{Entries: []kyvernov1.Attestor{{
						Keys: &kyvernov1.StaticKeyAttestor{},
					}}},
				},
			},
			errors: func(i *ImageVerification) field.ErrorList {
				return field.ErrorList{
					field.Invalid(path.Child("attestors").Index(0).Child("entries").Index(0).Child("keys"),
						i.Attestors[0].Entries[0].Keys, "A key is required"),
				}
			},
		},
		{
			name: "valid static key attestor",
			subject: ImageVerification{
				ImageReferences: []string{"*"},
				Attestors: []kyvernov1.AttestorSet{
					{Entries: []kyvernov1.Attestor{{
						Keys: &kyvernov1.StaticKeyAttestor{PublicKeys: "bla"},
					}}},
				},
			},
		},
		{
			name: "invalid keyless attestor",
			subject: ImageVerification{
				ImageReferences: []string{"*"},
				Attestors: []kyvernov1.AttestorSet{
					{Entries: []kyvernov1.Attestor{{
						Keyless: &kyvernov1.KeylessAttestor{Rekor: &kyvernov1.CTLog{}, Issuer: "", Subject: ""},
					}}},
				},
			},
			errors: func(i *ImageVerification) field.ErrorList {
				return field.ErrorList{
					field.Invalid(path.Child("attestors").Index(0).Child("entries").Index(0).Child("keyless"),
						i.Attestors[0].Entries[0].Keyless, "An URL is required"),
				}
			},
		},
		{
			name: "valid keyless attestor",
			subject: ImageVerification{
				ImageReferences: []string{"*"},
				Attestors: []kyvernov1.AttestorSet{
					{Entries: []kyvernov1.Attestor{{
						Keyless: &kyvernov1.KeylessAttestor{Rekor: &kyvernov1.CTLog{URL: "https://rekor.sigstore.dev"}, Issuer: "bla", Subject: "bla"},
					}}},
				},
			},
		},
		{
			name: "valid keyless attestor",
			subject: ImageVerification{
				ImageReferences: []string{"*"},
				Attestations: []kyvernov1.Attestation{
					{
						PredicateType: "foo",
					},
				},
			},
		},
	}

	for _, test := range testCases {
		subject := test.subject
		errs := subject.Validate(path)
		var expectedErrs field.ErrorList
		if test.errors != nil {
			expectedErrs = test.errors(&subject)
		}

		assert.Equal(t, len(errs), len(expectedErrs), fmt.Sprintf("test `%s` error count mismatch, errors %v", test.name, errs))
		if len(errs) != 0 {
			assert.DeepEqual(t, errs, expectedErrs)
		}
	}
}
