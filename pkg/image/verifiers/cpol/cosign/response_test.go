package cosign

import (
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/in-toto/in-toto-golang/in_toto"
	"gotest.tools/v3/assert"
)

func TestAttestationsResponse(t *testing.T) {
	t.Parallel()
	desc := &v1.Descriptor{Digest: v1.Hash{Algorithm: "sha256", Hex: strings.Repeat("a", 64)}}

	// verified bundles without an in-toto statement, e.g. message signatures
	resp, err := attestationsResponse([]*verificationResult{{Bundle: &verificationBundle{}, Desc: desc}})
	assert.ErrorContains(t, err, "no in-toto attestations found")
	assert.Assert(t, resp == nil)

	statement := &in_toto.Statement{} //nolint:staticcheck
	statement.PredicateType = "https://slsa.dev/provenance/v1"
	resp, err = attestationsResponse([]*verificationResult{
		{Bundle: &verificationBundle{}, Desc: desc},
		{Bundle: &verificationBundle{DSSE_Envelope: statement}, Desc: desc},
	})
	assert.NilError(t, err)
	assert.Equal(t, resp.Digest, desc.Digest.String())
	assert.Equal(t, len(resp.Statements), 1)
	assert.Equal(t, resp.Statements[0]["type"], "https://slsa.dev/provenance/v1")
}
