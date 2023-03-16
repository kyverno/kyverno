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
		name: "only key",
		subject: ImageVerification{
			ImageReferences: []string{"bla"},
			Key:             "bla",
		},
	}, {
		name: "only keyless",
		subject: ImageVerification{
			ImageReferences: []string{"bla"},
			Issuer:          "bla",
			Subject:         "*",
		},
		errors: func(i *ImageVerification) field.ErrorList {
			return field.ErrorList{
				field.Invalid(
					path.Child("attestors").Index(0).Child("entries").Index(0).Child("keyless"),
					i.Attestors[0].Entries[0].Keyless,
					"Either Rekor URL or roots are required"),
			}
		},
	}, {
		name: "key roots, issuer, and subject",
		subject: ImageVerification{
			ImageReferences: []string{"bla"},
			Issuer:          "bla",
			Subject:         "bla",
			Roots:           "bla",
		},
	}, {
		name: "empty",
		subject: ImageVerification{
			ImageReferences: []string{"bla"},
		},
	},
		{
			name: "no image",
			subject: ImageVerification{
				Image: "",
				Key:   "bla",
			},
			errors: func(i *ImageVerification) field.ErrorList {
				return field.ErrorList{
					field.Invalid(path, i, "An image reference is required"),
				}
			},
		},
		{
			name: "no image reference",
			subject: ImageVerification{
				Key: "bla",
			},
			errors: func(i *ImageVerification) field.ErrorList {
				return field.ErrorList{
					field.Invalid(path, i, "An image reference is required"),
				}
			},
		},
		{
			name: "no attestors",
			subject: ImageVerification{
				ImageReferences: []string{"*"},
				Attestors:       []AttestorSet{},
			},
		},
		{
			name: "no entries",
			subject: ImageVerification{
				ImageReferences: []string{"*"},
				Attestors: []AttestorSet{
					{Entries: []Attestor{}},
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
				Attestors: []AttestorSet{
					{Entries: []Attestor{{}}},
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
				Attestors: []AttestorSet{
					{Entries: []Attestor{{
						Keys: &StaticKeyAttestor{},
					}}},
				},
			},
			errors: func(i *ImageVerification) field.ErrorList {
				return field.ErrorList{
					field.Invalid(path.Child("attestors").Index(0).Child("entries").Index(0).Child("keys"),
						i.Attestors[0].Entries[0].Keys, "A public key, kms key or secret is required"),
				}
			},
		},
		{
			name: "valid static key attestor",
			subject: ImageVerification{
				ImageReferences: []string{"*"},
				Attestors: []AttestorSet{
					{Entries: []Attestor{{
						Keys: &StaticKeyAttestor{PublicKeys: "bla"},
					}}},
				},
			},
		},
		{
			name: "invalid keyless attestor",
			subject: ImageVerification{
				ImageReferences: []string{"*"},
				Attestors: []AttestorSet{
					{Entries: []Attestor{{
						Keyless: &KeylessAttestor{Rekor: &CTLog{}, Issuer: "", Subject: ""},
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
				Attestors: []AttestorSet{
					{Entries: []Attestor{{
						Keyless: &KeylessAttestor{Rekor: &CTLog{URL: "https://rekor.sigstore.dev"}, Issuer: "bla", Subject: "bla"},
					}}},
				},
			},
		},
		{
			name: "valid keyless attestor",
			subject: ImageVerification{
				ImageReferences: []string{"*"},
				Attestations: []Attestation{
					{
						PredicateType: "foo",
					},
				},
			},
		},
		{
			name: "multiple entries",
			subject: ImageVerification{
				ImageReferences: []string{"*"},
				Attestors: []AttestorSet{
					{
						Entries: []Attestor{
							{
								Keys: &StaticKeyAttestor{
									PublicKeys: "key1",
								},
							},
							{
								Keys: &StaticKeyAttestor{
									PublicKeys: "key2",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, test := range testCases {
		subject := test.subject.Convert()
		errs := subject.Validate(path)
		var expectedErrs field.ErrorList
		if test.errors != nil {
			expectedErrs = test.errors(subject)
		}

		assert.Equal(t, len(errs), len(expectedErrs), fmt.Sprintf("test `%s` error count mismatch, errors %v", test.name, errs))
		if len(errs) != 0 {
			assert.DeepEqual(t, errs, expectedErrs)
		}
	}
}
