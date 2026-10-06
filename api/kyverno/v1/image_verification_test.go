package v1

import (
	"fmt"
	"testing"

	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Test_ImageVerification(t *testing.T) {
	path := field.NewPath("dummy")
	testCases := []struct {
		name    string
		subject ImageVerification
		errors  func(*ImageVerification) field.ErrorList
	}{
		{
			name: "only key",
			subject: ImageVerification{
				ImageReferences: []string{"bla"},
				Key:             "bla",
			},
		},
		{
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
						"Either Rekor URL or roots are required",
					),
				}
			},
		},
		{
			name: "key roots, issuer, and subject",
			subject: ImageVerification{
				ImageReferences: []string{"bla"},
				Issuer:          "bla",
				Subject:         "bla",
				Roots:           "bla",
			},
		},
		{
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
			name: "static key invalid signature algorithm attestor",
			subject: ImageVerification{
				ImageReferences: []string{"*"},
				Attestors: []AttestorSet{
					{Entries: []Attestor{{
						Keys: &StaticKeyAttestor{PublicKeys: "bla", SignatureAlgorithm: "sha1"},
					}}},
				},
			},
			errors: func(i *ImageVerification) field.ErrorList {
				return field.ErrorList{
					field.Invalid(path.Child("attestors").Index(0).Child("entries").Index(0).Child("keys"),
						i.Attestors[0].Entries[0].Keys, "Invalid signature algorithm provided"),
				}
			},
		},
		{
			name: "valid static key default signature algorithm attestor",
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
			name: "valid static key sha224 signature algorithm attestor",
			subject: ImageVerification{
				ImageReferences: []string{"*"},
				Attestors: []AttestorSet{
					{Entries: []Attestor{{
						Keys: &StaticKeyAttestor{PublicKeys: "bla", SignatureAlgorithm: "sha224"},
					}}},
				},
			},
		},
		{
			name: "valid static key sah256 signature algorithm attestor",
			subject: ImageVerification{
				ImageReferences: []string{"*"},
				Attestors: []AttestorSet{
					{Entries: []Attestor{{
						Keys: &StaticKeyAttestor{PublicKeys: "bla", SignatureAlgorithm: "sha256"},
					}}},
				},
			},
		},
		{
			name: "valid static key sha384 signature algorithm attestor",
			subject: ImageVerification{
				ImageReferences: []string{"*"},
				Attestors: []AttestorSet{
					{Entries: []Attestor{{
						Keys: &StaticKeyAttestor{PublicKeys: "bla", SignatureAlgorithm: "sha384"},
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
						Keyless: &KeylessAttestor{Rekor: &Rekor{}, Issuer: "", Subject: ""},
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
						Keyless: &KeylessAttestor{Rekor: &Rekor{URL: "https://rekor.sigstore.dev"}, Issuer: "bla", Subject: "bla"},
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

	isAuditFailureAction := false
	for _, test := range testCases {
		subject := test.subject.Convert()
		errs := subject.Validate(isAuditFailureAction, path)
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

func Test_Audit_VerifyImageRule(t *testing.T) {
	path := field.NewPath("dummy")
	testCases := []struct {
		name    string
		subject ImageVerification
		errors  func(*ImageVerification) field.ErrorList
	}{
		{
			name: "mutateDigest set to true for audit failure action",
			subject: ImageVerification{
				ImageReferences: []string{"*"},
				Attestations: []Attestation{
					{
						PredicateType: "foo",
					},
				},
				MutateDigest: true,
			},
			errors: func(i *ImageVerification) field.ErrorList {
				return field.ErrorList{
					field.Invalid(
						path.Child("mutateDigest"),
						i.MutateDigest,
						"mutateDigest must be set to false for ‘Audit’ failure action",
					),
				}
			},
		},
		{
			name: "mutateDigest set to false for audit failure action",
			subject: ImageVerification{
				ImageReferences: []string{"*"},
				Attestations: []Attestation{
					{
						PredicateType: "foo",
					},
				},
				MutateDigest: false,
			},
		},
	}

	isAuditFailureAction := true // indicates validateFailureAction set to Audit
	for _, test := range testCases {
		subject := test.subject.Convert()
		errs := subject.Validate(isAuditFailureAction, path)
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

func Test_Enforce_VerifyImageRule(t *testing.T) {
	path := field.NewPath("dummy")
	testCases := []struct {
		name    string
		subject ImageVerification
		errors  func(*ImageVerification) field.ErrorList
	}{
		{
			name: "mutateDigest set to true for enforce failure action",
			subject: ImageVerification{
				ImageReferences: []string{"*"},
				Attestations: []Attestation{
					{
						PredicateType: "foo",
					},
				},
				MutateDigest: true,
			},
		},
		{
			name: "mutateDigest set to false for enforce failure action",
			subject: ImageVerification{
				ImageReferences: []string{"*"},
				Attestations: []Attestation{
					{
						PredicateType: "foo",
					},
				},
				MutateDigest: false,
			},
		},
	}

	isAuditFailureAction := false // indicates validateFailureAction set to Enforce
	for _, test := range testCases {
		subject := test.subject.Convert()
		errs := subject.Validate(isAuditFailureAction, path)
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

// TestConvertKeepsSynthesizedAttestorOnAttestations pins that the attestor
// Convert builds from the deprecated key shorthand survives onto the
// attestations it was attached to. It used to be discarded by a trailing
// reassignment, which left the entry with no attestor anywhere: verification
// then ran against an empty attestor set, which resolves to the public Fulcio
// roots with no issuer or subject constraint, so a rule that named a specific
// public key accepted anything signed through public Sigstore.
func TestConvertKeepsSynthesizedAttestorOnAttestations(t *testing.T) {
	const publicKey = "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE\n-----END PUBLIC KEY-----"

	iv := ImageVerification{
		ImageReferences: []string{"ghcr.io/example/*"},
		Key:             publicKey,
		Attestations: []Attestation{
			{Type: "https://slsa.dev/provenance/v0.2"},
			{Type: "https://example.com/custom"},
		},
	}

	converted := iv.Convert()

	assert.Equal(t, 0, len(converted.Attestors), "the attestor belongs to the attestations, not to the entry")
	assert.Equal(t, len(iv.Attestations), len(converted.Attestations))

	for i, attestation := range converted.Attestations {
		assert.Assert(t, len(attestation.Attestors) > 0,
			"attestation %d (%s) must carry the attestor synthesized from key", i, attestation.Type)
		entries := attestation.Attestors[0].Entries
		assert.Assert(t, len(entries) > 0, "attestation %d has an empty attestor set", i)
		assert.Assert(t, entries[0].Keys != nil, "attestation %d lost the static key attestor", i)
		assert.Equal(t, publicKey, entries[0].Keys.PublicKeys, "attestation %d has the wrong key", i)
	}

	// Each attestation must own its attestor: Attestor holds pointers, so one
	// shared set would make a later edit to one attestation change the others.
	first := converted.Attestations[0].Attestors[0].Entries[0].Keys
	second := converted.Attestations[1].Attestors[0].Entries[0].Keys
	assert.Assert(t, first != second, "attestations must not share one StaticKeyAttestor")
	first.PublicKeys = "mutated"
	assert.Equal(t, publicKey, second.PublicKeys, "editing one attestation's key changed another")

	// Positive control: with no attestations the attestor still lands on the
	// entry itself, which is the path that already worked.
	entryOnly := ImageVerification{ImageReferences: []string{"ghcr.io/example/*"}, Key: publicKey}
	convertedEntry := entryOnly.Convert()
	assert.Equal(t, 1, len(convertedEntry.Attestors))
	assert.Equal(t, publicKey, convertedEntry.Attestors[0].Entries[0].Keys.PublicKeys)
}

// TestConvertIsIdempotent pins that Convert can run twice without duplicating
// the attestor it synthesises. The mutate and validate paths convert a
// different number of times, so a non-idempotent Convert makes them disagree
// on the attestors for any rule using the deprecated key shorthand.
func TestConvertIsIdempotent(t *testing.T) {
	iv := ImageVerification{
		ImageReferences: []string{"ghcr.io/example/*"},
		Key:             "-----BEGIN PUBLIC KEY-----\nabc\n-----END PUBLIC KEY-----",
	}

	once := iv.Convert()
	twice := once.Convert()

	assert.Equal(t, 1, len(once.Attestors))
	assert.Equal(t, len(once.Attestors), len(twice.Attestors), "a second Convert must not append another attestor set")
	assert.Equal(t, "", once.Key, "Key must be cleared once it has been folded into an attestor")
	assert.DeepEqual(t, once.Attestors, twice.Attestors)
}
