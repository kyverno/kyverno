package egress

import (
	"context"
	"errors"
	"net"
	"time"
)

// DialContextFunc makes one connection attempt to an already validated IP.
type DialContextFunc func(context.Context, string, string) (net.Conn, error)

const fallbackDelay = 300 * time.Millisecond

// DialAddresses connects only to the supplied addresses and races IPv4 and IPv6
// using the same fallback delay as net.Dialer. The caller supplies one context
// deadline spanning resolution and all attempts. Remaining time is divided among
// candidates in each family so an unreachable address cannot consume the entire budget.
func DialAddresses(ctx context.Context, dial DialContextFunc, network, port string, addresses []net.IP) (net.Conn, error) {
	if len(addresses) == 0 {
		return nil, errors.New("no addresses to dial")
	}
	preferIPv4 := addresses[0].To4() != nil
	var primary, fallback []net.IP
	for _, ip := range addresses {
		if network == "tcp4" && ip.To4() == nil || network == "tcp6" && ip.To4() != nil {
			continue
		}
		if (ip.To4() != nil) == preferIPv4 {
			primary = append(primary, ip)
		} else {
			fallback = append(fallback, ip)
		}
	}
	if len(primary) == 0 {
		return dialSeries(ctx, dial, network, port, fallback)
	}
	if len(fallback) == 0 {
		return dialSeries(ctx, dial, network, port, primary)
	}
	return dialRace(ctx, dial, network, port, primary, fallback)
}

// dialSeries tries each address in turn and returns the first connection that succeeds.
func dialSeries(ctx context.Context, dial DialContextFunc, network, port string, ips []net.IP) (net.Conn, error) {
	var lastErr error
	for i, ip := range ips {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		attemptCtx := ctx
		cancel := func() {}
		if deadline, ok := ctx.Deadline(); ok {
			attemptDeadline, err := partialDialDeadline(time.Now(), deadline, len(ips)-i)
			if err != nil {
				return nil, err
			}
			if attemptDeadline.Before(deadline) {
				attemptCtx, cancel = context.WithDeadline(ctx, attemptDeadline)
			}
		}
		conn, err := dial(attemptCtx, network, net.JoinHostPort(ip.String(), port))
		cancel()
		if err == nil {
			return conn, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	if lastErr == nil {
		lastErr = errors.New("no addresses to dial")
	}
	return nil, lastErr
}

// partialDialDeadline follows net.Dialer's allocation of the remaining budget:
// split it across pending addresses, with at least two seconds per attempt when
// available. A shorter caller budget always wins, and the final address gets
// whatever remains of the original deadline.
func partialDialDeadline(now, deadline time.Time, remaining int) (time.Time, error) {
	if deadline.IsZero() {
		return deadline, nil
	}
	budget := deadline.Sub(now)
	if budget <= 0 {
		return time.Time{}, context.DeadlineExceeded
	}
	share := budget / time.Duration(remaining)
	if share < 2*time.Second {
		share = min(budget, 2*time.Second)
	}
	return now.Add(share), nil
}

type dialOutcome struct {
	conn      net.Conn
	err       error
	isPrimary bool
}

// dialRace runs the two address families concurrently, giving the preferred one a
// fallbackDelay head start, and returns the first connection to succeed (RFC 6555).
func dialRace(ctx context.Context, dial DialContextFunc, network, port string, primary, fallback []net.IP) (net.Conn, error) {
	ctx, cancel := context.WithCancel(ctx)
	// Cancelling on return tears down the losing attempt. A connection already returned is
	// unaffected: net.Dialer does not tie a live conn to the dial context.
	defer cancel()

	// Buffered, so a loser that finishes after we have returned never blocks on send.
	outcomes := make(chan dialOutcome, 2)
	race := func(ips []net.IP, isPrimary bool, delay time.Duration) {
		go func() {
			if delay > 0 {
				timer := time.NewTimer(delay)
				defer timer.Stop()
				select {
				case <-timer.C:
				case <-ctx.Done():
					outcomes <- dialOutcome{err: ctx.Err(), isPrimary: isPrimary}
					return
				}
			}
			conn, err := dialSeries(ctx, dial, network, port, ips)
			outcomes <- dialOutcome{conn: conn, err: err, isPrimary: isPrimary}
		}()
	}
	race(primary, true, 0)
	race(fallback, false, fallbackDelay)

	var firstErr error
	for i := 0; i < 2; i++ {
		out := <-outcomes
		if out.err == nil {
			// Drain the sibling so a connection that lands after this point is closed
			// rather than leaked.
			if remaining := 1 - i; remaining > 0 {
				go func(n int) {
					for j := 0; j < n; j++ {
						if late := <-outcomes; late.conn != nil {
							late.conn.Close()
						}
					}
				}(remaining)
			}
			return out.conn, nil
		}
		// Prefer the primary family's error: it describes the address the caller would
		// have reached without this guard.
		if firstErr == nil || out.isPrimary {
			firstErr = out.err
		}
	}
	return nil, firstErr
}

// EmbeddedIPv4 extracts the IPv4 address carried by an IPv6 address in a form that
// net.IP.To4 does not recognise: IPv4-compatible (::a.b.c.d), the NAT64 well-known prefix
// (64:ff9b::a.b.c.d), and 6to4 (2002:aabb:ccdd::). It returns nil when the address carries
// no embedded IPv4 address, or when To4 already handles it.
//
// Each of these is a distinct spelling of an address that an IPv4 CIDR on the blocklist is
// meant to cover. Without normalization, 64:ff9b::a9fe:a9fe reaches the metadata service
// even though 169.254.169.254/32 is blocked. NAT64 is the form that matters in practice,
// because it is standard in IPv6-only clusters; the other two are deprecated and need
// specific routing, so they are defense in depth.
//
// The IPv4-compatible branch also matches low IPv6 addresses that were never meant as an
// IPv4 encoding: ::1 yields 0.0.0.1, for example. That only ever widens the blocklist into
// reserved space, so it is left as is.
func EmbeddedIPv4(ip net.IP) net.IP {
	if ip.To4() != nil || len(ip.To16()) != net.IPv6len {
		return nil
	}
	ip = ip.To16()
	isZero := func(b []byte) bool {
		for _, c := range b {
			if c != 0 {
				return false
			}
		}
		return true
	}
	// ::a.b.c.d — IPv4-compatible IPv6 (RFC 4291).
	if isZero(ip[:12]) {
		return net.IPv4(ip[12], ip[13], ip[14], ip[15])
	}
	// 64:ff9b::a.b.c.d — NAT64 well-known prefix (RFC 6052).
	if ip[0] == 0x00 && ip[1] == 0x64 && ip[2] == 0xff && ip[3] == 0x9b && isZero(ip[4:12]) {
		return net.IPv4(ip[12], ip[13], ip[14], ip[15])
	}
	// 2002:aabb:ccdd::/48 — 6to4 (RFC 3056), which carries the IPv4 address in bytes 2-5
	// rather than at the end. 2002:a9fe:a9fe:: is 169.254.169.254.
	if ip[0] == 0x20 && ip[1] == 0x02 {
		return net.IPv4(ip[2], ip[3], ip[4], ip[5])
	}
	return nil
}
