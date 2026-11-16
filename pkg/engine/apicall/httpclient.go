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

// ScopedTokenClientTimeout returns the timeout configured for outbound CEL HTTP
// calls performed with the scoped token. A value of 0 disables the timeout.
func ScopedTokenClientTimeout() time.Duration {
	return scopedTokenClientTimeout
}

func readScopedToken() (string, bool) {
	return readTokenAt(scopedTokenPath)
}

// ScopedToken returns the scoped APICall token (the projected ServiceAccount
// token carrying the custom audience) read from the configured token file. The
// second result is false when no non-empty token is available, in which case
// callers proceed without an Authorization header — exactly as the scoped token
// client does. Unlike readScopedToken, the path is resolved at call time so the
// KYVERNO_SCOPED_TOKEN_PATH override is honoured per call.
func ScopedToken() (string, bool) {
	return readTokenAt(getScopedTokenPath())
}

func readTokenAt(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		scopedTokenReadWarningOnce.Do(func() {
			if os.IsNotExist(err) {
				klog.Warningf("optional scoped APICall token not found at %s; outbound calls will proceed without Authorization header unless explicitly provided", path)
			} else {
				klog.Warningf("failed to read optional scoped APICall token at %s: %v", path, err)
			}
		})
		return "", false
	}
	token := strings.TrimSpace(string(b))
	if token == "" {
		return "", false
	}
	return token, true
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
	return c.inner.Do(req)
}
