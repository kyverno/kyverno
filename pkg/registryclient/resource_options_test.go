package registryclient

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/require"
)

type registryChallengeTransport struct {
	guard        http.RoundTripper
	redirect     bool
	blockedCalls int
}

func (transport *registryChallengeTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Hostname() != "8.8.8.8" {
		transport.blockedCalls++
		return transport.guard.RoundTrip(request)
	}
	header := make(http.Header)
	status := http.StatusUnauthorized
	if transport.redirect {
		status = http.StatusTemporaryRedirect
		header.Set("Location", "http://127.0.0.1:5000/redirect")
	} else {
		header.Set("WWW-Authenticate", `Bearer realm="https://localhost:5000/token",service="registry",scope="repository:example/image:pull"`)
	}
	return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader("")), Request: request}, nil
}

func TestResourceRemoteGetGuardsTokenAndRedirectDestinations(t *testing.T) {
	t.Parallel()
	for _, redirect := range []bool{false, true} {
		label := "token"
		if redirect {
			label = "redirect"
		}
		t.Run(label, func(t *testing.T) {
			t.Parallel()
			configured := New(WithAllowInsecureRegistry(true)).(*client)
			transport := &registryChallengeTransport{guard: configured.transport, redirect: redirect}
			configured.transport = transport
			opts, names := configured.resourceImageOptions()
			reference, err := name.ParseReference("8.8.8.8/example/image:latest", names...)
			require.NoError(t, err)
			_, err = remote.Get(reference, opts...)
			require.ErrorIs(t, err, ErrRegistryAddressBlocked)
			require.Positive(t, transport.blockedCalls)
		})
	}
}
