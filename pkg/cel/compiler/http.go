package compiler

import (
	"fmt"

	"github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/sdk/cel/libs/http"
)

// NewLazyCELHTTPContext returns an http.ContextInterface that is safe to use at
// both admission (type-checking) and runtime (enforcement) time.
//
// Construction never errors: the operator's blocklist/allowlist flags are read
// lazily on each Get/Post call. For namespaced policies, the
// AllowHTTPInNamespacedPolicies toggle is also enforced at call time; attempts
// to call http.* from a namespaced policy when the toggle is disabled will
// return an error at evaluation time rather than causing an admission rejection.
func NewLazyCELHTTPContext(namespace string) http.ContextInterface {
	inner := http.NewLazyHTTPContext(toggle.HTTPBlocklist.Values, toggle.HTTPAllowlist.Values)
	if namespace == "" {
		return inner
	}
	return &namespacedHTTPContext{inner: inner}
}

// namespacedHTTPContext wraps an http.ContextInterface and enforces the
// AllowHTTPInNamespacedPolicies toggle at call time for namespaced policies.
type namespacedHTTPContext struct {
	inner http.ContextInterface
}

func (n *namespacedHTTPContext) Get(url string, headers map[string]string) (any, error) {
	if !toggle.AllowHTTPInNamespacedPolicies.Enabled() {
		return nil, fmt.Errorf("http calls are not allowed in namespaced policies: set --allowHTTPInNamespacedPolicies to enable")
	}
	return n.inner.Get(url, headers)
}

func (n *namespacedHTTPContext) Post(url string, data any, headers map[string]string) (any, error) {
	if !toggle.AllowHTTPInNamespacedPolicies.Enabled() {
		return nil, fmt.Errorf("http calls are not allowed in namespaced policies: set --allowHTTPInNamespacedPolicies to enable")
	}
	return n.inner.Post(url, data, headers)
}

func (n *namespacedHTTPContext) Client(caBundle string) (http.ContextInterface, error) {
	innerWithCA, err := n.inner.Client(caBundle)
	if err != nil {
		return nil, err
	}
	return &namespacedHTTPContext{inner: innerWithCA}, nil
}

// ValidateHTTPFlags eagerly validates the current blocklist and allowlist flag
// values, returning an error if any entry is malformed. Call this at startup to
// fail fast before serving any traffic.
func ValidateHTTPFlags() error {
	_, err := http.NewHTTPWithBlocklist(toggle.HTTPBlocklist.Values(), toggle.HTTPAllowlist.Values())
	if err != nil {
		return fmt.Errorf("invalid CEL http configuration: %w", err)
	}
	return nil
}

// ExpressionsUseHTTP reports whether any of the given CEL expression strings
// reference the "http" identifier. It parses (but does not type-check) each
// expression and walks the AST looking for ident nodes named "http". Malformed
// expressions are skipped — compilation errors are surfaced separately.
func ExpressionsUseHTTP(expressions ...string) bool {
	env, err := cel.NewEnv()
	if err != nil {
		return false
	}
	for _, expr := range expressions {
		if expr == "" {
			continue
		}
		ast, issues := env.Parse(expr)
		if issues != nil && issues.Err() != nil {
			continue
		}
		nav := celast.NavigateAST(ast.NativeRep())
		matches := celast.MatchDescendants(nav, func(e celast.NavigableExpr) bool {
			return e.Kind() == celast.IdentKind && e.AsIdent() == "http"
		})
		if len(matches) > 0 {
			return true
		}
	}
	return false
}
