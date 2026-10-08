package egress

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransportPinsExemptHostsAndSharesDeadline(t *testing.T) {
	t.Parallel()
	policy := testPolicy(t, Config{Mode: ModeEnforce, Allowlist: []string{"registry.internal"}}, nil)
	var resolutionDeadline time.Time
	policy.lookupNetIP = func(ctx context.Context, _, _ string) ([]netip.Addr, error) {
		resolutionDeadline, _ = ctx.Deadline()
		return []netip.Addr{netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.2")}, nil
	}
	var addresses []string
	var deadlines []time.Time
	transport := policy.WrapTransport(&http.Transport{DialContext: func(ctx context.Context, _, address string) (net.Conn, error) {
		addresses = append(addresses, address)
		deadline, _ := ctx.Deadline()
		deadlines = append(deadlines, deadline)
		return nil, fmt.Errorf("cannot connect to %s", address)
	}})
	defer transport.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://registry.internal/v2/", nil)
	require.NoError(t, err)
	_, err = transport.RoundTrip(request)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "10.0.0.")
	assert.Equal(t, []string{"10.0.0.1:80", "10.0.0.2:80"}, addresses)
	require.Len(t, deadlines, 2)
	callerDeadline, _ := ctx.Deadline()
	assert.Equal(t, callerDeadline, resolutionDeadline, "DNS uses the unchanged aggregate deadline")
	assert.True(t, deadlines[0].Before(resolutionDeadline), "the first address must leave time for fallback")
	assert.Equal(t, resolutionDeadline, deadlines[1], "the last address cannot extend the aggregate deadline")
}

func TestPrivateProxyDoesNotExemptDirectTraffic(t *testing.T) {
	t.Parallel()
	policy := testPolicy(t, Config{Mode: ModeEnforce}, map[string][]string{"public.example": {"93.184.216.34"}, "proxy.internal": {"10.0.0.1"}})
	proxy := &url.URL{Scheme: "http", Host: "proxy.internal:3128"}
	var dialed []string
	transport := policy.WrapTransport(&http.Transport{
		Proxy: func(request *http.Request) (*url.URL, error) {
			if request.URL.Hostname() == "public.example" {
				return proxy, nil
			}
			return nil, nil
		},
		DialContext: func(_ context.Context, _, address string) (net.Conn, error) {
			dialed = append(dialed, address)
			return nil, errors.New("dial sentinel")
		},
	})
	defer transport.CloseIdleConnections()
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://public.example/v2/", nil)
	require.NoError(t, err)
	_, err = transport.RoundTrip(request)
	require.Error(t, err)
	assert.Equal(t, []string{"10.0.0.1:3128"}, dialed, "operator proxy gets its own validated, pinned connection")
	request, err = http.NewRequestWithContext(context.Background(), http.MethodGet, "http://proxy.internal:3128/v2/", nil)
	require.NoError(t, err)
	_, err = transport.RoundTrip(request)
	require.ErrorIs(t, err, ErrAddressBlocked)
	assert.Len(t, dialed, 1, "NO_PROXY never inherits the proxy's private-network exemption")

	// A later direct request to the same endpoint must use its new validated
	// address, not any private address previously cached for the proxy.
	policy.lookupNetIP = func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("93.184.216.35")}, nil
	}
	_, err = transport.RoundTrip(request)
	require.Error(t, err)
	assert.Equal(t, []string{"10.0.0.1:3128", "93.184.216.35:3128"}, dialed)
}

func TestProxyCannotForwardBlockedTarget(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"169.254.169.254", "internal.example"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			policy := testPolicy(t, Config{Mode: ModeEnforce}, map[string][]string{"internal.example": {"10.0.0.2"}})
			var dialed atomic.Int32
			transport := policy.WrapTransport(&http.Transport{
				Proxy: http.ProxyURL(&url.URL{Scheme: "http", Host: "proxy.internal:3128"}),
				DialContext: func(context.Context, string, string) (net.Conn, error) {
					dialed.Add(1)
					return nil, errors.New("unexpected dial")
				},
			})
			defer transport.CloseIdleConnections()
			request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+target+"/", nil)
			require.NoError(t, err)
			_, err = transport.RoundTrip(request)
			require.ErrorIs(t, err, ErrAddressBlocked)
			assert.Zero(t, dialed.Load())
		})
	}
}

func TestRedirectBlockedBeforeCredentialsReachDestination(t *testing.T) {
	t.Parallel()
	policy := testPolicy(t, Config{}, map[string][]string{"registry.example": {"93.184.216.34"}})
	var requests, dials atomic.Int32
	transport := policy.WrapTransport(&http.Transport{DialContext: func(_ context.Context, _, address string) (net.Conn, error) {
		dials.Add(1)
		if address != "93.184.216.34:80" {
			return nil, errors.New("unexpected destination")
		}
		client, server := net.Pipe()
		go func() {
			defer server.Close()
			request, err := http.ReadRequest(bufio.NewReader(server))
			if err != nil {
				return
			}
			requests.Add(1)
			request.Body.Close()
			_, _ = io.WriteString(server, "HTTP/1.1 302 Found\r\nLocation: http://169.254.169.254/latest/meta-data/\r\nContent-Length: 0\r\nConnection: close\r\n\r\n")
		}()
		return client, nil
	}})
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://registry.example/v2/", nil)
	require.NoError(t, err)
	request.Header.Set("Authorization", "Bearer registry-secret")
	_, err = client.Do(request)
	require.ErrorIs(t, err, ErrAddressBlocked)
	assert.Equal(t, int32(1), requests.Load())
	assert.Equal(t, int32(1), dials.Load(), "the redirect must not send a request or credentials to metadata")
}

