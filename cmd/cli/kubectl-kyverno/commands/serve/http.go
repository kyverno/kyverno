package serve

import (
	"net/http"
	"time"
)

// DefaultHTTPTimeout bounds each HTTP call a policy makes during an evaluation.
const DefaultHTTPTimeout = 30 * time.Second

// boundPolicyHTTP bounds the HTTP calls policies make (CEL http.Get and http.Post).
//
// The CEL HTTP library sends its requests without a deadline, through http.DefaultClient or a
// client built on a clone of http.DefaultTransport. Without a bound, an endpoint that accepts the
// connection and never answers would hold the server lock forever and stall every later request.
// kyverno serve owns its process, so it sets the bounds there, before any policy runs: the client
// timeout covers the whole call on the default client, and the response header timeout covers
// guarded clients, whose transports are cloned from the default one.
func boundPolicyHTTP(timeout time.Duration) {
	http.DefaultClient.Timeout = timeout
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		t.ResponseHeaderTimeout = timeout
	}
}
