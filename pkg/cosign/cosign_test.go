package cosign

import (
	"testing"

	"github.com/sigstore/cosign/pkg/oci"

	"github.com/go-logr/logr"
	"github.com/sigstore/cosign/pkg/cosign"
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
    "optional": null
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
  "Optional": {}
}`

func TestCosignPayload(t *testing.T) {
	var log logr.Logger = logr.DiscardLogger{}
	image := "registry-v2.nirmata.io/pause"
	signedPayloads := cosign.SignedPayload{Payload: []byte(cosignPayload)}
	d, err := extractDigest(image, []oci.Signature{&sig{cosignPayload: signedPayloads}}, log)
	assert.NilError(t, err)
	assert.Equal(t, d, "sha256:4a1c4b21597c1b4415bdbecb28a3296c6b5e23ca4f9feeb599860a1dac6a0108")

	image2 := "gcr.io/tekton-releases/github.com/tektoncd/pipeline/cmd/nop"
	signedPayloads2 := cosign.SignedPayload{Payload: []byte(tektonPayload)}
	d2, err := extractDigest(image2, []oci.Signature{&sig{cosignPayload: signedPayloads2}}, log)
	assert.NilError(t, err)
	assert.Equal(t, d2, "sha256:6a037d5ba27d9c6be32a9038bfe676fb67d2e4145b4f53e9c61fb3e69f06e816")
}
