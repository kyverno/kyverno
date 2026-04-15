package compiler

import (
	"fmt"

	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/sdk/cel/libs/http"
)

// NewCELHTTPContext constructs an http.ContextInterface using the operator-configured
// blocklist and allowlist (see --httpBlocklist / --httpAllowlist flags).
// Returns an error only if the configured entries are malformed.
func NewCELHTTPContext() (http.ContextInterface, error) {
	ctx, err := http.NewHTTPWithBlocklist(toggle.HTTPBlocklist.Values(), toggle.HTTPAllowlist.Values())
	if err != nil {
		return nil, fmt.Errorf("invalid CEL http configuration: %w", err)
	}
	return ctx, nil
}
