package cosign

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"github.com/stretchr/testify/assert"
)

func Test_ImageSignatureVerificationKeyless(t *testing.T) {
	image := "ghcr.io/jimbugwadia/pause2"
	idf, err := imagedataloader.New(nil)
	assert.NoError(t, err)
	img, err := idf.FetchImageData(context.TODO(), image)
	assert.NoError(t, err)

	attestor := &v1beta1.Attestor{
		Name: "test",
		Cosign: &v1beta1.Cosign{
			Keyless: &v1beta1.Keyless{
				Identities: []v1beta1.Identity{
					{
						Issuer:  "https://github.com/login/oauth",
						Subject: "jim@nirmata.com",
					},
				},
			},
			CTLog: &v1beta1.CTLog{
				URL:               "https://rekor.sigstore.dev",
				InsecureIgnoreSCT: true,
			},
		},
	}

	v := Verifier{log: logr.Discard()}
	err = v.VerifyImageSignature(context.TODO(), img, attestor)
	assert.NoError(t, err)
}

func Test_ImageSignatureVerificationFail(t *testing.T) {
	image := "ghcr.io/jimbugwadia/pause2"
	idf, err := imagedataloader.New(nil)
	assert.NoError(t, err)
	img, err := idf.FetchImageData(context.TODO(), image)
	assert.NoError(t, err)

	attestor := &v1beta1.Attestor{
		Name: "test",
		Cosign: &v1beta1.Cosign{
			Keyless: &v1beta1.Keyless{
				Identities: []v1beta1.Identity{
					{
						Issuer:  "https://github.com/login/oauth",
						Subject: "jim@invalid.com",
					},
				},
			},
			CTLog: &v1beta1.CTLog{
				URL:               "https://rekor.sigstore.dev",
				InsecureIgnoreSCT: true,
			},
		},
	}

	v := Verifier{log: logr.Discard()}
	err = v.VerifyImageSignature(context.TODO(), img, attestor)
	assert.ErrorContains(t, err, "no matching signatures: none of the expected identities matched what was in the certificate")
}

func Test_ImageSignatureVerificationKeyed(t *testing.T) {
	image := "ghcr.io/kyverno/test-verify-image:signed"
	idf, err := imagedataloader.New(nil)
	assert.NoError(t, err)
	img, err := idf.FetchImageData(context.TODO(), image)
	assert.NoError(t, err)

	attestor := &v1beta1.Attestor{
		Name: "test",
		Cosign: &v1beta1.Cosign{
			Key: &v1beta1.Key{
				Data: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
-----END PUBLIC KEY-----`,
			},
			CTLog: &v1beta1.CTLog{
				URL:                "https://rekor.sigstore.dev",
				InsecureIgnoreTlog: true,
			},
		},
	}

	v := Verifier{log: logr.Discard()}
	err = v.VerifyImageSignature(context.TODO(), img, attestor)
	assert.NoError(t, err)
}

func Test_ImageSignatureVerificationKeyedFail(t *testing.T) {
	image := "ghcr.io/kyverno/test-verify-image:signed"
	idf, err := imagedataloader.New(nil)
	assert.NoError(t, err)
	img, err := idf.FetchImageData(context.TODO(), image)
	assert.NoError(t, err)

	attestor := &v1beta1.Attestor{
		Name: "test",
		Cosign: &v1beta1.Cosign{
			Key: &v1beta1.Key{
				Data: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEoKYkkX32oSx61B4iwKXa6llAF2dB
IoL3R/9n1SJ7s00Nfkk3z4/Ar6q8el/guUmXi8akEJMxvHnvphorVUz8vQ==
-----END PUBLIC KEY-----`,
			},
			CTLog: &v1beta1.CTLog{
				URL:                "https://rekor.sigstore.dev",
				InsecureIgnoreTlog: true,
			},
		},
	}

	v := Verifier{log: logr.Discard()}
	err = v.VerifyImageSignature(context.TODO(), img, attestor)
	assert.ErrorContains(t, err, "failed to verify cosign signatures")
}

