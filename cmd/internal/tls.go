package internal

import (
	"github.com/kyverno/kyverno/pkg/config"
	sharedtls "github.com/kyverno/pkg/tls"
)

// NewSharedTLSConfig builds a *sharedtls.Config used by the shared
// certmanager controller from the given service name and the Kyverno
// namespace. Centralizing this construction avoids drift between the
// kyverno, cleanup-controller, and metrics binaries.
func NewSharedTLSConfig(serviceName string) *sharedtls.Config {
	return &sharedtls.Config{
		ServiceName: serviceName,
		Namespace:   config.KyvernoNamespace(),
	}
}
