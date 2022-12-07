package tracing

import (
	"fmt"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var defaultSpanFormatter = otelhttp.WithSpanNameFormatter(
	func(_ string, request *http.Request) string {
		return fmt.Sprintf("HTTP %s %s", request.Method, request.URL.Path)
	},
)

func Transport(base http.RoundTripper, opts ...otelhttp.Option) *otelhttp.Transport {
	o := []otelhttp.Option{defaultSpanFormatter}
	o = append(o, opts...)
	return otelhttp.NewTransport(base, o...)
}
