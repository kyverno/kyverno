package egress

import (
	"bufio"
	"context"
	"errors"
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

// DNS and every connection attempt share one budget, so a registry with several A records cannot
// spend the caller's whole deadline on the first unreachable one.
// The proxy's exemption is scoped to its role, not to its address: the same endpoint reached
// directly is an ordinary destination and goes through both denylists. Otherwise configuring a
// proxy inside a denied range would quietly open that address to every NO_PROXY request too.
func TestProxyExemptionDoesNotExtendToDirectTraffic(t *testing.T) {
	t.Parallel()
	policy := testPolicy(t, Config{Mode: ModeEnforce}, map[string][]string{"public.example": {publicIPv4}, "proxy.internal": {"10.0.0.1"}})
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
	assert.Equal(t, []string{"proxy.internal:3128"}, dialed, "the proxy endpoint is exempt")
	request, err = http.NewRequestWithContext(context.Background(), http.MethodGet, "http://proxy.internal:3128/v2/", nil)
	require.NoError(t, err)
	_, err = transport.RoundTrip(request)
	require.ErrorIs(t, err, ErrAddressBlocked)
	assert.Len(t, dialed, 1, "a NO_PROXY request never inherits the proxy's exemption")

}

// A proxy endpoint is operator configuration from the process environment, not a destination any
// policy or admitted resource can choose, so neither denylist applies to it. That is what makes a
// sidecar egress proxy on loopback, or one inside RFC 1918, usable at all while those ranges are
// denied for registries -- the previous design blocked exactly that deployment. The target is
// still resolved and checked on its own: routing through a proxy must not launder it.
func TestDeniedProxyEndpointIsUsableButTargetIsStillChecked(t *testing.T) {
	t.Parallel()
	for _, proxyAddress := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254"} {
		t.Run(proxyAddress, func(t *testing.T) {
			t.Parallel()
			policy := testPolicy(t, Config{Mode: ModeEnforce}, map[string][]string{
				"proxy.internal":  {proxyAddress},
				"public.example":  {publicIPv4},
				"private.example": {"192.168.1.1"},
			})
			sentinel := errors.New("dial reached the proxy")
			var dialed []string
			transport := policy.WrapTransport(&http.Transport{
				Proxy: http.ProxyURL(&url.URL{Scheme: "http", Host: "proxy.internal:3128"}),
				DialContext: func(_ context.Context, _, address string) (net.Conn, error) {
					dialed = append(dialed, address)
					return nil, sentinel
				},
			})
			defer transport.CloseIdleConnections()
			request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://public.example/v2/", nil)
			require.NoError(t, err)
			_, err = transport.RoundTrip(request)
			require.ErrorIs(t, err, sentinel, "a proxy inside a denied range was not dialled")
			assert.Equal(t, []string{"proxy.internal:3128"}, dialed)

			request, err = http.NewRequestWithContext(context.Background(), http.MethodGet, "http://private.example/v2/", nil)
			require.NoError(t, err)
			_, err = transport.RoundTrip(request)
			require.ErrorIs(t, err, ErrAddressBlocked, "the proxy's exemption covered its target")
			assert.Len(t, dialed, 1, "a denied target reached the proxy anyway")
		})
	}
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
	policy := testPolicy(t, Config{Mode: ModeEnforce}, map[string][]string{"registry.example": {publicIPv4}})
	var requests, dials atomic.Int32
	transport := policy.WrapTransport(&http.Transport{DialContext: func(_ context.Context, _, address string) (net.Conn, error) {
		dials.Add(1)
		if address != "registry.example:80" {
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
	policy := testPolicy(t, Config{Mode: ModeEnforce}, nil)
	var resolves, dials, requests atomic.Int32
	policy.lookupNetIP = func(context.Context, string, string) ([]netip.Addr, error) {
		if resolves.Add(1) > 2 {
			return []netip.Addr{netip.MustParseAddr("169.254.169.254")}, nil
		}
		return []netip.Addr{netip.MustParseAddr(publicIPv4)}, nil
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

type trackingBody struct {
	io.Reader
	closed bool
}

func (b *trackingBody) Close() error { b.closed = true; return nil }

func TestBlockedRequestClosesBody(t *testing.T) {
	t.Parallel()
	policy := testPolicy(t, Config{Mode: ModeEnforce}, nil)
	transport := policy.WrapTransport(&http.Transport{})
	defer transport.CloseIdleConnections()
	body := &trackingBody{Reader: strings.NewReader("credentials")}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://169.254.169.254/latest/", body)
	require.NoError(t, err)
	_, err = transport.RoundTrip(request)
	require.ErrorIs(t, err, ErrAddressBlocked)
	assert.True(t, body.closed)
}

// An authority mismatch is a malformed request rather than a denied destination, so it is
// rejected in audit mode too -- these fixtures stay on the default mode to pin that. WriteProxy
// uses Host for the absolute request target and URL.Opaque can override the target entirely, so
// neither may select a destination the policy never saw.
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
			policy := testPolicy(t, Config{}, map[string][]string{"public.example": {publicIPv4}, "proxy.internal": {"10.0.0.1"}})
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
			policy := testPolicy(t, Config{Mode: ModeEnforce}, map[string][]string{"public.example": {publicIPv4}, "proxy.internal": {"10.0.0.1"}})
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
			assert.Equal(t, "proxy.internal:3128", dialed)
		})
	}
}

// A proxied request only ever dials the proxy, so the pool must follow the proxy's addresses
// alone. Keying it by the target too would retire a healthy proxy connection every time any
// registry's DNS answer moved, and a fleet of registries behind one proxy would churn the
// bounded LRU out of useful entries.
// A proxy owns downstream DNS, so a registry that only it can resolve must still be reachable.
// Failing here would break those deployments in every mode, including audit.
func TestTransportDefersUnresolvableTargetToProxy(t *testing.T) {
	t.Parallel()
	policy := testPolicy(t, Config{Mode: ModeEnforce}, map[string][]string{"proxy.example": {publicIPv4}})
	reachedDial := errors.New("dial sentinel")
	var dialed []string
	base := &http.Transport{
		Proxy: http.ProxyURL(&url.URL{Scheme: "http", Host: "proxy.example"}),
		DialContext: func(_ context.Context, _, address string) (net.Conn, error) {
			dialed = append(dialed, address)
			return nil, reachedDial
		},
	}
	transport := policy.WrapTransport(base)

	// "registry.internal" is absent from the resolver answers, so the local lookup reports
	// not-found. The request must still be attempted, against the proxy.
	request, err := http.NewRequest(http.MethodGet, "https://registry.internal/v2/", nil)
	require.NoError(t, err)
	_, err = transport.RoundTrip(request)
	// hostError keeps resolved addresses out of the message but still unwraps to the cause.
	require.ErrorIs(t, err, reachedDial)
	require.NotEmpty(t, dialed, "the request never reached the proxy")
	assert.Equal(t, "proxy.example:80", dialed[0], "dialled something other than the proxy")
}

// Deferring to the proxy must not become a way around the denylist: a literal address never
// reaches the resolver, so it cannot be excused as not-found.
func TestTransportBlocksLiteralAddressBehindProxy(t *testing.T) {
	t.Parallel()
	policy := testPolicy(t, Config{Mode: ModeEnforce}, map[string][]string{"proxy.example": {publicIPv4}})
	base := &http.Transport{
		Proxy: http.ProxyURL(&url.URL{Scheme: "http", Host: "proxy.example"}),
		DialContext: func(_ context.Context, _, address string) (net.Conn, error) {
			t.Errorf("dialled %q for a blocked destination", address)
			return nil, errors.New("unreachable")
		},
	}
	transport := policy.WrapTransport(base)

	request, err := http.NewRequest(http.MethodGet, "https://169.254.169.254/v2/", nil)
	require.NoError(t, err)
	_, err = transport.RoundTrip(request)
	require.ErrorIs(t, err, ErrAddressBlocked)
}

// A resolver failure that is not a plain not-found still fails closed, with or without a proxy.
func TestTransportFailsClosedOnResolverError(t *testing.T) {
	t.Parallel()
	policy := testPolicy(t, Config{}, map[string][]string{"proxy.example": {publicIPv4}})
	policy.lookupNetIP = func(_ context.Context, _, host string) ([]netip.Addr, error) {
		if host == "proxy.example" {
			return []netip.Addr{netip.MustParseAddr(publicIPv4)}, nil
		}
		return nil, &net.DNSError{Err: "server misbehaving", Name: host, IsTemporary: true}
	}
	base := &http.Transport{
		Proxy: http.ProxyURL(&url.URL{Scheme: "http", Host: "proxy.example"}),
		DialContext: func(_ context.Context, _, address string) (net.Conn, error) {
			t.Errorf("dialled %q despite an unresolved destination", address)
			return nil, errors.New("unreachable")
		},
	}
	request, err := http.NewRequest(http.MethodGet, "https://registry.internal/v2/", nil)
	require.NoError(t, err)
	_, err = policy.WrapTransport(base).RoundTrip(request)
	require.ErrorContains(t, err, "failed to resolve")
}

// remoteConn reports a chosen peer address, standing in for a TCP connection the inner transport
// dialled after resolving the name itself.
type remoteConn struct {
	net.Conn
	remote net.Addr
	closed atomic.Bool
}

func (c *remoteConn) RemoteAddr() net.Addr { return c.remote }
func (c *remoteConn) Close() error         { c.closed.Store(true); return c.Conn.Close() }

// serveOK answers every request on server with a 200 and counts them.
func serveOK(server net.Conn, requests *atomic.Int32) {
	defer server.Close()
	reader := bufio.NewReader(server)
	for {
		request, err := http.ReadRequest(reader)
		if err != nil {
			return
		}
		request.Body.Close()
		requests.Add(1)
		if _, err = io.WriteString(server, "HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n"); err != nil {
			return
		}
	}
}

// The inner transport resolves the name again when it dials. If that answer differs from the one
// RoundTrip checked -- DNS rebinding -- the connection it actually reached is checked too.
func TestTransportChecksTheAddressItDialled(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		mode    string
		blocked bool
	}{{mode: ModeEnforce, blocked: true}, {mode: ModeAudit}} {
		t.Run(tc.mode, func(t *testing.T) {
			t.Parallel()
			policy := testPolicy(t, Config{Mode: tc.mode}, map[string][]string{"registry.example": {publicIPv4}})
			var requests atomic.Int32
			var conn *remoteConn
			transport := policy.WrapTransport(&http.Transport{DialContext: func(context.Context, string, string) (net.Conn, error) {
				client, server := net.Pipe()
				go serveOK(server, &requests)
				conn = &remoteConn{Conn: client, remote: &net.TCPAddr{IP: net.ParseIP("169.254.169.254"), Port: 80}}
				return conn, nil
			}})
			defer transport.CloseIdleConnections()
			request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://registry.example/v2/", nil)
			require.NoError(t, err)
			response, err := transport.RoundTrip(request)
			if !tc.blocked {
				require.NoError(t, err)
				require.NoError(t, response.Body.Close())
				assert.Equal(t, int32(1), requests.Load(), "audit logs the address and connects anyway")
				return
			}
			require.ErrorIs(t, err, ErrAddressBlocked)
			assert.NotContains(t, err.Error(), "169.254", "the resolved address stays out of the error")
			assert.True(t, conn.closed.Load(), "the connection to the denied address was left open")
			assert.Zero(t, requests.Load(), "a request was written to the denied address")
		})
	}
}

// A proxy endpoint is exempt from both lists, so the connection to it is not re-checked either:
// a sidecar proxy on loopback has to keep working.
func TestTransportDoesNotCheckTheProxyConnection(t *testing.T) {
	t.Parallel()
	policy := testPolicy(t, Config{Mode: ModeEnforce}, map[string][]string{"registry.example": {publicIPv4}})
	var requests atomic.Int32
	transport := policy.WrapTransport(&http.Transport{
		Proxy: http.ProxyURL(&url.URL{Scheme: "http", Host: "proxy.internal:3128"}),
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			client, server := net.Pipe()
			go serveOK(server, &requests)
			return &remoteConn{Conn: client, remote: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 3128}}, nil
		},
	})
	defer transport.CloseIdleConnections()
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://registry.example/v2/", nil)
	require.NoError(t, err)
	response, err := transport.RoundTrip(request)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	assert.Equal(t, int32(1), requests.Load())
}
