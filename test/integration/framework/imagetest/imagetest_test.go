package imagetest

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_PushSubjectReferrerAndPinnedRef(t *testing.T) {
	reg := NewRegistry(t)
	subject := reg.PushSubject(t, "v1")
	require.NotEmpty(t, subject.Digest.Hex)
	assert.Greater(t, subject.Size, int64(0))

	assert.Equal(t, reg.Repo.Tag("v1").String()+"@"+subject.Digest.String(), reg.PinnedRef("v1", subject))
	assert.Equal(t, reg.Repo.Digest(subject.Digest.String()).String(), reg.PinnedRef("", subject))

	payload := []byte(`{"test":"bundle"}`)
	reg.PushReferrer(t, subject, payload, BundleMediaType, BundleMediaType)

	index, err := remote.Referrers(reg.Repo.Digest(subject.Digest.String()))
	require.NoError(t, err)
	im, err := index.IndexManifest()
	require.NoError(t, err)
	require.Len(t, im.Manifests, 1)

	refDesc, err := remote.Get(reg.Repo.Digest(im.Manifests[0].Digest.String()))
	require.NoError(t, err)
	refImg, err := refDesc.Image()
	require.NoError(t, err)
	refManifest, err := refImg.Manifest()
	require.NoError(t, err)
	assert.Equal(t, BundleMediaType, refManifest.ArtifactType)
	require.Len(t, refManifest.Layers, 1)
	assert.Equal(t, BundleMediaType, string(refManifest.Layers[0].MediaType))
	assert.Equal(t, int64(len(payload)), refManifest.Layers[0].Size)

	// Fetching the image config blob exercises the BlobGets counter on GET /blobs/sha256:...
	img, err := remote.Image(reg.Repo.Tag("v1"))
	require.NoError(t, err)
	_, err = img.RawConfigFile()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, reg.BlobGets.Load(), int64(1))
}

func TestVirtualSigstore_TrustedRootAndSigning(t *testing.T) {
	reg := NewRegistry(t)
	vs := NewVirtualSigstore(t)

	t.Run("trusted root composition", func(t *testing.T) {
		fullTR, err := root.NewTrustedRootFromJSON([]byte(vs.TrustedRootJSON(t)))
		require.NoError(t, err)
		assert.NotEmpty(t, fullTR.FulcioCertificateAuthorities())
		assert.NotEmpty(t, fullTR.TimestampingAuthorities())
		assert.Empty(t, fullTR.RekorLogs())
		assert.Empty(t, fullTR.CTLogs())

		caOnlyTR, err := root.NewTrustedRootFromJSON([]byte(ComposeTrustedRootJSON(t, vs, nil)))
		require.NoError(t, err)
		assert.NotEmpty(t, caOnlyTR.FulcioCertificateAuthorities())
		assert.Empty(t, caOnlyTR.TimestampingAuthorities())

		tsaOnlyTR, err := root.NewTrustedRootFromJSON([]byte(ComposeTrustedRootJSON(t, nil, vs)))
		require.NoError(t, err)
		assert.Empty(t, tsaOnlyTR.FulcioCertificateAuthorities())
		assert.NotEmpty(t, tsaOnlyTR.TimestampingAuthorities())
	})

	t.Run("sign keyless v0.3 bundle with default and custom predicate", func(t *testing.T) {
		subject := reg.PushSubject(t, "bundle")
		tests := []struct {
			input string
			want  string
		}{
			{input: "", want: DefaultPredicateType},
			{input: "https://example.com/custom/v1", want: "https://example.com/custom/v1"},
		}
		for _, tt := range tests {
			raw := vs.SignKeylessBundle(t, subject.Digest.Hex, DefaultSubject, DefaultIssuer, tt.input)
			var parsed struct {
				MediaType            string `json:"mediaType"`
				VerificationMaterial any    `json:"verificationMaterial"`
				DSSEEnvelope         struct {
					Payload string `json:"payload"`
				} `json:"dsseEnvelope"`
			}
			require.NoError(t, json.Unmarshal(raw, &parsed))
			assert.Equal(t, BundleMediaType, parsed.MediaType)
			assert.NotNil(t, parsed.VerificationMaterial)

			payloadBytes, err := base64.StdEncoding.DecodeString(parsed.DSSEEnvelope.Payload)
			require.NoError(t, err)
			var stmt struct {
				PredicateType string `json:"predicateType"`
			}
			require.NoError(t, json.Unmarshal(payloadBytes, &stmt))
			assert.Equal(t, tt.want, stmt.PredicateType)
		}
	})

	t.Run("sign legacy v2 keyless signature", func(t *testing.T) {
		subject := reg.PushSubject(t, "legacy")
		vs.SignLegacyKeyless(t, reg.Repo, "legacy", DefaultSubject, DefaultIssuer)

		sigTag := strings.Replace(subject.Digest.String(), ":", "-", 1) + ".sig"
		sigImg, err := remote.Image(reg.Repo.Tag(sigTag))
		require.NoError(t, err)
		layers, err := sigImg.Layers()
		require.NoError(t, err)
		require.Len(t, layers, 1)
	})
}
