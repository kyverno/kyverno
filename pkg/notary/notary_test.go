package notary

import (
	"context"
	"testing"

	gcrremote "github.com/google/go-containerregistry/0_14/pkg/v1/remote" // TODO: Remove this once we upgrade tp cosign version < 2.0.2
	"github.com/google/go-containerregistry/pkg/name"
	"gotest.tools/assert"
)

func TestExtractStatements(t *testing.T) {
	imageRef := "jimnotarytest.azurecr.io/jim/net-monitor:v1"
	ref, err := name.ParseReference(imageRef)
	assert.NilError(t, err)
	repoDesc, err := gcrremote.Head(ref)
	assert.NilError(t, err)
	referrers, err := gcrremote.Referrers(ref.Context().Digest(repoDesc.Digest.String()))
	assert.NilError(t, err)
	referrersDescs, err := referrers.IndexManifest()
	assert.NilError(t, err)

	for _, referrer := range referrersDescs.Manifests {
		if referrer.ArtifactType == "application/vnd.cncf.notary.signature" {
			statements, err := extractStatements(context.Background(), ref, referrer, nil)
			assert.NilError(t, err)
			assert.Assert(t, len(statements) == 1)
			assert.Assert(t, statements[0]["type"] == referrer.ArtifactType)
			assert.Assert(t, statements[0]["mediaType"] == string(referrer.MediaType))
		}
	}
}
