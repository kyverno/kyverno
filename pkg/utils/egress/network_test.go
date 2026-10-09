package egress

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDialAddressesFallsBackWithinSameFamily(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	peer, expected := net.Pipe()
	defer peer.Close()
	defer expected.Close()
	var attempted []string
	var deadlines []time.Time
	connection, err := DialAddresses(ctx, func(attemptCtx context.Context, _, address string) (net.Conn, error) {
		attempted = append(attempted, address)
		deadline, ok := attemptCtx.Deadline()
		require.True(t, ok)
		deadlines = append(deadlines, deadline)
		if len(attempted) == 1 {
			// Model a blackholed IPv4 address. The attempt must expire before
			// the caller's budget so the second address can still connect.
			<-attemptCtx.Done()
			return nil, attemptCtx.Err()
		}
		return expected, nil
	}, "tcp", "443", []net.IP{net.ParseIP("192.0.2.1"), net.ParseIP("192.0.2.2")})
	require.NoError(t, err)
	assert.Same(t, expected, connection)
	assert.NoError(t, ctx.Err(), "fallback must finish inside the original aggregate deadline")
	assert.Equal(t, []string{"192.0.2.1:443", "192.0.2.2:443"}, attempted)
	require.Len(t, deadlines, 2)
	assert.True(t, deadlines[0].Before(deadlines[1]))
	outerDeadline, _ := ctx.Deadline()
	assert.Equal(t, outerDeadline, deadlines[1])
}

func TestDialAddressesCancellationStopsRemainingCandidates(t *testing.T) {
	t.Parallel()
	for _, alreadyCanceled := range []bool{true, false} {
		name := "during first attempt"
		if alreadyCanceled {
			name = "before dialing"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if alreadyCanceled {
				cancel()
			}
			attempts := 0
			connection, err := DialAddresses(ctx, func(attemptCtx context.Context, _, _ string) (net.Conn, error) {
				attempts++
				cancel()
				<-attemptCtx.Done()
				return nil, attemptCtx.Err()
			}, "tcp", "443", []net.IP{net.ParseIP("192.0.2.1"), net.ParseIP("192.0.2.2")})
			require.ErrorIs(t, err, context.Canceled)
			assert.Nil(t, connection)
			if alreadyCanceled {
				assert.Zero(t, attempts)
			} else {
				assert.Equal(t, 1, attempts)
			}
		})
	}
}

func TestDialAddressesWithoutDeadline(t *testing.T) {
	t.Parallel()
	peer, expected := net.Pipe()
	defer peer.Close()
	defer expected.Close()
	attempts := 0
	connection, err := DialAddresses(context.Background(), func(ctx context.Context, _, _ string) (net.Conn, error) {
		_, hasDeadline := ctx.Deadline()
		assert.False(t, hasDeadline, "the helper must not invent a caller timeout")
		attempts++
		if attempts == 1 {
			return nil, errors.New("connection refused")
		}
		return expected, nil
	}, "tcp", "443", []net.IP{net.ParseIP("192.0.2.1"), net.ParseIP("192.0.2.2")})
	require.NoError(t, err)
	assert.Same(t, expected, connection)
	assert.Equal(t, 2, attempts)
}

func TestPartialDialDeadline(t *testing.T) {
	t.Parallel()
	now := time.Now()
	for _, tc := range []struct {
		name      string
		budget    time.Duration
		remaining int
		want      time.Duration
	}{
		{name: "registry budget split", budget: 5 * time.Second, remaining: 2, want: 2500 * time.Millisecond},
		{name: "apiCall budget split", budget: 30 * time.Second, remaining: 4, want: 7500 * time.Millisecond},
		{name: "minimum attempt budget", budget: 5 * time.Second, remaining: 8, want: 2 * time.Second},
		{name: "short caller budget", budget: 500 * time.Millisecond, remaining: 2, want: 500 * time.Millisecond},
		{name: "last candidate", budget: 3 * time.Second, remaining: 1, want: 3 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			deadline, err := partialDialDeadline(now, now.Add(tc.budget), tc.remaining)
			require.NoError(t, err)
			assert.Equal(t, now.Add(tc.want), deadline)
		})
	}
	_, err := partialDialDeadline(now, now, 2)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	deadline, err := partialDialDeadline(now, time.Time{}, 2)
	require.NoError(t, err)
	assert.True(t, deadline.IsZero())
}
