package cosign

import (
	"testing"

	"github.com/sigstore/cosign/pkg/cosign"
	"github.com/sigstore/cosign/pkg/oci"
	"gotest.tools/assert"
)

const cosignPayload = `{
  "critical": {
 	   "identity": {
 	     "docker-reference": "registry-v2.nirmata.io/pause"
 	    },
   	"image": {
 	     "docker-manifest-digest": "sha256:4a1c4b21597c1b4415bdbecb28a3296c6b5e23ca4f9feeb599860a1dac6a0108"
 	    },
 	    "type": "cosign container image signature"
    },
    "optional": {
		"foo": "bar",
		"bar": "baz"
	}
}`

const tektonPayload = `{
  "Critical": {
    "Identity": {
      "docker-reference": "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/nop"
    },
    "Image": {
      "Docker-manifest-digest": "sha256:6a037d5ba27d9c6be32a9038bfe676fb67d2e4145b4f53e9c61fb3e69f06e816"
    },
    "Type": "Tekton container signature"
  },
  "Optional": {
	  "Issuer": "https://github.com/login/oauth",
	  "Subject": "https://github.com/mycompany/demo/.github/workflows/ci.yml@refs/heads/main"
  }
}`

func TestCosignPayload(t *testing.T) {
	image := "registry-v2.nirmata.io/pause"
	signedPayloads := cosign.SignedPayload{Payload: []byte(cosignPayload)}
	p, err := extractPayload([]oci.Signature{&sig{cosignPayload: signedPayloads}})
	assert.NilError(t, err)
	a := map[string]string{"foo": "bar"}
	err = checkAnnotations(p, a)
	assert.NilError(t, err)
	d, err := extractDigest(image, p)
	assert.NilError(t, err)
	assert.Equal(t, d, "sha256:4a1c4b21597c1b4415bdbecb28a3296c6b5e23ca4f9feeb599860a1dac6a0108")

	image2 := "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/nop"
	signedPayloads2 := cosign.SignedPayload{Payload: []byte(tektonPayload)}
	signatures2 := []oci.Signature{&sig{cosignPayload: signedPayloads2}}
	p2, err := extractPayload(signatures2)
	assert.NilError(t, err)

	d2, err := extractDigest(image2, p2)
	assert.NilError(t, err)
	assert.Equal(t, d2, "sha256:6a037d5ba27d9c6be32a9038bfe676fb67d2e4145b4f53e9c61fb3e69f06e816")
}

func TestCosignKeyless(t *testing.T) {
	opts := Options{
		ImageRef: "ghcr.io/jimbugwadia/pause2",
		Issuer:   "https://github.com/",
		Subject:  "jim",
	}

	_, err := verifySignature(opts)
	assert.Error(t, err, "subject mismatch: expected jim@nirmata.com, got jim")

	opts.Subject = "jim@nirmata.com"
	_, err = verifySignature(opts)
	assert.Error(t, err, "issuer mismatch: expected https://github.com/login/oauth, got https://github.com/")

	opts.Issuer = "https://github.com/login/oauth"
	_, err = verifySignature(opts)
	assert.NilError(t, err)

}