func TestReusedConnectionStillValidatesEveryRequest(t *testing.T) {
	t.Parallel()
	policy := testPolicy(t, Config{}, nil)
	var resolves, dials, requests atomic.Int32
	policy.lookupNetIP = func(context.Context, string, string) ([]netip.Addr, error) {
		if resolves.Add(1) > 2 {
			return []netip.Addr{netip.MustParseAddr("169.254.169.254")}, nil
		}
		return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
	}
	transport := policy.WrapTransport(&http.Transport{DialContext: func(context.Context, string, string) (net.Conn, error) {
		dials.Add(1)
		client, server := net.Pipe()
		go func() {
			defer server.Close()
			reader := bufio.NewReader(server)
			for {
				request, err := http.ReadRequest(reader)
				if err != nil {
					return
				}
				request.Body.Close()
				requests.Add(1)
				if _, err = io.WriteString(server, "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok"); err != nil {
					return
				}
			}
		}()
		return client, nil
	}})
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	for range 2 {
		request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://registry.example/v2/", nil)
		require.NoError(t, err)
		response, err := client.Do(request)
		require.NoError(t, err)
		_, err = io.Copy(io.Discard, response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://registry.example/v2/", nil)
	require.NoError(t, err)
	_, err = client.Do(request)
	require.ErrorIs(t, err, ErrAddressBlocked)
	assert.Equal(t, int32(1), dials.Load(), "validated requests share the connection pool")
	assert.Equal(t, int32(2), requests.Load(), "the blocked request cannot reuse an existing connection")
}

func TestDialAddressesRacesIPv4AndIPv6(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	primaryStarted := make(chan struct{})
	primaryCanceled := make(chan struct{})
	peer, expected := net.Pipe()
	defer peer.Close()
	defer expected.Close()
	connection, err := DialAddresses(ctx, func(ctx context.Context, _, address string) (net.Conn, error) {
		if strings.HasPrefix(address, "[") {
			close(primaryStarted)
			<-ctx.Done()
			close(primaryCanceled)
			return nil, ctx.Err()
		}
		<-primaryStarted
		return expected, nil
	}, "tcp", "443", []net.IP{net.ParseIP("2001:db8::1"), net.ParseIP("93.184.216.34")})
	require.NoError(t, err)
	assert.Same(t, expected, connection)
	select {
	case <-primaryCanceled:
	case <-ctx.Done():
		t.Fatal("losing address family was not canceled")
	}
}

type trackingBody struct {
	io.Reader
	closed bool
}

func (b *trackingBody) Close() error { b.closed = true; return nil }

func TestBlockedRequestClosesBody(t *testing.T) {
	t.Parallel()
	policy := testPolicy(t, Config{}, nil)
	transport := policy.WrapTransport(&http.Transport{})
	defer transport.CloseIdleConnections()
	body := &trackingBody{Reader: strings.NewReader("credentials")}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://169.254.169.254/latest/", body)
	require.NoError(t, err)
	_, err = transport.RoundTrip(request)
	require.ErrorIs(t, err, ErrAddressBlocked)
	assert.True(t, body.closed)
}

func TestTransportRejectsAlternateRequestAuthority(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		host   string
		opaque string
	}{
		{name: "metadata Host", host: "169.254.169.254"},
		{name: "different public Host", host: "other.example"},
		{name: "different port", host: "public.example:8080"},
		{name: "opaque request target", opaque: "//169.254.169.254/latest/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			policy := testPolicy(t, Config{}, map[string][]string{"public.example": {"93.184.216.34"}, "proxy.internal": {"10.0.0.1"}})
			var dials atomic.Int32
			transport := policy.WrapTransport(&http.Transport{
				Proxy: http.ProxyURL(&url.URL{Scheme: "http", Host: "proxy.internal:3128"}),
				DialContext: func(context.Context, string, string) (net.Conn, error) {
					dials.Add(1)
					return nil, errors.New("unexpected dial")
				},
			})
			defer transport.CloseIdleConnections()
			request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://public.example/v2/", nil)
			require.NoError(t, err)
			if tc.host != "" {
				request.Host = tc.host
			}
			request.URL.Opaque = tc.opaque
			_, err = transport.RoundTrip(request)
			require.ErrorIs(t, err, ErrAddressBlocked)
			assert.Zero(t, dials.Load())
		})
	}
}

func TestTransportAcceptsCanonicalRequestAuthority(t *testing.T) {
	t.Parallel()
	for _, host := range []string{"", "public.example", "PUBLIC.EXAMPLE.", "public.example:80", "public.example:080"} {
		t.Run(host, func(t *testing.T) {
			t.Parallel()
			policy := testPolicy(t, Config{}, map[string][]string{"public.example": {"93.184.216.34"}, "proxy.internal": {"10.0.0.1"}})
			sentinel := errors.New("dial reached validated proxy")
			var dialed string
			transport := policy.WrapTransport(&http.Transport{
				Proxy: http.ProxyURL(&url.URL{Scheme: "http", Host: "proxy.internal:3128"}),
				DialContext: func(_ context.Context, _, address string) (net.Conn, error) {
					dialed = address
					return nil, sentinel
				},
			})
			defer transport.CloseIdleConnections()
			request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://public.example/v2/", nil)
			require.NoError(t, err)
			request.Host = host
			_, err = transport.RoundTrip(request)
			require.ErrorIs(t, err, sentinel)
			assert.Equal(t, "10.0.0.1:3128", dialed)
		})
	}
}
