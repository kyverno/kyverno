package cosign

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/engine/adapters"
	"github.com/kyverno/kyverno/pkg/image/verifiers"
	"github.com/kyverno/kyverno/pkg/registryclient"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	"github.com/sigstore/rekor/pkg/generated/client/tlog"
	"github.com/stretchr/testify/require"
)

func policyEndpointKey(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func TestPolicyKeyAndRekorEndpointsUseNativeClients(t *testing.T) {
	t.Parallel()
	key := policyEndpointKey(t)
	var keys, logs atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/key" {
			keys.Add(1)
			_, _ = response.Write([]byte(key))
			return
		}
		logs.Add(1)
		http.Error(response, "native Rekor request reached the policy endpoint", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	options, err := buildCosignOptions(context.Background(), verifiers.Options{
		Client: adapters.RegistryClient(registryclient.New(registryclient.WithEgressMode("enforce"))),
		Key:    server.URL + "/key", IgnoreSCT: true,
		RekorURL: server.URL, RekorPubKey: key,
	})
	require.NoError(t, err)
	require.Positive(t, keys.Load())
	_, err = options.RekorClient.Tlog.GetLogInfo(tlog.NewGetLogInfoParamsWithContext(context.Background()))
	require.Error(t, err)
	require.NotErrorIs(t, err, registryclient.ErrRegistryAddressBlocked)
	require.Positive(t, logs.Load())
}

func TestPolicySignatureRepositoryUsesNativeOptions(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(registry.New())
	defer server.Close()
	repository := strings.TrimPrefix(server.URL, "http://") + "/policy/signatures"
	image, err := name.ParseReference(repository + ":latest")
	require.NoError(t, err)
	require.NoError(t, remote.Write(image, empty.Image))
	digest, err := empty.Image.Digest()
	require.NoError(t, err)
	signature, err := name.ParseReference(repository + ":sha256-" + digest.Hex + ".sig")
	require.NoError(t, err)
	require.NoError(t, remote.Write(signature, empty.Image))
	client := adapters.RegistryClient(registryclient.New(registryclient.WithEgressMode("enforce"), registryclient.WithAllowInsecureRegistry(true)))
	for _, override := range []string{"", repository} {
		options, err := buildCosignOptions(context.Background(), verifiers.Options{
			Client: client, Repository: override, Key: policyEndpointKey(t), IgnoreTlog: true, IgnoreSCT: true,
		})
		require.NoError(t, err)
		signatures, err := ociremote.Signatures(signature, options.RegistryClientOpts...)
		if err == nil {
			_, err = signatures.Get()
		}
		if override == "" {
			require.ErrorIs(t, err, registryclient.ErrRegistryAddressBlocked)
		} else {
			require.NoError(t, err)
		}
	}
	for _, reference := range []string{image.String(), image.Context().Digest(digest.String()).String()} {
		options := verifiers.Options{
			Client: client, ImageRef: reference, Repository: repository,
			Key: policyEndpointKey(t), IgnoreTlog: true, IgnoreSCT: true,
		}
		_, err = NewVerifier().VerifySignature(context.Background(), options)
		require.ErrorIs(t, err, registryclient.ErrRegistryAddressBlocked)
		_, err = NewVerifier().FetchAttestations(context.Background(), options)
		require.ErrorIs(t, err, registryclient.ErrRegistryAddressBlocked)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resolved, err := resourceReference(ctx, verifiers.Options{
		Client: client, ImageRef: "8.8.8.8/resource/image@" + digest.String(), Repository: repository,
	})
	require.NoError(t, err)
	require.Equal(t, repository+"@"+digest.String(), resolved.String())
}