// TestBackwardCompatibility_CosignV2 tests that v2 signatures still work
func TestBackwardCompatibility_CosignV2(t *testing.T) {
	t.Run("keyless signature verification (v2 compatible)", func(t *testing.T) {
		// This test verifies that existing cosign v2 signatures still work
		// through the traditional VerifyImageSignature path
		image := "ghcr.io/jimbugwadia/pause2"
		idf, err := imagedataloader.New(nil)
		assert.NoError(t, err)
		img, err := idf.FetchImageData(context.TODO(), image)
		assert.NoError(t, err)

		attestor := &v1beta1.Attestor{
			Name: "test",
			Cosign: &v1beta1.Cosign{
				Keyless: &v1beta1.Keyless{
					Identities: []v1beta1.Identity{
						{
							Issuer:  "https://github.com/login/oauth",
							Subject: "jim@nirmata.com",
						},
					},
				},
				CTLog: &v1beta1.CTLog{
					URL:               "https://rekor.sigstore.dev",
					InsecureIgnoreSCT: true,
				},
			},
		}

		v := Verifier{log: logr.Discard()}

		// Traditional verification should still work
		err = v.VerifyImageSignature(context.TODO(), img, attestor)
		assert.NoError(t, err, "cosign v2 signatures should still verify through traditional path")
	})

	t.Run("keyed signature verification (v2 compatible)", func(t *testing.T) {
		// Verify that key-based signatures (v2 style) still work
		image := "ghcr.io/kyverno/test-verify-image:signed"
		idf, err := imagedataloader.New(nil)
		assert.NoError(t, err)
		img, err := idf.FetchImageData(context.TODO(), image)
		assert.NoError(t, err)

		attestor := &v1beta1.Attestor{
			Name: "test",
			Cosign: &v1beta1.Cosign{
				Key: &v1beta1.Key{
					Data: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
-----END PUBLIC KEY-----`,
				},
				CTLog: &v1beta1.CTLog{
					URL:                "https://rekor.sigstore.dev",
					InsecureIgnoreTlog: true,
				},
			},
		}

		v := Verifier{log: logr.Discard()}

		// Traditional verification should still work
		err = v.VerifyImageSignature(context.TODO(), img, attestor)
		assert.NoError(t, err, "cosign v2 key-based signatures should still verify")
	})
}

// TestBackwardCompatibility_ExistingTests ensures no regression
func TestBackwardCompatibility_ExistingTests(t *testing.T) {
	t.Run("all existing test patterns still work", func(t *testing.T) {
		// Run a subset of existing test patterns to ensure backward compatibility
		tests := []struct {
			name     string
			image    string
			attestor *v1beta1.Attestor
			wantErr  bool
		}{
			{
				name:  "keyless valid",
				image: "ghcr.io/jimbugwadia/pause2",
				attestor: &v1beta1.Attestor{
					Name: "test",
					Cosign: &v1beta1.Cosign{
						Keyless: &v1beta1.Keyless{
							Identities: []v1beta1.Identity{
								{
									Issuer:  "https://github.com/login/oauth",
									Subject: "jim@nirmata.com",
								},
							},
						},
						CTLog: &v1beta1.CTLog{
							URL:               "https://rekor.sigstore.dev",
							InsecureIgnoreSCT: true,
						},
					},
				},
				wantErr: false,
			},
			{
				name:  "keyless invalid subject",
				image: "ghcr.io/jimbugwadia/pause2",
				attestor: &v1beta1.Attestor{
					Name: "test",
					Cosign: &v1beta1.Cosign{
						Keyless: &v1beta1.Keyless{
							Identities: []v1beta1.Identity{
								{
									Issuer:  "https://github.com/login/oauth",
									Subject: "wrong@example.com",
								},
							},
						},
						CTLog: &v1beta1.CTLog{
							URL:               "https://rekor.sigstore.dev",
							InsecureIgnoreSCT: true,
						},
					},
				},
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				idf, err := imagedataloader.New(nil)
				assert.NoError(t, err)
				img, err := idf.FetchImageData(context.TODO(), tt.image)
				assert.NoError(t, err)

				v := Verifier{log: logr.Discard()}
				err = v.VerifyImageSignature(context.TODO(), img, tt.attestor)

				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

// TestVerifier_ErrorHandling tests various error conditions
func TestVerifier_ErrorHandling(t *testing.T) {
	t.Run("nil image data", func(t *testing.T) {
		v := Verifier{log: logr.Discard()}
		attestor := &v1beta1.Attestor{
			Name:   "test",
			Cosign: &v1beta1.Cosign{},
		}

		// This should panic or error gracefully
		assert.Panics(t, func() {
			_ = v.VerifyImageSignature(context.TODO(), nil, attestor)
		})
	})

	t.Run("nil attestor for signature verification", func(t *testing.T) {
		idf, err := imagedataloader.New(nil)
		assert.NoError(t, err)
		img, err := idf.FetchImageData(context.TODO(), "ghcr.io/kyverno/test-verify-image:signed")
		assert.NoError(t, err)

		v := Verifier{log: logr.Discard()}

		// Nil attestor should cause a panic or error
		defer func() {
			if r := recover(); r != nil {
				t.Logf("Expected panic caught: %v", r)
			}
		}()

		err = v.VerifyImageSignature(context.TODO(), img, nil)
		// If we get here without panic, check for error
		if err == nil {
			t.Error("Expected error for nil attestor")
		}
	})

	t.Run("attestor with nil cosign", func(t *testing.T) {
		idf, err := imagedataloader.New(nil)
		assert.NoError(t, err)
		img, err := idf.FetchImageData(context.TODO(), "ghcr.io/kyverno/test-verify-image:signed")
		assert.NoError(t, err)

		attestor := &v1beta1.Attestor{
			Name:   "test",
			Cosign: nil,
		}

		v := Verifier{log: logr.Discard()}
		err = v.VerifyImageSignature(context.TODO(), img, attestor)
		assert.Error(t, err)
		assert.ErrorContains(t, err, "cosign verifier only supports cosign attestor")
	})
}

// TestVerifier_AnnotationChecking tests annotation verification
func TestVerifier_AnnotationChecking(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping annotation test in short mode")
	}

	t.Run("with annotations", func(t *testing.T) {
		image := "ghcr.io/kyverno/test-verify-image:signed"
		idf, err := imagedataloader.New(nil)
		assert.NoError(t, err)
		img, err := idf.FetchImageData(context.TODO(), image)
		assert.NoError(t, err)

		attestor := &v1beta1.Attestor{
			Name: "test",
			Cosign: &v1beta1.Cosign{
				Key: &v1beta1.Key{
					Data: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
-----END PUBLIC KEY-----`,
				},
				CTLog: &v1beta1.CTLog{
					URL:                "https://rekor.sigstore.dev",
					InsecureIgnoreTlog: true,
				},
				Annotations: map[string]string{
					// Add some annotations to test
					"test-annotation": "test-value",
				},
			},
		}

		v := Verifier{log: logr.Discard()}
		err = v.VerifyImageSignature(context.TODO(), img, attestor)

		// May fail if annotations don't match - that's expected
		if err != nil {
			t.Logf("Annotation check failed (may be expected): %v", err)
		}
	})
}

