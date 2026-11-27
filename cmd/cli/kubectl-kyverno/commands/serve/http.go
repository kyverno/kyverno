package serve

import (
	"net/http"
	"time"

	"github.com/kyverno/kyverno/pkg/cel/compiler"
)

// DefaultHTTPTimeout bounds each HTTP call a policy makes during an evaluation.
const DefaultHTTPTimeout = 30 * time.Second

// boundPolicyHTTP bounds the HTTP calls policies make (CEL http.Get and http.Post).
//
// The CEL HTTP library sends its requests without a deadline, and its guarded clients (the ones
// kyverno uses, since its default blocklist is never empty) have no overall timeout. Without a
// bound, an endpoint that accepts the connection and never answers, or sends headers and then
// stalls the body, would hold the server lock forever and stall every later request.
//
// compiler.SetHTTPCallTimeout bounds each whole call, body included. The client and transport
// settings below also close the connection on a header stall, so most stalled calls don't linger
// in the background. They must be set before the first evaluation, because the library clones the
// default transport lazily.
func boundPolicyHTTP(timeout time.Duration) {
	compiler.SetHTTPCallTimeout(timeout)
	http.DefaultClient.Timeout = timeout
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		t.ResponseHeaderTimeout = timeout
	}
}
