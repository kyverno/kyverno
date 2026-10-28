package cosign

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/registryclient"
	"github.com/kyverno/sdk/extensions/imagedataloader"
	ociremote "github.com/sigstore/cosign/v3/pkg/oci/remote"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

type sourceScopeTransport struct {
	target  string
	blocked bool
	calls   int
}

func (transport *sourceScopeTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.calls++
	if transport.blocked {
		return nil, registryclient.ErrRegistryAddressBlocked
	}
	forwarded := request.Clone(request.Context())
	forwarded.URL.Host = transport.target
	forwarded.URL.Scheme = "http"
	return http.DefaultTransport.RoundTrip(forwarded)
}

func TestPolicySignatureRepositoryUsesNativeOptions(t *testing.T) {
	server := httptest.NewServer(registry.New())
	defer server.Close()
	repository := strings.TrimPrefix(server.URL, "http://") + "/signatures"
	reference, err := name.ParseReference(repository + ":tag")
	require.NoError(t, err)
	require.NoError(t, remote.Write(reference, empty.Image))
	native, names := registryclient.GlobalImageDataOptions()
	guarded, _ := registryclient.ResourceImageOptions()
	guardedBacking := append(guarded, nil)
	guarded = guardedBacking[:len(guarded)]
	attestor := &v1beta1.Cosign{
		Key:   &v1beta1.Key{Data: testPublicKey},
		CTLog: &v1beta1.CTLog{InsecureIgnoreTlog: true},
	}
	opts, err := checkOptions(context.Background(), attestor, guarded, names, nil)
	require.NoError(t, err)
	require.Nil(t, guardedBacking[len(guarded)], "checkOptions must not write into the caller's spare capacity")
	_, err = ociremote.Signatures(reference, opts.RegistryClientOpts...)
	require.ErrorIs(t, err, registryclient.ErrRegistryAddressBlocked)
	attestor.Source = &v1beta1.Source{SignaturePullSecrets: []corev1.LocalObjectReference{{Name: "signatures"}}}
	_, err = checkOptions(context.Background(), attestor, guarded, names, nil)
	require.NoError(t, err)
	require.Nil(t, guardedBacking[len(guarded)], "source credentials must not write into the caller's spare capacity")
	attestor.Source = &v1beta1.Source{Repository: repository}
	opts, err = checkOptions(context.Background(), attestor, native, names, nil)
	require.NoError(t, err)
	_, err = ociremote.Signatures(reference, opts.RegistryClientOpts...)
	require.NoError(t, err, "policy-controlled repositories must not inherit the workload guard")
}

func TestPolicyPublicKeyURLUsesUpstreamLoader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(testPublicKey))
	}))
	defer server.Close()
	opts, err := checkOptions(context.Background(), &v1beta1.Cosign{
		Key:   &v1beta1.Key{KMS: server.URL + "/key.pem"},
		CTLog: &v1beta1.CTLog{InsecureIgnoreTlog: true},
	}, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, opts.SigVerifier)
}

func TestPolicyRepositoryNeverReresolvesWorkloadTag(t *testing.T) {
	var originCalls, repositoryCalls atomic.Int32
	registryHandler := registry.New()
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		originCalls.Add(1)
		registryHandler.ServeHTTP(writer, request)
	}))
	defer origin.Close()
	imageReference := strings.TrimPrefix(origin.URL, "http://") + "/workload:tag"
	reference, err := name.ParseReference(imageReference)
	require.NoError(t, err)
	require.NoError(t, remote.Write(reference, empty.Image))
	transport := &sourceScopeTransport{target: strings.TrimPrefix(origin.URL, "http://")}
	images, err := imagedataloader.NewImageContext(nil, []remote.Option{remote.WithTransport(transport)}, nil)
	require.NoError(t, err)
	image, err := images.Get(context.Background(), imageReference, nil, nil)
	require.NoError(t, err)
	transport.blocked = true
	repository := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		repositoryCalls.Add(1)
		if request.URL.Path == "/v2/" {
			writer.WriteHeader(http.StatusOK)
			return
		}
		writer.WriteHeader(http.StatusNotFound)
	}))
	defer repository.Close()
	native, _ := registryclient.GlobalImageDataOptions()
	verifier := NewVerifier(nil, logr.Discard(), native)
	attestor := &v1beta1.Attestor{Name: "key", Cosign: &v1beta1.Cosign{
		Key:   &v1beta1.Key{Data: testPublicKey},
		CTLog: &v1beta1.CTLog{InsecureIgnoreTlog: true},
	}}
	for _, verify := range []func() error{
		func() error { return verifier.VerifyImageSignature(context.Background(), image, attestor) },
		func() error {
			return verifier.VerifyAttestationSignature(context.Background(), image, &v1beta1.Attestation{Name: "sbom", InToto: &v1beta1.InToto{Type: "https://example.test/sbom"}}, attestor)
		},
	} {
		attestor.Cosign.Source = nil
		transport.calls = 0
		verificationError := verify()
		require.ErrorIs(t, verificationError, registryclient.ErrRegistryAddressBlocked)
		require.Positive(t, transport.calls, "default signature and attestation lookups must retain workload options")
		attestor.Cosign.Source = &v1beta1.Source{Repository: strings.TrimPrefix(repository.URL, "http://") + "/signatures"}
		originCalls.Store(0)
		repositoryCalls.Store(0)
		transport.calls = 0
		verificationError = verify()
		require.Error(t, verificationError, "fixture deliberately has no signatures")
		require.NotErrorIs(t, verificationError, registryclient.ErrRegistryAddressBlocked)
		require.Positive(t, repositoryCalls.Load(), "policy-authored repository must be reached: %v", verificationError)
		require.Zero(t, originCalls.Load(), "native policy options must never re-resolve the workload tag")
		require.Zero(t, transport.calls, "explicit repositories must use native options")
	}
}
