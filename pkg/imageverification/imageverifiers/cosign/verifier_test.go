package cosign

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	"github.com/stretchr/testify/assert"
)

func Test_ImageSignatureVerificationKeyless(t *testing.T) {
	image := "ghcr.io/jimbugwadia/pause2"
	idf, err := imagedataloader.New(nil)
	assert.NoError(t, err)
	img, err := idf.FetchImageData(context.TODO(), image)
	assert.NoError(t, err)

	attestor := &v1alpha1.Attestor{
		Name: "test",
		Cosign: &v1alpha1.Cosign{
			Keyless: &v1alpha1.Keyless{
				Identities: []v1alpha1.Identity{
					{
						Issuer:  "https://github.com/login/oauth",
						Subject: "jim@nirmata.com",
					},
				},
			},
			CTLog: &v1alpha1.CTLog{
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

	attestor := &v1alpha1.Attestor{
		Name: "test",
		Cosign: &v1alpha1.Cosign{
			Keyless: &v1alpha1.Keyless{
				Identities: []v1alpha1.Identity{
					{
						Issuer:  "https://github.com/login/oauth",
						Subject: "jim@invalid.com",
					},
				},
			},
			CTLog: &v1alpha1.CTLog{
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

	attestor := &v1alpha1.Attestor{
		Name: "test",
		Cosign: &v1alpha1.Cosign{
			Key: &v1alpha1.Key{
				Data: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
-----END PUBLIC KEY-----`,
			},
			CTLog: &v1alpha1.CTLog{
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

	attestor := &v1alpha1.Attestor{
		Name: "test",
		Cosign: &v1alpha1.Cosign{
			Key: &v1alpha1.Key{
				Data: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEoKYkkX32oSx61B4iwKXa6llAF2dB
IoL3R/9n1SJ7s00Nfkk3z4/Ar6q8el/guUmXi8akEJMxvHnvphorVUz8vQ==
-----END PUBLIC KEY-----`,
			},
			CTLog: &v1alpha1.CTLog{
				URL:                "https://rekor.sigstore.dev",
				InsecureIgnoreTlog: true,
			},
		},
	}

	v := Verifier{log: logr.Discard()}
	err = v.VerifyImageSignature(context.TODO(), img, attestor)
	assert.ErrorContains(t, err, "failed to verify cosign signatures")
}
