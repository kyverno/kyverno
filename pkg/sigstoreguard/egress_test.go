package sigstoreguard

import (
	"context"
	"crypto"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kyverno/kyverno/pkg/utils/egress"
	"github.com/stretchr/testify/require"
)

func TestSigstoreEndpointsBlockUnsafeAddresses(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, endpoint := range []string{"http://127.0.0.1", "http://169.254.169.254", "http://[::1]"} {
		t.Run(endpoint, func(t *testing.T) {
			_, err := LoadFileOrURL(context.Background(), endpoint+"/root.json")
			require.ErrorIs(t, err, egress.ErrAddressBlocked)
			_, err = PublicKeyFromKeyRefWithHashAlgo(context.Background(), endpoint+"/key.pub", crypto.SHA256)
			require.ErrorIs(t, err, egress.ErrAddressBlocked)
			rekor, err := NewRekorClient(endpoint)
			require.NoError(t, err)
			_, err = rekor.Tlog.GetLogInfo(nil)
			require.ErrorIs(t, err, egress.ErrAddressBlocked)
			_, err = newTUFClient(context.Background(), endpoint, nil, false)
			require.ErrorIs(t, err, egress.ErrAddressBlocked)
		})
	}
}

func TestSigstoreClientsBlockRedirects(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, destination := range []string{"http://169.254.169.254/latest/meta-data", "http://127.0.0.1/private", "http://[::1]/private"} {
		t.Run(destination, func(t *testing.T) {
			var dials atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, destination, http.StatusFound)
			}))
			defer server.Close()
			policy, err := egress.New(egress.Config{})
			require.NoError(t, err)
			transport := policy.WrapTransport(&http.Transport{
				DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
					dials.Add(1)
					if address != "93.184.216.34:80" {
						t.Errorf("unexpected dial to %q", address)
						return nil, fmt.Errorf("unexpected dial to %q", address)
					}
					return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
				},
			})
			defer transport.CloseIdleConnections()
			client := &http.Client{Transport: transport}
			originalDefaultClient := http.DefaultClient.Transport
			originalDefaultTransport := http.DefaultTransport
			_, err = loadFileOrURL(context.Background(), "http://93.184.216.34/root.json", client)
			require.ErrorIs(t, err, egress.ErrAddressBlocked)
			rekor, err := newRekorClient("http://93.184.216.34", client)
			require.NoError(t, err)
			_, err = rekor.Tlog.GetLogInfo(nil)
			require.ErrorIs(t, err, egress.ErrAddressBlocked)
			_, err = newTUFClientWithHTTP(context.Background(), "http://93.184.216.34", nil, false, client, false)
			require.ErrorIs(t, err, egress.ErrAddressBlocked)
			require.Positive(t, dials.Load())
			require.Same(t, originalDefaultTransport, http.DefaultTransport)
			require.Equal(t, originalDefaultClient, http.DefaultClient.Transport)
		})
	}
}

func TestTUFHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := TrustedRootFor(ctx, "https://tuf-repo-cdn.sigstore.dev", nil)
	require.ErrorIs(t, err, context.Canceled)
}

func TestRekorDefaultsToPublicService(t *testing.T) {
	var destination string
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		destination = req.URL.String()
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"rootHash":"00","treeSize":0,"signedTreeHead":""}`)), Request: req}, nil
	})}
	rekor, err := newRekorClient("", client)
	require.NoError(t, err)
	_, err = rekor.Tlog.GetLogInfo(nil)
	require.NoError(t, err)
	require.Equal(t, "https://rekor.sigstore.dev/api/v1/log", destination)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
