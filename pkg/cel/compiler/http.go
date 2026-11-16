package compiler

import (
	"fmt"
	"strings"
	"sync"

	"github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"
	"github.com/kyverno/kyverno/pkg/engine/apicall"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/sdk/extensions/cel/libs/globalcontext"
	"github.com/kyverno/sdk/extensions/cel/libs/http"
)

// sharedHTTPContext is built once on the first http.* call and reused across
// admission requests so the underlying http.Transport is not recreated per call.
// It applies the operator-configured SSRF blocklist and allowlist for all policies.
var sharedHTTPContext = &cachedHTTPContext{}

type cachedHTTPContext struct {
	mu     sync.Mutex
	cached http.ContextInterface
}

func (c *cachedHTTPContext) getOrBuild() (http.ContextInterface, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cached != nil {
		return c.cached, nil
	}
	ctx, err := http.NewHTTPWithBlocklist(toggle.HTTPBlocklist.Values(), toggle.HTTPAllowlist.Values())
	if err != nil {
		return nil, err
	}
	// The SDK constructors hardcode http.DefaultClient and take no client option,
	// so CEL http.Get/Post would otherwise go out without the projected
	// ServiceAccount token. Wrap the context so the token is attached per call.
	c.cached = &tokenHTTPContext{inner: ctx}
	return c.cached, nil
}

// tokenHTTPContext decorates an http.ContextInterface so CEL http.Get and
// http.Post attach the projected ServiceAccount token (custom audience
// kyverno-svc.kyverno.io) as an Authorization Bearer header when the policy did
// not already supply one. Contexts derived through Client(caBundle) are wrapped
// too, so CA-bundle clients authenticate as well.
type tokenHTTPContext struct {
	inner http.ContextInterface
}

func (c *tokenHTTPContext) Get(url string, headers map[string]string) (any, error) {
	return c.inner.Get(url, withScopedToken(headers))
}

func (c *tokenHTTPContext) Post(url string, data any, headers map[string]string) (any, error) {
	return c.inner.Post(url, data, withScopedToken(headers))
}

func (c *tokenHTTPContext) Client(caBundle string) (http.ContextInterface, error) {
	inner, err := c.inner.Client(caBundle)
	if err != nil {
		return nil, err
	}
	return &tokenHTTPContext{inner: inner}, nil
}

// withScopedToken returns headers with the scoped APICall token attached as an
// Authorization Bearer header, unless the caller already set one (compared
// case-insensitively). The caller's map is never mutated. When no token is
// available the headers are returned unchanged.
func withScopedToken(headers map[string]string) map[string]string {
	token, ok := apicall.ScopedToken()
	if !ok || token == "" {
		return headers
	}
	for k := range headers {
		if strings.EqualFold(k, "Authorization") {
			return headers
		}
	}
	out := make(map[string]string, len(headers)+1)
	for k, v := range headers {
		out[k] = v
	}
	out["Authorization"] = "Bearer " + token
	return out
}

func (c *cachedHTTPContext) Get(url string, headers map[string]string) (any, error) {
	ctx, err := c.getOrBuild()
	if err != nil {
		return nil, err
	}
	return ctx.Get(url, headers)
}

func (c *cachedHTTPContext) Post(url string, data any, headers map[string]string) (any, error) {
	ctx, err := c.getOrBuild()
	if err != nil {
		return nil, err
	}
	return ctx.Post(url, data, headers)
}

func (c *cachedHTTPContext) Client(caBundle string) (http.ContextInterface, error) {
	ctx, err := c.getOrBuild()
	if err != nil {
		return nil, err
	}
	return ctx.Client(caBundle)
}

// NewLazyCELHTTPContext returns an http.ContextInterface safe for use at both
// admission (type-checking) and evaluation time. For namespaced policies the
// AllowHTTPInNamespacedPolicies toggle is enforced at call time.
func NewLazyCELHTTPContext(namespace string) http.ContextInterface {
	if namespace == "" {
		return sharedHTTPContext
	}
	return &namespacedHTTPContext{inner: sharedHTTPContext}
}

func ConfineGlobalContext(inner globalcontext.ContextInterface, namespace string) globalcontext.ContextInterface {
	if namespace == "" {
		return inner
	}
	return namespacedGlobalContext{}
}

type namespacedGlobalContext struct{}

func (namespacedGlobalContext) GetGlobalReference(_, _ string) (any, error) {
	return nil, fmt.Errorf("globalContext.* is not allowed in namespaced policies")
}

// namespacedHTTPContext enforces the AllowHTTPInNamespacedPolicies toggle at call time.
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

// ValidateHTTPFlags validates the blocklist and allowlist flag values at startup.
func ValidateHTTPFlags() error {
	_, err := http.NewHTTPWithBlocklist(toggle.HTTPBlocklist.Values(), toggle.HTTPAllowlist.Values())
	if err != nil {
		return fmt.Errorf("invalid CEL http configuration: %w", err)
	}
	return nil
}

var (
	parseEnvOnce sync.Once
	parseEnv     *cel.Env
)

func getParseEnv() *cel.Env {
	parseEnvOnce.Do(func() {
		env, err := cel.NewEnv()
		if err != nil {
			return
		}
		parseEnv = env
	})
	return parseEnv
}

// ExpressionsUseHTTP reports whether any expression references the "http" identifier.
// Expressions are parsed but not type-checked; malformed expressions are skipped.
func ExpressionsUseHTTP(expressions ...string) bool {
	return expressionsUseIdent("http", expressions...)
}

func ExpressionsUseGlobalContext(expressions ...string) bool {
	return expressionsUseIdent(GlobalContextKey, expressions...)
}

func expressionsUseIdent(ident string, expressions ...string) bool {
	env := getParseEnv()
	if env == nil {
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
			return e.Kind() == celast.IdentKind && e.AsIdent() == ident
		})
		if len(matches) > 0 {
			return true
		}
	}
	return false
}
