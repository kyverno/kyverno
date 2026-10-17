package ivpol

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

// isolatedRegistryTransportTest runs each fixture in its own process: restoring
// the SDK's DefaultTransport does not clear Kyverno's cached egress transports.
// It returns true only in the child, where the fixture can replace the transport.
func isolatedRegistryTransportTest(t *testing.T) bool {
	t.Helper()
	const child = "KYVERNO_TEST_REGISTRY_TRANSPORT"
	if os.Getenv(child) == t.Name() {
		return true
	}
	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+regexp.QuoteMeta(t.Name())+"$", "-test.timeout=1m")
	command.Env = append(os.Environ(), child+"="+t.Name())
	output, err := command.CombinedOutput()
	require.NoError(t, err, "%s", output)
	return false
}

// DialContext survives http.Transport.Clone, unlike RegisterProtocol. The egress
// guard still validates and pins the documentation address before the fixture
// redirects that one address to its local TLS server.
func registryFixtureTransport(t *testing.T, fixture http.RoundTripper) *http.Transport {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response, err := fixture.RoundTrip(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer response.Body.Close()
		for name, values := range response.Header {
			w.Header()[name] = values
		}
		w.WriteHeader(response.StatusCode)
		_, _ = io.Copy(w, response.Body)
	}))
	t.Cleanup(server.Close)
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig.ServerName = "127.0.0.1"
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != "192.0.2.1:443" {
			return nil, fmt.Errorf("unexpected fixture destination %q", address)
		}
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	t.Cleanup(transport.CloseIdleConnections)
	return transport
}
