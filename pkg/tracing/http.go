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

func RequestFilterIsInSpan(request *http.Request) bool {
	return IsInSpan(request.Context())
}

func Transport(base http.RoundTripper, opts ...otelhttp.Option) *otelhttp.Transport {
	o := []otelhttp.Option{defaultSpanFormatter}
	o = append(o, opts...)
	return otelhttp.NewTransport(base, o...)
}