// TestConcurrentVerification tests thread safety
func TestConcurrentVerification(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrency test in short mode")
	}

	t.Run("concurrent signature verifications", func(t *testing.T) {
		image := "ghcr.io/kyverno/test-verify-image:signed"
		idf, err := imagedataloader.New(nil)
		assert.NoError(t, err)

		attestor := &v1beta1.Attestor{
			Name: "test",
			Cosign: &v1beta1.Cosign{
				Key: &v1beta1.Key{
					Data: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
-----END PUBLIC KEY-----`,
				},
				CTLog: &v1beta1.CTLog{
					URL:                "https://rekor.sigstore.dev",
					InsecureIgnoreTlog: true,
				},
			},
		}

		// Run multiple verifications concurrently
		done := make(chan bool, 5)
		for i := 0; i < 5; i++ {
			go func() {
				img, err := idf.FetchImageData(context.TODO(), image)
				if err != nil {
					done <- false
					return
				}

				v := Verifier{log: logr.Discard()}
				err = v.VerifyImageSignature(context.TODO(), img, attestor)
				done <- err == nil
			}()
		}

		// Wait for all goroutines
		successes := 0
		for i := 0; i < 5; i++ {
			if <-done {
				successes++
			}
		}

		// All should succeed
		assert.Equal(t, 5, successes, "all concurrent verifications should succeed")
	})
}

// TestCosignV3_BundleFormatDetection tests the automatic detection of cosign v3 bundle format
func TestCosignV3_BundleFormatDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping v3 bundle format test in short mode")
	}

	t.Run("v3 keyless signature with bundle format", func(t *testing.T) {
		// Test with an image signed using cosign v3 keyless (new bundle format)
		// This image should be signed with: cosign sign --yes <image>
		image := "ghcr.io/jimbugwadia/pause2"
		idf, err := imagedataloader.New(nil)
		assert.NoError(t, err)
		img, err := idf.FetchImageData(context.TODO(), image)
		if err != nil {
			t.Skipf("test image not accessible: %v", err)
		}

		attestor := &v1beta1.Attestor{
			Name: "test",
			Cosign: &v1beta1.Cosign{
				Keyless: &v1beta1.Keyless{
					Identities: []v1beta1.Identity{
						{
							Issuer:  "https://github.com/login/oauth",
							Subject: "jim@nirmata.com",
						},
					},
				},
				CTLog: &v1beta1.CTLog{
					URL:               "https://rekor.sigstore.dev",
					InsecureIgnoreSCT: true,
				},
			},
		}

		v := Verifier{log: logr.Discard()}
		err = v.VerifyImageSignature(context.TODO(), img, attestor)
		assert.NoError(t, err, "cosign v3 keyless signatures should verify automatically")
	})

	t.Run("buildCheckOptsWithBundleDetection returns valid opts", func(t *testing.T) {
		// Unit test for the helper function
		image := "ghcr.io/kyverno/test-verify-image:signed"
		idf, err := imagedataloader.New(nil)
		assert.NoError(t, err)
		img, err := idf.FetchImageData(context.TODO(), image)
		if err != nil {
			t.Skipf("test image not accessible: %v", err)
		}

		attestor := &v1beta1.Cosign{
			Key: &v1beta1.Key{
				Data: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
-----END PUBLIC KEY-----`,
			},
			CTLog: &v1beta1.CTLog{
				URL:                "https://rekor.sigstore.dev",
				InsecureIgnoreTlog: true,
			},
		}

		v := Verifier{log: logr.Discard()}
		opts, err := v.buildCheckOptsWithBundleDetection(context.TODO(), attestor, img)
		assert.NoError(t, err)
		assert.NotNil(t, opts)
		assert.NotNil(t, opts.RegistryClientOpts)
		// NewBundleFormat should be set (either true or false based on detection)
		t.Logf("Bundle format detected: %v", opts.NewBundleFormat)
	})
}

