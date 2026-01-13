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
	// cosign-testbed repository images
	testbedRegistry      = "ghcr.io/lucchmielowski/cosign-testbed"
	testbedUnsigned      = testbedRegistry + ":unsigned"
	testbedV2Traditional = testbedRegistry + ":v2-traditional"
	testbedV2Keyless     = testbedRegistry + ":v2-keyless"
	testbedV3Traditional = testbedRegistry + ":v3-traditional"
	testbedV3Keyless     = testbedRegistry + ":v3-keyless"
	testbedV3Bundle      = testbedRegistry + ":v3-bundle"

	// Public key from cosign-testbed repository
	testbedPublicKey = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEIOJTQ992VBJyyx52p3s1W/lqwNxI
rFxZI4BL3S6ZGyJFockpfppxOycEkUaGVTUvL0Tp7Yi0eYRJ4TtKxs1lXQ==
-----END PUBLIC KEY-----`

	// GitHub Actions OIDC configuration
	githubActionsIssuer = "https://token.actions.githubusercontent.com"
	testbedWorkflowID   = "https://github.com/lucchmielowski/cosign-testbed/.github/workflows/ci.yml@refs/heads/main"
)

// Test backward compatibility with existing images
func Test_ImageSignatureVerificationKeyless(t *testing.T) {
	idf, err := imagedataloader.New(nil)
	require.NoError(t, err)

	img, err := idf.FetchImageData(context.TODO(), testbedV2Keyless)
	if err != nil {
		t.Skipf("testbed image not accessible: %v", err)
	}

	attestor := &v1beta1.Attestor{
		Name: "test-keyless",
		Cosign: &v1beta1.Cosign{
			Keyless: &v1beta1.Keyless{
				Identities: []v1beta1.Identity{
					{
						Issuer:  githubActionsIssuer,
						Subject: testbedWorkflowID,
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

	img, err := idf.FetchImageData(context.TODO(), testbedV2Keyless)
	if err != nil {
		t.Skipf("testbed image not accessible: %v", err)
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

	img, err := idf.FetchImageData(context.TODO(), testbedV2Traditional)
	if err != nil {
		t.Skipf("testbed image not accessible: %v", err)
	}

	attestor := &v1beta1.Attestor{
		Name: "test-keyed",
		Cosign: &v1beta1.Cosign{
			Key: &v1beta1.Key{
				Data: testbedPublicKey,
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

	img, err := idf.FetchImageData(context.TODO(), testbedV2Traditional)
	if err != nil {
		t.Skipf("testbed image not accessible: %v", err)
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

		img, err := idf.FetchImageData(context.TODO(), testbedV3Traditional)
		if err != nil {
			t.Skipf("test image not accessible: %v", err)
		}

		attestor := &v1beta1.Attestor{
			Name: "v3-key-based",
			Cosign: &v1beta1.Cosign{
				Key: &v1beta1.Key{
					Data: testbedPublicKey,
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

		img, err := idf.FetchImageData(context.TODO(), testbedV3Keyless)
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
							Subject: testbedWorkflowID,
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

		img, err := idf.FetchImageData(context.TODO(), testbedV3Bundle)
		if err != nil {
			t.Skipf("test image not accessible: %v", err)
		}

		attestor := &v1beta1.Attestor{
			Name: "v3-multiplatform",
			Cosign: &v1beta1.Cosign{
				Key: &v1beta1.Key{
					Data: testbedPublicKey,
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
					Data: testbedPublicKey,
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
		imgV2, err := idf.FetchImageData(context.TODO(), testbedV2Traditional)
		if err == nil {
			err = v.VerifyImageSignature(context.TODO(), imgV2, attestor)
			assert.NoError(t, err, "v2 image should verify with cosign v3 library")
		} else {
			t.Logf("v2 image not accessible, skipping: %v", err)
		}

		// Test v3 image
		imgV3, err := idf.FetchImageData(context.TODO(), testbedV3Traditional)
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
							Subject: testbedWorkflowID,
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
		imgV2, err := idf.FetchImageData(context.TODO(), testbedV2Keyless)
		if err == nil {
			err = v.VerifyImageSignature(context.TODO(), imgV2, attestor)
			assert.NoError(t, err, "v2 keyless image should verify with cosign v3 library")
		} else {
			t.Logf("v2 keyless image not accessible, skipping: %v", err)
		}

		// Test v3 keyless image
		imgV3, err := idf.FetchImageData(context.TODO(), testbedV3Keyless)
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
					Data: testbedPublicKey,
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
							Subject: testbedWorkflowID,
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
			{"v2-traditional", testbedV2Traditional, keyAttestor},
			{"v2-keyless", testbedV2Keyless, keylessAttestor},
			{"v3-traditional", testbedV3Traditional, keyAttestor},
			{"v3-keyless", testbedV3Keyless, keylessAttestor},
			{"v3-bundle", testbedV3Bundle, keyAttestor},
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
					Data: testbedPublicKey,
				},
				CTLog: &v1beta1.CTLog{
					URL:                "https://rekor.sigstore.dev",
					InsecureIgnoreTlog: true,
					InsecureIgnoreSCT:  true,
				},
			},
		}

		v := Verifier{log: logr.Discard()}

		img, err := idf.FetchImageData(context.TODO(), testbedV3Bundle)
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

		img, err := idf.FetchImageData(context.TODO(), testbedUnsigned)
		if err != nil {
			t.Skipf("unsigned image not accessible: %v", err)
		}

		attestor := &v1beta1.Attestor{
			Name: "unsigned-test",
			Cosign: &v1beta1.Cosign{
				Key: &v1beta1.Key{
					Data: testbedPublicKey,
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

		img, err := idf.FetchImageData(context.TODO(), testbedV3Traditional)
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

		img, err := idf.FetchImageData(context.TODO(), testbedV3Keyless)
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

		attestor := &v1beta1.Attestor{
			Name: "concurrent-test",
			Cosign: &v1beta1.Cosign{
				Key: &v1beta1.Key{
					Data: testbedPublicKey,
				},
				CTLog: &v1beta1.CTLog{
					URL:                "https://rekor.sigstore.dev",
					InsecureIgnoreTlog: true,
					InsecureIgnoreSCT:  true,
				},
			},
		}

		images := []string{
			testbedV2Traditional,
			testbedV3Traditional,
			testbedV3Bundle,
		}

		done := make(chan bool, len(images))

		for _, image := range images {
			go func(img string) {
				imgData, err := idf.FetchImageData(context.TODO(), img)
				if err != nil {
					done <- false
					return
				}

				v := Verifier{log: logr.Discard()}
				err = v.VerifyImageSignature(context.TODO(), imgData, attestor)
				done <- err == nil
			}(image)
		}

		successes := 0
		for i := 0; i < len(images); i++ {
			if <-done {
				successes++
			}
		}

		assert.Equal(t, len(images), successes, "all concurrent verifications should succeed")
	})
}
