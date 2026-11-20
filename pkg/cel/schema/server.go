package schema

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/kyverno/kyverno/pkg/cel/requestbudget"
	"github.com/kyverno/kyverno/pkg/logging"
)

var logger = logging.WithName("payload-schema")

// Decision is the aggregated verdict exposed to a binding's bodyExpression
// as `decision`.
type Decision struct {
	Allowed  bool
	Message  string
	Schema   string
	Policies []string
	// Rejected marks a failure before a typed object existed (unreadable or
	// malformed body, no schema selected, validation failure). The binding's
	// bodyExpression never runs for these; a fixed denial is sent instead.
	Rejected bool
}

// ServeHTTP implements the request path from the proposal: route -> binding
// -> schema selection -> validation -> policies -> deny-overrides -> response.
func (r *Registry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	start := time.Now()
	// An unknown method on a known path is indistinguishable from an unknown
	// route, so a protected route is not disclosed.
	b, ok := r.bindings[routeKey(req.Method, req.URL.Path)]
	if !ok {
		http.NotFound(w, req)
		return
	}
	ctx, _ := requestbudget.Ensure(req.Context())
	if t := b.limits.TimeoutMillis; t > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(t)*time.Millisecond)
		defer cancel()
	}
	transport := transportOf(req)
	if r.opts.Principal != nil {
		// Authenticate and authorize before reading the body or selecting a schema.
		principal, ok := r.opts.Principal(req)
		if !ok {
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		if !b.allowed[principal] {
			logger.V(2).Info("forbidden principal", "binding", b.name, "principal", principal)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		transport["principal"] = principal
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, req.Body, b.limits.MaxBodyBytes))
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	var d Decision
	if err != nil {
		d = Decision{Message: "reading body: " + err.Error(), Rejected: true}
	} else {
		d = r.decide(ctx, b, raw, transport)
	}
	logger.V(2).Info("decision", "method", req.Method, "path", req.URL.Path, "binding", b.name,
		"schema", d.Schema, "allowed", d.Allowed, "policies", d.Policies, "message", d.Message,
		"duration", time.Since(start).Round(time.Microsecond).String())
	status := b.spec.Response.StatusCode
	if status == 0 {
		status = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(b.render(ctx, d))
}

func transportOf(req *http.Request) map[string]any {
	headers := map[string]any{}
	for k, v := range req.Header {
		l := make([]any, len(v))
		for i, s := range v {
			l[i] = s
		}
		headers[strings.ToLower(k)] = l
	}
	return map[string]any{"method": req.Method, "path": req.URL.Path, "headers": headers}
}

// decide is fail-closed: anything unexpected yields a denial.
func (r *Registry) decide(ctx context.Context, b *compiledBinding, raw []byte, transport map[string]any) Decision {
	doc, err := ParseJSON(raw, b.limits.MaxDepth)
	if err != nil {
		return Decision{Message: err.Error(), Rejected: true}
	}
	cs, err := b.selectSchema(doc)
	if err != nil {
		return Decision{Message: "schema selection: " + err.Error(), Rejected: true}
	}
	d := Decision{Schema: cs.name}
	payload, err := cs.schema.Build(ctx, doc, transport)
	if err != nil {
		d.Message = err.Error()
		d.Rejected = true
		return d
	}
	var denials []string
	for _, p := range cs.policies {
		d.Policies = append(d.Policies, p.name)
		for _, v := range p.validations {
			out, err := v.expr.Run(ctx, payload)
			var failure string
			switch {
			case err != nil:
				failure = fmt.Sprintf("%s: evaluation error: %v", p.name, err)
			case out != true:
				msg := v.message
				if msg == "" {
					msg = "validation failed"
				}
				failure = fmt.Sprintf("%s: %s", p.name, msg)
			default:
				continue
			}
			if p.enforcing {
				denials = append(denials, failure)
			} else {
				logger.V(2).Info("audit", "failure", failure)
			}
		}
	}
	required := b.spec.Enforcement.RequireApplicablePolicy
	switch {
	case len(denials) > 0:
		d.Message = strings.Join(denials, "; ")
	case len(cs.policies) == 0 && (required == nil || *required || b.spec.Enforcement.DefaultOnNoMatch != "Allow"):
		d.Message = "no applicable policy"
	default:
		d.Allowed = true
	}
	return d
}

// selectSchema picks exactly one schema; it never retries a weaker one.
func (b *compiledBinding) selectSchema(doc any) (*compiledSchema, error) {
	if b.explicit != nil {
		return b.explicit, nil
	}
	var matched []*compiledSchema
	for _, c := range b.cands {
		if c.match.matches(doc) {
			matched = append(matched, c)
		}
	}
	switch len(matched) {
	case 1:
		return matched[0], nil
	case 0:
		return nil, errors.New("no candidate schema matches")
	default:
		names := make([]string, len(matched))
		for i, m := range matched {
			names[i] = m.name
		}
		return nil, fmt.Errorf("ambiguous: %s", strings.Join(names, ", "))
	}
}

func (m Match) matches(doc any) bool {
	for _, d := range m.Discriminators {
		v, ok := pointer(doc, d.Pointer)
		if !ok || fmt.Sprint(v) != d.Equals {
			return false
		}
	}
	for _, p := range m.RequiredPaths {
		if _, ok := pointer(doc, p); !ok {
			return false
		}
	}
	return true
}

// pointer resolves an RFC 6901 JSON Pointer against decoded JSON.
func pointer(doc any, ptr string) (any, bool) {
	if ptr == "" {
		return doc, true
	}
	if !strings.HasPrefix(ptr, "/") {
		return nil, false
	}
	cur := doc
	for _, tok := range strings.Split(ptr[1:], "/") {
		tok = strings.ReplaceAll(strings.ReplaceAll(tok, "~1", "/"), "~0", "~")
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = m[tok]; !ok {
			return nil, false
		}
	}
	return cur, true
}

// render builds the response body: the binding's bodyExpression if set,
// otherwise a default {"decision": ...} shape. Failures before a typed object
// existed always get the fixed default denial, never the template.
func (b *compiledBinding) render(ctx context.Context, d Decision) any {
	if b.response == nil || d.Rejected {
		if d.Allowed {
			return map[string]any{"decision": "granted"}
		}
		return map[string]any{"decision": "denied", "reason": d.Message}
	}
	out, _, err := b.response.ContextEval(ctx, map[string]any{"decision": map[string]any{
		"allowed": d.Allowed, "message": d.Message, "schema": d.Schema,
	}})
	if err != nil {
		return map[string]any{"decision": "denied", "reason": "response expression: " + err.Error()}
	}
	native, err := out.ConvertToNative(reflect.TypeOf(map[string]any{}))
	if err != nil {
		return map[string]any{"decision": "denied", "reason": "response expression: " + err.Error()}
	}
	return native
}

// MTLSPrincipal derives the caller identity from a verified client
// certificate: the first URI SAN (e.g. a SPIFFE ID), else the common name.
func MTLSPrincipal(req *http.Request) (string, bool) {
	if req.TLS == nil || len(req.TLS.VerifiedChains) == 0 || len(req.TLS.VerifiedChains[0]) == 0 {
		return "", false
	}
	return principalOf(req.TLS.VerifiedChains[0][0])
}

func principalOf(c *x509.Certificate) (string, bool) {
	if len(c.URIs) > 0 {
		return c.URIs[0].String(), true
	}
	if c.Subject.CommonName != "" {
		return c.Subject.CommonName, true
	}
	return "", false
}