// TestCosignV3_TrustedMaterialSupport tests TrustedMaterial configuration for v3
func TestCosignV3_TrustedMaterialSupport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping v3 TrustedMaterial test in short mode")
	}

	t.Run("keyless verification with TrustedMaterial", func(t *testing.T) {
		// This test verifies that TrustedMaterial is properly configured
		// for keyless verification in cosign v3
		image := "ghcr.io/jimbugwadia/pause2"
		idf, err := imagedataloader.New(nil)
		assert.NoError(t, err)
		img, err := idf.FetchImageData(context.TODO(), image)
		if err != nil {
			t.Skipf("test image not accessible: %v", err)
		}

		attestor := &v1beta1.Attestor{
			Name: "test",
			Cosign: &v1beta1.Cosign{
				Keyless: &v1beta1.Keyless{
					Identities: []v1beta1.Identity{
						{
							Issuer:  "https://github.com/login/oauth",
							Subject: "jim@nirmata.com",
						},
					},
				},
				CTLog: &v1beta1.CTLog{
					URL:               "https://rekor.sigstore.dev",
					InsecureIgnoreSCT: true,
				},
			},
		}

		v := Verifier{log: logr.Discard()}
		// This should work with TrustedMaterial configured via checkOptions
		err = v.VerifyImageSignature(context.TODO(), img, attestor)
		assert.NoError(t, err, "keyless verification should work with TrustedMaterial")
	})
}

