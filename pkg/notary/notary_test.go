package notary

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"gotest.tools/assert"
)

func TestExtractStatements(t *testing.T) {
	imageRef := "jimnotarytest.azurecr.io/jim/net-monitor:v1"
	ref, err := name.ParseReference(imageRef)
	assert.NilError(t, err)
	repoDesc, err := crane.Head(imageRef)
	assert.NilError(t, err)
	referrers, err := remote.Referrers(ref.Context().Digest(repoDesc.Digest.String()))
	assert.NilError(t, err)
	referrersDescs, err := referrers.IndexManifest()
	assert.NilError(t, err)

	for _, referrer := range referrersDescs.Manifests {
		if referrer.ArtifactType == "application/vnd.cncf.notary.signature" {
			statements, err := extractStatements(context.Background(), ref, referrer)
			assert.NilError(t, err)
			assert.Assert(t, len(statements) == 1)
			assert.Assert(t, statements[0]["type"] == referrer.ArtifactType)
			assert.Assert(t, statements[0]["mediaType"] == string(referrer.MediaType))
		}
	}
}
