package apicall

import (
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"k8s.io/klog/v2"
)

// scopedTokenPath is the mount path of the projected ServiceAccount token used
// for outbound APICall and CEL http requests. Unlike the default SA token, this
// token carries a custom audience (configured via .Values.apiCallToken.audience)
// so that if it is leaked to an external service it cannot be replayed against
// the Kubernetes API server.
const (
	scopedTokenPathEnvVar           = "KYVERNO_SCOPED_TOKEN_PATH"              // #nosec G101 false positive: environment variable name
	defaultScopedTokenPath          = "/var/run/secrets/kyverno/apicall/token" // #nosec G101 false positive: token file path, not a credential
	defaultScopedTokenClientTimeout = 30 * time.Second
)

var (
	scopedTokenPath            = getScopedTokenPath()
	scopedTokenClientTimeout   = defaultScopedTokenClientTimeout
	scopedTokenReadWarningOnce sync.Once
)

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

func readScopedToken() (string, bool) {
	b, err := os.ReadFile(scopedTokenPath)
	if err != nil {
		scopedTokenReadWarningOnce.Do(func() {
			if os.IsNotExist(err) {
				klog.Warningf("optional scoped APICall token not found at %s; outbound calls will proceed without Authorization header unless explicitly provided", scopedTokenPath)
			} else {
				klog.Warningf("failed to read optional scoped APICall token at %s: %v", scopedTokenPath, err)
			}
		})
		return "", false
	}
	return strings.TrimSpace(string(b)), true
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
		token, ok := readScopedToken()
		if ok && token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
	return c.inner.Do(req) //nolint:gosec // SSRF is mitigated by the blocklist/allowlist applied in NewCELHTTPContext
}
