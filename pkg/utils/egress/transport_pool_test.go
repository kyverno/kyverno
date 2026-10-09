package egress

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransportHTTP2PoolsFollowValidatedAddresses(t *testing.T) {
	t.Parallel()
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, r.Proto)
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()
	policy := testPolicy(t, Config{}, nil)
	var generation, dials atomic.Int32
	policy.lookupNetIP = func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr(fmt.Sprintf("192.0.2.%d", generation.Load()+1))}, nil
	}
	transport := policy.WrapTransport(&http.Transport{
		TLSClientConfig:   server.Client().Transport.(*http.Transport).TLSClientConfig.Clone(),
		ForceAttemptHTTP2: true,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			dials.Add(1)
			expected := fmt.Sprintf("192.0.2.%d:443", generation.Load()+1)
			if address != expected {
				return nil, fmt.Errorf("unexpected destination %s, want %s", address, expected)
			}
			return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
		},
	})
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	for _, next := range []int32{0, 0, 1, 1} {
		generation.Store(next)
		request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/v2/", nil)
		require.NoError(t, err)
		response, err := client.Do(request)
		require.NoError(t, err)
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		assert.Equal(t, "HTTP/2.0", string(body))
	}
	assert.Equal(t, int32(2), dials.Load())
}

func TestTransportPoolEvictionPreservesActiveResponses(t *testing.T) {
	t.Parallel()
	policy := testPolicy(t, Config{}, nil)
	var generation atomic.Int32
	policy.lookupNetIP = func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr(fmt.Sprintf("192.0.2.%d", generation.Load()+1))}, nil
	}
	finishBody := make(chan struct{})
	var finishOnce sync.Once
	finish := func() { finishOnce.Do(func() { close(finishBody) }) }
	t.Cleanup(finish)
	firstClosed := make(chan struct{})
	transport := policy.WrapTransport(&http.Transport{DialContext: func(_ context.Context, _, address string) (net.Conn, error) {
		client, server := net.Pipe()
		go func() {
			defer server.Close()
			reader := bufio.NewReader(server)
			for {
				request, err := http.ReadRequest(reader)
				if err != nil {
					if address == "192.0.2.1:80" {
						close(firstClosed)
					}
					return
				}
				request.Body.Close()
				if address == "192.0.2.1:80" {
					if _, err := io.WriteString(server, "HTTP/1.1 200 OK\r\nContent-Length: 4\r\n\r\none"); err != nil {
						return
					}
					<-finishBody
					if _, err := io.WriteString(server, "!"); err != nil {
						return
					}
				} else if _, err := io.WriteString(server, "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok"); err != nil {
					return
				}
			}
		}()
		return client, nil
	}})
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://registry.example/v2/", nil)
	require.NoError(t, err)
	active, err := client.Do(request)
	require.NoError(t, err)
	defer active.Body.Close()
	// Fill the bounded cache with distinct destinations while the first body is
	// still streaming. Eviction must release only idle connections.
	for i := range maxDestinationPools {
		generation.Store(int32(i + 1))
		response, err := client.Do(request.Clone(context.Background()))
		require.NoError(t, err)
		_, err = io.Copy(io.Discard, response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
	}
	finish()
	body, err := io.ReadAll(active.Body)
	require.NoError(t, err)
	assert.Equal(t, "one!", string(body))
	require.NoError(t, active.Body.Close())
	select {
	case <-firstClosed:
	case <-time.After(time.Second):
		t.Fatal("evicted pool retained the completed connection")
	}
}
