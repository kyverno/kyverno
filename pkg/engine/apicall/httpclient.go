package apicall

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// scopedTokenPath is the mount path of the projected ServiceAccount token used
// for outbound APICall and CEL http requests. Unlike the default SA token, this
// token carries a custom audience (configured via .Values.apiCallToken.audience)
// so that if it is leaked to an external service it cannot be replayed against
// the Kubernetes API server.
const (
	scopedTokenPathEnvVar           = "KYVERNO_SCOPED_TOKEN_PATH"
	defaultScopedTokenPath          = "/var/run/secrets/kyverno/apicall/token"
	defaultScopedTokenClientTimeout = 30 * time.Second
)

var scopedTokenPath = getScopedTokenPath()
var scopedTokenClientTimeout = defaultScopedTokenClientTimeout

func getScopedTokenPath() string {
	if path := os.Getenv(scopedTokenPathEnvVar); path != "" {
		return path
	}
	return defaultScopedTokenPath
}

// SetScopedTokenClientTimeout configures timeout for outbound CEL HTTP calls
// performed through the scoped token client. A value of 0 disables timeout.
func SetScopedTokenClientTimeout(timeout time.Duration) {
	scopedTokenClientTimeout = timeout
}

// scopedTokenClient wraps http.Client and injects the scoped APICall token as
// an Authorization Bearer header whenever the caller has not already set one.
type scopedTokenClient struct {
	inner *http.Client
}

// NewScopedTokenClient returns a *scopedTokenClient that injects the scoped
// APICall token into outbound HTTP requests. This concrete type satisfies the
// ClientInterface expected by github.com/kyverno/sdk/cel/libs/http.NewHTTP.
func NewScopedTokenClient() *scopedTokenClient {
	return &scopedTokenClient{
		inner: &http.Client{
			Transport: tracing.Transport(http.DefaultTransport, otelhttp.WithFilter(tracing.RequestFilterIsInSpan)),
			Timeout:   scopedTokenClientTimeout,
		},
	}
}

func (c *scopedTokenClient) Do(req *http.Request) (*http.Response, error) {
	if req.Header.Get("Authorization") == "" {
		b, err := os.ReadFile(scopedTokenPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read required scoped APICall token from %s: %w", scopedTokenPath, err)
		}
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(b)))
	}
	return c.inner.Do(req)
}
