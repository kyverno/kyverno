package compiler

import (
	"fmt"
	"sync"

	"github.com/google/cel-go/cel"
	celast "github.com/google/cel-go/common/ast"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/sdk/cel/libs/http"
)

// sharedHTTPContext is built once on the first http.* call and reused across
// admission requests so the underlying http.Transport is not recreated per call.
// It applies the operator-configured SSRF blocklist and is used for namespaced policies.
var sharedHTTPContext = &cachedHTTPContext{}

// clusterHTTPContext is the unrestricted counterpart used for cluster-scoped policies.
// Cluster-scoped policies require cluster-admin privileges, so no SSRF blocklist is needed.
var clusterHTTPContext = &cachedHTTPContext{unrestricted: true}

type cachedHTTPContext struct {
	mu           sync.Mutex
	cached       http.ContextInterface
	unrestricted bool
}

func (c *cachedHTTPContext) getOrBuild() (http.ContextInterface, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cached != nil {
		return c.cached, nil
	}
	var blocklist, allowlist []string
	if !c.unrestricted {
		blocklist = toggle.HTTPBlocklist.Values()
		allowlist = toggle.HTTPAllowlist.Values()
	}
	ctx, err := http.NewHTTPWithBlocklist(blocklist, allowlist)
	if err != nil {
		return nil, err
	}
	c.cached = ctx
	return ctx, nil
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
		return clusterHTTPContext
	}
	return &namespacedHTTPContext{inner: sharedHTTPContext}
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
			return e.Kind() == celast.IdentKind && e.AsIdent() == "http"
		})
		if len(matches) > 0 {
			return true
		}
	}
	return false
}
