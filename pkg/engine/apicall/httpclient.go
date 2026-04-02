package apicall

import (
	"net/http"
	"os"
	"strings"
)

// scopedTokenPath is the mount path of the projected ServiceAccount token used
// for outbound APICall and CEL http requests. Unlike the default SA token, this
// token carries a custom audience (configured via .Values.apiCallToken.audience)
// so that if it is leaked to an external service it cannot be replayed against
// the Kubernetes API server.
const scopedTokenPath = "/var/run/secrets/kyverno/apicall/token"

// scopedTokenClient wraps http.Client and injects the scoped APICall token as
// an Authorization Bearer header whenever the caller has not already set one.
type scopedTokenClient struct {
	inner *http.Client
}

// NewScopedTokenClient returns a *scopedTokenClient that injects the scoped
// APICall token into outbound HTTP requests. This concrete type satisfies the
// ClientInterface expected by github.com/kyverno/sdk/cel/libs/http.NewHTTP.
func NewScopedTokenClient() *scopedTokenClient {
	return &scopedTokenClient{inner: &http.Client{}}
}

func (c *scopedTokenClient) Do(req *http.Request) (*http.Response, error) {
	if req.Header.Get("Authorization") == "" {
		if b, err := os.ReadFile(scopedTokenPath); err == nil {
			req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(b)))
		}
	}
	return c.inner.Do(req)
}
