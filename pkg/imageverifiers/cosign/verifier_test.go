package cosign

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/imagedataloader"
	"github.com/stretchr/testify/assert"
)

func Test_ImageSignatureVerificationStandard(t *testing.T) {
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

	v := cosignVerifier{log: logr.Discard()}
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

	v := cosignVerifier{log: logr.Discard()}
	err = v.VerifyImageSignature(context.TODO(), img, attestor)
	assert.ErrorContains(t, err, "no matching signatures: none of the expected identities matched what was in the certificate")
}