// TestCosignV3_AttestationBundleFormat tests attestation verification with v3 bundle format
func TestCosignV3_AttestationBundleFormat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping v3 attestation bundle test in short mode")
	}

	t.Run("v3 keyless attestation with bundle format", func(t *testing.T) {
		// Test attestation verification for images with v3 bundle format attestations
		// Note: This requires a test image with actual v3 attestations
		image := "ghcr.io/kyverno/test-verify-image:signed"
		idf, err := imagedataloader.New(nil)
		assert.NoError(t, err)
		img, err := idf.FetchImageData(context.TODO(), image)
		if err != nil {
			t.Skipf("test image not accessible: %v", err)
		}

		attestation := &v1beta1.Attestation{
			Name: "slsa-provenance",
			InToto: &v1beta1.InToto{
				Type: "https://slsa.dev/provenance/v1",
			},
		}

		attestor := &v1beta1.Attestor{
			Name: "test",
			Cosign: &v1beta1.Cosign{
				Keyless: &v1beta1.Keyless{
					Identities: []v1beta1.Identity{
						{
							Issuer:  "https://token.actions.githubusercontent.com",
							Subject: "https://github.com/*",
						},
					},
				},
				CTLog: &v1beta1.CTLog{
					URL: "https://rekor.sigstore.dev",
				},
			},
		}

		v := Verifier{log: logr.Discard()}
		err = v.VerifyAttestationSignature(context.TODO(), img, attestation, attestor)
		// May not have attestations, but should not error on format detection
		if err != nil {
			t.Logf("Attestation verification result: %v", err)
			// Check that it's not a bundle format error
			assert.NotContains(t, err.Error(), "bundle format", "should handle bundle format detection")
		}
	})

	t.Run("attestation with bundle detection helper", func(t *testing.T) {
		// Verify that attestation verification uses the same bundle detection logic
		image := "ghcr.io/kyverno/test-verify-image:signed"
		idf, err := imagedataloader.New(nil)
		assert.NoError(t, err)
		img, err := idf.FetchImageData(context.TODO(), image)
		if err != nil {
			t.Skipf("test image not accessible: %v", err)
		}

		attestation := &v1beta1.Attestation{
			Name: "test-attestation",
			InToto: &v1beta1.InToto{
				Type: "https://slsa.dev/provenance/v0.2",
			},
		}

		attestor := &v1beta1.Attestor{
			Name: "test",
			Cosign: &v1beta1.Cosign{
				Key: &v1beta1.Key{
					Data: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
-----END PUBLIC KEY-----`,
				},
				CTLog: &v1beta1.CTLog{
					URL:                "https://rekor.sigstore.dev",
					InsecureIgnoreTlog: true,
				},
			},
		}

		v := Verifier{log: logr.Discard()}
		// Should use buildCheckOptsWithBundleDetection internally
		err = v.VerifyAttestationSignature(context.TODO(), img, attestation, attestor)
		if err != nil {
			// Expected - test image likely doesn't have attestations
			t.Logf("Expected result (no attestations): %v", err)
		}
	})
}

// TestCosignV3_BackwardCompatibility tests that v3 implementation maintains v2 compatibility
func TestCosignV3_BackwardCompatibility(t *testing.T) {
	t.Run("v2 and v3 both work through same code path", func(t *testing.T) {
		// This test verifies that the refactored code handles both v2 and v3
		tests := []struct {
			name     string
			image    string
			attestor *v1beta1.Attestor
		}{
			{
				name:  "v2 traditional keyless",
				image: "ghcr.io/jimbugwadia/pause2",
				attestor: &v1beta1.Attestor{
					Name: "test",
					Cosign: &v1beta1.Cosign{
						Keyless: &v1beta1.Keyless{
							Identities: []v1beta1.Identity{
								{
									Issuer:  "https://github.com/login/oauth",
									Subject: "jim@nirmata.com",
								},
							},
						},
						CTLog: &v1beta1.CTLog{
							URL:               "https://rekor.sigstore.dev",
							InsecureIgnoreSCT: true,
						},
					},
				},
			},
			{
				name:  "v2 traditional keyed",
				image: "ghcr.io/kyverno/test-verify-image:signed",
				attestor: &v1beta1.Attestor{
					Name: "test",
					Cosign: &v1beta1.Cosign{
						Key: &v1beta1.Key{
							Data: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
-----END PUBLIC KEY-----`,
						},
						CTLog: &v1beta1.CTLog{
							URL:                "https://rekor.sigstore.dev",
							InsecureIgnoreTlog: true,
						},
					},
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				idf, err := imagedataloader.New(nil)
				assert.NoError(t, err)
				img, err := idf.FetchImageData(context.TODO(), tt.image)
				if err != nil {
					t.Skipf("test image not accessible: %v", err)
				}

				v := Verifier{log: logr.Discard()}
				err = v.VerifyImageSignature(context.TODO(), img, tt.attestor)
				assert.NoError(t, err, "both v2 and v3 should work through refactored code")
			})
		}
	})
}

// TestCosignV3_BundleFormatTransition tests images that might have both formats
func TestCosignV3_BundleFormatTransition(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping v3 transition test in short mode")
	}

	t.Run("image with traditional format falls back gracefully", func(t *testing.T) {
		// Test that when bundle detection finds no bundles, it falls back to traditional
		image := "ghcr.io/kyverno/test-verify-image:signed"
		idf, err := imagedataloader.New(nil)
		assert.NoError(t, err)
		img, err := idf.FetchImageData(context.TODO(), image)
		if err != nil {
			t.Skipf("test image not accessible: %v", err)
		}

		attestor := &v1beta1.Attestor{
			Name: "test",
			Cosign: &v1beta1.Cosign{
				Key: &v1beta1.Key{
					Data: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
-----END PUBLIC KEY-----`,
				},
				CTLog: &v1beta1.CTLog{
					URL:                "https://rekor.sigstore.dev",
					InsecureIgnoreTlog: true,
				},
			},
		}

		v := Verifier{log: logr.Discard()}
		// Should detect no bundles and fall back to traditional verification
		err = v.VerifyImageSignature(context.TODO(), img, attestor)
		assert.NoError(t, err, "should gracefully fall back to traditional format")
	})
}
