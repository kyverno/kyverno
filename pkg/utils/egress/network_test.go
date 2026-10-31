package egress

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeConn struct{ net.Conn }

// Close keeps the drain path safe: when a losing family connects after the race is already
// won, dialRace closes that connection, and the embedded net.Conn here is nil.
func (fakeConn) Close() error { return nil }

// dialScript answers per address family so a test can make one family fail or stall.
func dialScript(t *testing.T, answer func(ip net.IP) (net.Conn, error)) DialContextFunc {
	t.Helper()
	return func(ctx context.Context, _, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		require.NoError(t, err)
		ip := net.ParseIP(host)
		require.NotNil(t, ip, "dialled a non-IP address %q", host)
		return answer(ip)
	}
}

func isIPv4(ip net.IP) bool { return ip.To4() != nil }

// A dead primary family must release the fallback immediately rather than making every
// connection wait out fallbackDelay. On a dual-stack cluster with unroutable records that
// delay lands on each admission-path request.
func TestDialRaceFallbackStartsWhenPrimaryFails(t *testing.T) {
	t.Parallel()
	dial := dialScript(t, func(ip net.IP) (net.Conn, error) {
		if isIPv4(ip) {
			return nil, errors.New("network unreachable")
		}
		return fakeConn{}, nil
	})

	start := time.Now()
	conn, err := DialAddresses(context.Background(), dial, "tcp", "443",
		[]net.IP{net.ParseIP("10.0.0.1"), net.ParseIP("fd00::1")})
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, conn)
	require.Less(t, elapsed, fallbackDelay/2,
		"fallback waited for the timer instead of the primary's failure")
}

// The head start still has to exist: a primary that succeeds within the delay must win, so
// the preferred family is not abandoned just because the fallback is reachable.
func TestDialRacePrimaryKeepsHeadStart(t *testing.T) {
	t.Parallel()
	var fallbackDialed atomic.Bool
	dial := dialScript(t, func(ip net.IP) (net.Conn, error) {
		if isIPv4(ip) {
			return fakeConn{}, nil
		}
		fallbackDialed.Store(true)
		return fakeConn{}, nil
	})

	conn, err := DialAddresses(context.Background(), dial, "tcp", "443",
		[]net.IP{net.ParseIP("10.0.0.1"), net.ParseIP("fd00::1")})
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.False(t, fallbackDialed.Load(), "fallback ran despite the primary succeeding first")
}

// Both families failing must surface the primary's error, since that describes the address
// the caller would have reached without this guard.
func TestDialRacePrefersPrimaryError(t *testing.T) {
	t.Parallel()
	primaryErr := errors.New("primary refused")
	dial := dialScript(t, func(ip net.IP) (net.Conn, error) {
		if isIPv4(ip) {
			return nil, primaryErr
		}
		return nil, errors.New("fallback refused")
	})

	_, err := DialAddresses(context.Background(), dial, "tcp", "443",
		[]net.IP{net.ParseIP("10.0.0.1"), net.ParseIP("fd00::1")})
	require.ErrorIs(t, err, primaryErr)
}

// A cancelled caller must not be kept waiting for the fallback timer.
func TestDialRaceHonoursCancellation(t *testing.T) {
	t.Parallel()
	var dialed atomic.Int32
	dial := dialScript(t, func(net.IP) (net.Conn, error) {
		dialed.Add(1)
		return nil, errors.New("unreachable")
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, err := DialAddresses(ctx, dial, "tcp", "443",
		[]net.IP{net.ParseIP("10.0.0.1"), net.ParseIP("fd00::1")})
	// Assert the cancellation specifically: any error would otherwise satisfy this test even
	// if the context were ignored and the dial simply failed on its own.
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, dialed.Load(), "dialled despite the context already being cancelled")
	require.Less(t, time.Since(start), fallbackDelay, "cancellation waited for the fallback timer")
}

func TestDialAddressesRejectsEmpty(t *testing.T) {
	t.Parallel()
	_, err := DialAddresses(context.Background(), nil, "tcp", "443", nil)
	require.Error(t, err)
}
