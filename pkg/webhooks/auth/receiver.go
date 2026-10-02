package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/kyverno/kyverno/pkg/config"
)

// Receiver holds the same trusted destination inputs used by webhook registration.
type Receiver struct {
	Verifier  Verifier
	server    string
	port      int32
	name      string
	namespace string
}

func NewReceiver(v Verifier, server string, port int32) *Receiver {
	return &Receiver{
		Verifier:  v,
		server:    server,
		port:      port,
		name:      config.KyvernoServiceName(),
		namespace: config.KyvernoNamespace(),
	}
}

// Audience preserves the route path as matched by the HTTP router.
func (r *Receiver) Audience(path string) string {
	if r.server != "" {
		return "https://" + r.server + path
	}
	port := r.port
	if port == 0 {
		port = 443
	}
	if path == "" {
		path = "/"
	}
	return fmt.Sprintf("https://%s.%s.svc:%s%s", r.name, r.namespace, strconv.FormatInt(int64(port), 10), path)
}

func (r *Receiver) VerifyRequest(request *http.Request, apiGroup string) error {
	if r == nil || r.Verifier == nil {
		return errors.New("webhook authentication is unavailable")
	}
	if request.URL.RawPath != "" || request.URL.RawQuery != "" {
		return errors.New("admission URL differs from the registered route")
	}
	values := request.Header.Values("Authorization")
	if len(values) != 1 {
		return errors.New("invalid authorization header")
	}
	parts := strings.Split(values[0], " ")
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" || strings.ContainsAny(parts[1], ",\t\r\n") {
		return errors.New("invalid bearer token")
	}
	return r.Verifier.Verify(request.Context(), parts[1], r.Audience(request.URL.Path), apiGroup)
}
