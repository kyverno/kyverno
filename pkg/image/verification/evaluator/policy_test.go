package evaluator

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/stretchr/testify/assert"
)

func Test_CheckDigests(t *testing.T) {
	t.Parallel()

	digest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	tests := []struct {
		name         string
		verifyDigest bool
		images       []string
		wantFail     bool
	}{
		{
			name:         "verifyDigest disabled allows tagged image",
			verifyDigest: false,
			images:       []string{"ghcr.io/foo:latest"},
			wantFail:     false,
		},
		{
			name:         "verifyDigest enabled rejects tagged image",
			verifyDigest: true,
			images:       []string{"ghcr.io/foo:latest"},
			wantFail:     true,
		},
		{
			name:         "verifyDigest enabled allows digest image",
			verifyDigest: true,
			images: []string{
				"ghcr.io/foo@" + digest,
			},
			wantFail: false,
		},
		{
			name:         "verifyDigest enabled rejects image without tag or digest",
			verifyDigest: true,
			images: []string{
				"ghcr.io/foo",
			},
			wantFail: true,
		},
		{
			name:         "verifyDigest enabled allows tag with digest",
			verifyDigest: true,
			images: []string{
				"ghcr.io/foo:latest@" + digest,
			},
			wantFail: false,
		},
		{
			name:         "verifyDigest enabled rejects mixed images",
			verifyDigest: true,
			images: []string{
				"ghcr.io/foo@" + digest,
				"ghcr.io/bar:latest",
			},
			wantFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &compiledPolicy{
				verifyDigest: tt.verifyDigest,
				nameOpts:     []name.Option{},
			}

			result, err := c.checkDigests(tt.images)

			assert.NoError(t, err)

			if tt.wantFail {
				assert.NotNil(t, result)
				assert.False(t, result.Result)
				assert.Contains(t, result.Message, "does not have a digest")
			} else {
				assert.Nil(t, result)
			}
		})
	}
}
