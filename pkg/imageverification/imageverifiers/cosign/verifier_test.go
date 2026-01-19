package cosign

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// Test images from cosign test repository
	testRegistry           = "ghcr.io/lucchmielowski/kyverno-cosign-testbed"
	unsignedImage          = testRegistry + ":unsigned"
	githubAttestationImage = testRegistry + ":github-attestation"
	v2KeyBasedImage        = testRegistry + ":v2-traditional"
	v2KeylessImage         = testRegistry + ":v2-keyless"
	v3KeyBasedImage        = testRegistry + ":v3-traditional"
	v3KeylessImage         = testRegistry + ":v3-keyless"
	v3BundleImage          = testRegistry + ":v3-bundle"

	// GitHub Actions OIDC configuration for keyless signing
	githubWorkflowID = "https://github.com/lucchmielowski/kyverno-cosign-testbed/.github/workflows/ci.yml@refs/heads/main"
)

const (
	// testPublicKey is defined in opts_test.go and shared across test files
	githubActionsIssuer = "https://token.actions.githubusercontent.com"
)

// Test backward compatibility with existing images
func Test_ImageSignatureVerificationKeyless(t *testing.T) {
	idf, err := imagedataloader.New(nil)
	require.NoError(t, err)

	img, err := idf.FetchImageData(context.TODO(), v2KeylessImage)
	if err != nil {
		t.Skipf("test image not accessible: %v", err)
	}

	attestor := &v1beta1.Attestor{
		Name: "test-keyless",
		Cosign: &v1beta1.Cosign{
			Keyless: &v1beta1.Keyless{
				Identities: []v1beta1.Identity{
					{
						Issuer:  githubActionsIssuer,
						Subject: githubWorkflowID,
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
	assert.NoError(t, err, "keyless signature verification should succeed")
}

func Test_ImageSignatureVerificationFail(t *testing.T) {
	idf, err := imagedataloader.New(nil)
	require.NoError(t, err)

	img, err := idf.FetchImageData(context.TODO(), v2KeylessImage)
	if err != nil {
		t.Skipf("test image not accessible: %v", err)
	}

	// Use wrong subject - should fail verification
	attestor := &v1beta1.Attestor{
		Name: "test-wrong-identity",
		Cosign: &v1beta1.Cosign{
			Keyless: &v1beta1.Keyless{
				Identities: []v1beta1.Identity{
					{
						Issuer:  githubActionsIssuer,
						Subject: "https://github.com/wrong/repo/.github/workflows/wrong.yml@refs/heads/main",
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
	assert.Error(t, err, "verification should fail with wrong keyless identity")
}

func Test_ImageSignatureVerificationKeyed(t *testing.T) {
	idf, err := imagedataloader.New(nil)
	require.NoError(t, err)

	img, err := idf.FetchImageData(context.TODO(), v2KeyBasedImage)
	if err != nil {
		t.Skipf("test image not accessible: %v", err)
	}

	attestor := &v1beta1.Attestor{
		Name: "test-keyed",
		Cosign: &v1beta1.Cosign{
			Key: &v1beta1.Key{
				Data: testPublicKey,
			},
			CTLog: &v1beta1.CTLog{
				URL:                "https://rekor.sigstore.dev",
				InsecureIgnoreTlog: true,
				InsecureIgnoreSCT:  true,
			},
		},
	}

	v := Verifier{log: logr.Discard()}
	err = v.VerifyImageSignature(context.TODO(), img, attestor)
	assert.NoError(t, err, "key-based signature verification should succeed")
}

func Test_ImageSignatureVerificationKeyedFail(t *testing.T) {
	idf, err := imagedataloader.New(nil)
	require.NoError(t, err)

	img, err := idf.FetchImageData(context.TODO(), v2KeyBasedImage)
	if err != nil {
		t.Skipf("test image not accessible: %v", err)
	}

	// Use wrong public key - should fail verification
	wrongKey := `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEoKYkkX32oSx61B4iwKXa6llAF2dB
IoL3R/9n1SJ7s00Nfkk3z4/Ar6q8el/guUmXi8akEJMxvHnvphorVUz8vQ==
-----END PUBLIC KEY-----`

	attestor := &v1beta1.Attestor{
		Name: "test-wrong-key",
		Cosign: &v1beta1.Cosign{
			Key: &v1beta1.Key{
				Data: wrongKey,
			},
			CTLog: &v1beta1.CTLog{
				URL:                "https://rekor.sigstore.dev",
				InsecureIgnoreTlog: true,
				InsecureIgnoreSCT:  true,
			},
		},
	}

	v := Verifier{log: logr.Discard()}
	err = v.VerifyImageSignature(context.TODO(), img, attestor)
	assert.Error(t, err, "verification should fail with wrong public key")
}

func TestCosign_V3_KeyBased(t *testing.T) {
	t.Run("v3 key-based signature verification", func(t *testing.T) {
		idf, err := imagedataloader.New(nil)
		require.NoError(t, err)

		img, err := idf.FetchImageData(context.TODO(), v3KeyBasedImage)
		if err != nil {
			t.Skipf("test image not accessible: %v", err)
		}

		attestor := &v1beta1.Attestor{
			Name: "v3-key-based",
			Cosign: &v1beta1.Cosign{
				Key: &v1beta1.Key{
					Data: testPublicKey,
				},
				CTLog: &v1beta1.CTLog{
					URL:                "https://rekor.sigstore.dev",
					InsecureIgnoreTlog: true,
					InsecureIgnoreSCT:  true,
				},
			},
		}

		v := Verifier{log: logr.Discard()}
		err = v.VerifyImageSignature(context.TODO(), img, attestor)
		assert.NoError(t, err, "cosign v3 traditional signature should verify successfully")
	})
}

func TestCosign_V3_Keyless(t *testing.T) {
	t.Run("v3 keyless OIDC signature verification", func(t *testing.T) {
		idf, err := imagedataloader.New(nil)
		require.NoError(t, err)

		img, err := idf.FetchImageData(context.TODO(), v3KeylessImage)
		if err != nil {
			t.Skipf("test image not accessible: %v", err)
		}

		attestor := &v1beta1.Attestor{
			Name: "v3-keyless",
			Cosign: &v1beta1.Cosign{
				Keyless: &v1beta1.Keyless{
					Identities: []v1beta1.Identity{
						{
							Issuer:  githubActionsIssuer,
							Subject: githubWorkflowID,
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
		assert.NoError(t, err, "cosign v3 keyless signature should verify successfully")
	})
}

func TestCosign_V3_MultiPlatform(t *testing.T) {
	t.Run("v3 digest-based signature for multi-platform images", func(t *testing.T) {
		idf, err := imagedataloader.New(nil)
		require.NoError(t, err)

		img, err := idf.FetchImageData(context.TODO(), v3BundleImage)
		if err != nil {
			t.Skipf("test image not accessible: %v", err)
		}

		attestor := &v1beta1.Attestor{
			Name: "v3-multiplatform",
			Cosign: &v1beta1.Cosign{
				Key: &v1beta1.Key{
					Data: testPublicKey,
				},
				CTLog: &v1beta1.CTLog{
					URL:                "https://rekor.sigstore.dev",
					InsecureIgnoreTlog: true,
					InsecureIgnoreSCT:  true,
				},
			},
		}

		v := Verifier{log: logr.Discard()}
		err = v.VerifyImageSignature(context.TODO(), img, attestor)
		assert.NoError(t, err, "cosign v3 digest-based signature should verify successfully")
	})
}

func TestBackwardCompatibility_V2toV3(t *testing.T) {
	t.Run("verify v2 and v3 traditional with same attestor", func(t *testing.T) {
		idf, err := imagedataloader.New(nil)
		require.NoError(t, err)

		attestor := &v1beta1.Attestor{
			Name: "compat-test",
			Cosign: &v1beta1.Cosign{
				Key: &v1beta1.Key{
					Data: testPublicKey,
				},
				CTLog: &v1beta1.CTLog{
					URL:                "https://rekor.sigstore.dev",
					InsecureIgnoreTlog: true,
					InsecureIgnoreSCT:  true,
				},
			},
		}

		v := Verifier{log: logr.Discard()}

		// Test v2 image
		imgV2, err := idf.FetchImageData(context.TODO(), v2KeyBasedImage)
		if err == nil {
			err = v.VerifyImageSignature(context.TODO(), imgV2, attestor)
			assert.NoError(t, err, "v2 image should verify with cosign v3 library")
		} else {
			t.Logf("v2 image not accessible, skipping: %v", err)
		}

		// Test v3 image
		imgV3, err := idf.FetchImageData(context.TODO(), v3KeyBasedImage)
		if err == nil {
			err = v.VerifyImageSignature(context.TODO(), imgV3, attestor)
			assert.NoError(t, err, "v3 image should verify with cosign v3 library")
		} else {
			t.Logf("v3 image not accessible, skipping: %v", err)
		}
	})

	t.Run("verify v2 and v3 keyless with same attestor", func(t *testing.T) {
		idf, err := imagedataloader.New(nil)
		require.NoError(t, err)

		attestor := &v1beta1.Attestor{
			Name: "keyless-compat-test",
			Cosign: &v1beta1.Cosign{
				Keyless: &v1beta1.Keyless{
					Identities: []v1beta1.Identity{
						{
							Issuer:  githubActionsIssuer,
							Subject: githubWorkflowID,
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

		// Test v2 keyless image
		imgV2, err := idf.FetchImageData(context.TODO(), v2KeylessImage)
		if err == nil {
			err = v.VerifyImageSignature(context.TODO(), imgV2, attestor)
			assert.NoError(t, err, "v2 keyless image should verify with cosign v3 library")
		} else {
			t.Logf("v2 keyless image not accessible, skipping: %v", err)
		}

		// Test v3 keyless image
		imgV3, err := idf.FetchImageData(context.TODO(), v3KeylessImage)
		if err == nil {
			err = v.VerifyImageSignature(context.TODO(), imgV3, attestor)
			assert.NoError(t, err, "v3 keyless image should verify with cosign v3 library")
		} else {
			t.Logf("v3 keyless image not accessible, skipping: %v", err)
		}
	})
}

func TestBundleAutoDetection(t *testing.T) {
	t.Run("auto-detection should work for all image types", func(t *testing.T) {
		idf, err := imagedataloader.New(nil)
		require.NoError(t, err)

		keyAttestor := &v1beta1.Attestor{
			Name: "autodetect-key",
			Cosign: &v1beta1.Cosign{
				Key: &v1beta1.Key{
					Data: testPublicKey,
				},
				CTLog: &v1beta1.CTLog{
					URL:                "https://rekor.sigstore.dev",
					InsecureIgnoreTlog: true,
					InsecureIgnoreSCT:  true,
				},
			},
		}

		keylessAttestor := &v1beta1.Attestor{
			Name: "autodetect-keyless",
			Cosign: &v1beta1.Cosign{
				Keyless: &v1beta1.Keyless{
					Identities: []v1beta1.Identity{
						{
							Issuer:  githubActionsIssuer,
							Subject: githubWorkflowID,
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

		testCases := []struct {
			name     string
			image    string
			attestor *v1beta1.Attestor
		}{
			{"v2-traditional", v2KeyBasedImage, keyAttestor},
			{"v2-keyless", v2KeylessImage, keylessAttestor},
			{"v3-traditional", v3KeyBasedImage, keyAttestor},
			{"v3-keyless", v3KeylessImage, keylessAttestor},
			{"v3-bundle", v3BundleImage, keyAttestor},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				img, err := idf.FetchImageData(context.TODO(), tc.image)
				if err != nil {
					t.Skipf("image %s not accessible: %v", tc.image, err)
				}

				err = v.VerifyImageSignature(context.TODO(), img, tc.attestor)
				assert.NoError(t, err, "auto-detection should handle %s correctly", tc.name)
			})
		}
	})
}

func TestMultiPlatformImages(t *testing.T) {
	t.Run("multi-platform manifest list verification", func(t *testing.T) {
		idf, err := imagedataloader.New(nil)
		require.NoError(t, err)

		attestor := &v1beta1.Attestor{
			Name: "multiplatform-test",
			Cosign: &v1beta1.Cosign{
				Key: &v1beta1.Key{
					Data: testPublicKey,
				},
				CTLog: &v1beta1.CTLog{
					URL:                "https://rekor.sigstore.dev",
					InsecureIgnoreTlog: true,
					InsecureIgnoreSCT:  true,
				},
			},
		}

		v := Verifier{log: logr.Discard()}

		img, err := idf.FetchImageData(context.TODO(), v3BundleImage)
		if err != nil {
			t.Skipf("v3-bundle image not accessible: %v", err)
		}

		err = v.VerifyImageSignature(context.TODO(), img, attestor)
		assert.NoError(t, err, "multi-platform image with digest-based signature should verify")
	})
}

func TestNegative_UnsignedImage(t *testing.T) {
	t.Run("unsigned image should fail verification", func(t *testing.T) {
		idf, err := imagedataloader.New(nil)
		require.NoError(t, err)

		img, err := idf.FetchImageData(context.TODO(), unsignedImage)
		if err != nil {
			t.Skipf("unsigned image not accessible: %v", err)
		}

		attestor := &v1beta1.Attestor{
			Name: "unsigned-test",
			Cosign: &v1beta1.Cosign{
				Key: &v1beta1.Key{
					Data: testPublicKey,
				},
				CTLog: &v1beta1.CTLog{
					URL:                "https://rekor.sigstore.dev",
					InsecureIgnoreTlog: true,
					InsecureIgnoreSCT:  true,
				},
			},
		}

		v := Verifier{log: logr.Discard()}
		err = v.VerifyImageSignature(context.TODO(), img, attestor)
		assert.Error(t, err, "unsigned image should fail verification")
	})
}

func TestNegative_WrongPublicKey(t *testing.T) {
	t.Run("wrong public key should fail verification", func(t *testing.T) {
		idf, err := imagedataloader.New(nil)
		require.NoError(t, err)

		img, err := idf.FetchImageData(context.TODO(), v3KeyBasedImage)
		if err != nil {
			t.Skipf("v3-traditional image not accessible: %v", err)
		}

		wrongKey := `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEoKYkkX32oSx61B4iwKXa6llAF2dB
IoL3R/9n1SJ7s00Nfkk3z4/Ar6q8el/guUmXi8akEJMxvHnvphorVUz8vQ==
-----END PUBLIC KEY-----`

		attestor := &v1beta1.Attestor{
			Name: "wrong-key-test",
			Cosign: &v1beta1.Cosign{
				Key: &v1beta1.Key{
					Data: wrongKey,
				},
				CTLog: &v1beta1.CTLog{
					URL:                "https://rekor.sigstore.dev",
					InsecureIgnoreTlog: true,
					InsecureIgnoreSCT:  true,
				},
			},
		}

		v := Verifier{log: logr.Discard()}
		err = v.VerifyImageSignature(context.TODO(), img, attestor)
		assert.Error(t, err, "wrong public key should fail verification")
	})
}

func TestNegative_WrongKeylessIdentity(t *testing.T) {
	t.Run("wrong keyless identity should fail verification", func(t *testing.T) {
		idf, err := imagedataloader.New(nil)
		require.NoError(t, err)

		img, err := idf.FetchImageData(context.TODO(), v3KeylessImage)
		if err != nil {
			t.Skipf("v3-keyless image not accessible: %v", err)
		}

		attestor := &v1beta1.Attestor{
			Name: "wrong-identity-test",
			Cosign: &v1beta1.Cosign{
				Keyless: &v1beta1.Keyless{
					Identities: []v1beta1.Identity{
						{
							Issuer:  githubActionsIssuer,
							Subject: "https://github.com/wrong/repo/.github/workflows/wrong.yml@refs/heads/main",
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
		assert.Error(t, err, "wrong keyless identity should fail verification")
	})
}

func TestConcurrentVerification(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrent test in short mode")
	}

	t.Run("concurrent verification of multiple images", func(t *testing.T) {
		idf, err := imagedataloader.New(nil)
		require.NoError(t, err)

		// Pre-check that all images are accessible before running concurrent tests
		images := []string{
			v2KeyBasedImage,
			v3KeyBasedImage,
			v3BundleImage,
		}

		for _, image := range images {
			_, err := idf.FetchImageData(context.TODO(), image)
			if err != nil {
				t.Skipf("image %s not accessible: %v", image, err)
			}
		}

		attestor := &v1beta1.Attestor{
			Name: "concurrent-test",
			Cosign: &v1beta1.Cosign{
				Key: &v1beta1.Key{
					Data: testPublicKey,
				},
				CTLog: &v1beta1.CTLog{
					URL:                "https://rekor.sigstore.dev",
					InsecureIgnoreTlog: true,
					InsecureIgnoreSCT:  true,
				},
			},
		}

		type result struct {
			image   string
			success bool
			err     error
		}

		results := make(chan result, len(images))

		for _, image := range images {
			go func(img string) {
				imgData, err := idf.FetchImageData(context.TODO(), img)
				if err != nil {
					results <- result{image: img, success: false, err: err}
					return
				}

				v := Verifier{log: logr.Discard()}
				err = v.VerifyImageSignature(context.TODO(), imgData, attestor)
				results <- result{image: img, success: err == nil, err: err}
			}(image)
		}

		var failures []result
		successes := 0
		for i := 0; i < len(images); i++ {
			res := <-results
			if res.success {
				successes++
			} else {
				failures = append(failures, res)
			}
		}

		if len(failures) > 0 {
			for _, failure := range failures {
				t.Errorf("verification failed for image %s: %v", failure.image, failure.err)
			}
		}

		assert.Equal(t, len(images), successes, "all concurrent verifications should succeed")
	})
}

func TestVerifyImageSignature_ErrorCases(t *testing.T) {
	t.Run("nil cosign attestor", func(t *testing.T) {
		idf, err := imagedataloader.New(nil)
		require.NoError(t, err)

		img, err := idf.FetchImageData(context.TODO(), v2KeyBasedImage)
		if err != nil {
			t.Skipf("test image not accessible: %v", err)
		}

		attestor := &v1beta1.Attestor{
			Name:   "nil-cosign",
			Cosign: nil,
		}

		v := Verifier{log: logr.Discard()}
		err = v.VerifyImageSignature(context.TODO(), img, attestor)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cosign verifier only supports cosign attestor")
	})

	t.Run("invalid key data in checkOptions", func(t *testing.T) {
		idf, err := imagedataloader.New(nil)
		require.NoError(t, err)

		img, err := idf.FetchImageData(context.TODO(), v2KeyBasedImage)
		if err != nil {
			t.Skipf("test image not accessible: %v", err)
		}

		attestor := &v1beta1.Attestor{
			Name: "invalid-key",
			Cosign: &v1beta1.Cosign{
				Key: &v1beta1.Key{
					Data: "invalid-key-data",
				},
				CTLog: &v1beta1.CTLog{
					URL:                "https://rekor.sigstore.dev",
					InsecureIgnoreTlog: true,
					InsecureIgnoreSCT:  true,
				},
			},
		}

		v := Verifier{log: logr.Discard()}
		err = v.VerifyImageSignature(context.TODO(), img, attestor)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to build cosign verification opts")
	})

	t.Run("empty key data", func(t *testing.T) {
		idf, err := imagedataloader.New(nil)
		require.NoError(t, err)

		img, err := idf.FetchImageData(context.TODO(), v2KeyBasedImage)
		if err != nil {
			t.Skipf("test image not accessible: %v", err)
		}

		attestor := &v1beta1.Attestor{
			Name: "empty-key",
			Cosign: &v1beta1.Cosign{
				Key: &v1beta1.Key{
					Data: "",
				},
				CTLog: &v1beta1.CTLog{
					URL:                "https://rekor.sigstore.dev",
					InsecureIgnoreTlog: true,
					InsecureIgnoreSCT:  true,
				},
			},
		}

		v := Verifier{log: logr.Discard()}
		err = v.VerifyImageSignature(context.TODO(), img, attestor)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to verify cosign signatures")
	})
}

func TestVerifyAttestationSignature_ErrorCases(t *testing.T) {
	t.Run("nil cosign attestor", func(t *testing.T) {
		idf, err := imagedataloader.New(nil)
		require.NoError(t, err)

		img, err := idf.FetchImageData(context.TODO(), v2KeyBasedImage)
		if err != nil {
			t.Skipf("test image not accessible: %v", err)
		}

		attestor := &v1beta1.Attestor{
			Name:   "nil-cosign",
			Cosign: nil,
		}

		attestation := &v1beta1.Attestation{
			Name: "test-attestation",
			InToto: &v1beta1.InToto{
				Type: "slsaprovenance",
			},
		}

		v := Verifier{log: logr.Discard()}
		err = v.VerifyAttestationSignature(context.TODO(), img, attestation, attestor)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cosign verifier only supports cosign attestor")
	})

	t.Run("invalid key data in attestation verification", func(t *testing.T) {
		idf, err := imagedataloader.New(nil)
		require.NoError(t, err)

		img, err := idf.FetchImageData(context.TODO(), v2KeyBasedImage)
		if err != nil {
			t.Skipf("test image not accessible: %v", err)
		}

		attestor := &v1beta1.Attestor{
			Name: "invalid-key-attestation",
			Cosign: &v1beta1.Cosign{
				Key: &v1beta1.Key{
					Data: "invalid-key-data",
				},
				CTLog: &v1beta1.CTLog{
					URL:                "https://rekor.sigstore.dev",
					InsecureIgnoreTlog: true,
					InsecureIgnoreSCT:  true,
				},
			},
		}

		attestation := &v1beta1.Attestation{
			Name: "test-attestation",
			InToto: &v1beta1.InToto{
				Type: "slsaprovenance",
			},
		}

		v := Verifier{log: logr.Discard()}
		err = v.VerifyAttestationSignature(context.TODO(), img, attestation, attestor)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to build cosign verification opts")
	})
}

func Test_GitHubAttestationVerification(t *testing.T) {
	t.Run("verify SLSA provenance attestation with GitHub Actions keyless", func(t *testing.T) {
		idf, err := imagedataloader.New(nil)
		require.NoError(t, err)

		// Use an image that should have SLSA provenance attestations
		// Based on the policy: ghcr.io/lucchmielowski/kyverno-cosign-testbed:*
		// The image must have SLSA provenance attestations signed with GitHub Actions
		img, err := idf.FetchImageData(context.TODO(), githubAttestationImage)
		if err != nil {
			t.Skipf("test image %s not accessible: %v", githubAttestationImage, err)
		}

		attestor := &v1beta1.Attestor{
			Name: "github-keyless-attestation",
			Cosign: &v1beta1.Cosign{
				Keyless: &v1beta1.Keyless{
					Identities: []v1beta1.Identity{
						{
							Issuer:  githubActionsIssuer,
							Subject: githubWorkflowID,
						},
					},
				},
				CTLog: &v1beta1.CTLog{
					URL:               "https://rekor.sigstore.dev",
					InsecureIgnoreSCT: true,
				},
			},
		}

		attestation := &v1beta1.Attestation{
			Name: "slsa",
			InToto: &v1beta1.InToto{
				Type: "https://slsa.dev/provenance/v1",
			},
		}

		v := Verifier{log: logr.Discard()}
		err = v.VerifyAttestationSignature(context.TODO(), img, attestation, attestor)
		assert.NoError(t, err, "SLSA provenance attestation verification should succeed with GitHub Actions keyless")
	})
}
