package schema

import (
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"time"
)

// Decision is the aggregated verdict exposed to a binding's bodyExpression
// as `decision`.
type Decision struct {
	Allowed  bool
	Message  string
	Schema   string
	Policies []string
}

// ServeHTTP implements the request path from the proposal: route -> binding
// -> schema selection -> validation -> policies -> deny-overrides -> response.
func (r *Registry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	start := time.Now()
	b, ok := r.bindings[req.URL.Path]
	if !ok {
		http.NotFound(w, req)
		return
	}
	if !slices.Contains(b.spec.Route.Methods, req.Method) {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
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
			log.Printf("%s %s binding=%s forbidden principal=%q", req.Method, req.URL.Path, b.name, principal)
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
		d = Decision{Message: "reading body: " + err.Error()}
	} else {
		d = r.decide(b, raw, transport, start)
	}
	log.Printf("%s %s binding=%s schema=%q allowed=%t policies=%v msg=%q (%s)",
		req.Method, req.URL.Path, b.name, d.Schema, d.Allowed, d.Policies, d.Message, time.Since(start).Round(time.Microsecond))
	status := b.spec.Response.StatusCode
	if status == 0 {
		status = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(b.render(d))
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
func (r *Registry) decide(b *compiledBinding, raw []byte, transport map[string]any, start time.Time) Decision {
	doc, err := ParseJSON(raw)
	if err != nil {
		return Decision{Message: err.Error()}
	}
	if depth(doc) > b.limits.MaxDepth {
		return Decision{Message: fmt.Sprintf("payload nesting exceeds maxDepth %d", b.limits.MaxDepth)}
	}
	cs, err := b.selectSchema(doc)
	if err != nil {
		return Decision{Message: "schema selection: " + err.Error()}
	}
	d := Decision{Schema: cs.name}
	payload, err := cs.schema.Build(doc, transport)
	if err != nil {
		d.Message = err.Error()
		return d
	}
	var denials []string
	for _, p := range cs.policies {
		d.Policies = append(d.Policies, p.name)
		for _, v := range p.validations {
			if t := b.limits.TimeoutMillis; t > 0 && time.Since(start) > time.Duration(t)*time.Millisecond {
				d.Message = fmt.Sprintf("evaluation exceeded timeoutMillis %d", t)
				return d
			}
			out, err := v.expr.Eval(payload)
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
				log.Printf("audit: %s", failure)
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
// otherwise a default {"decision": ...} shape.
func (b *compiledBinding) render(d Decision) any {
	if b.response == nil {
		if d.Allowed {
			return map[string]any{"decision": "granted"}
		}
		return map[string]any{"decision": "denied", "reason": d.Message}
	}
	out, _, err := b.response.Eval(map[string]any{"decision": map[string]any{
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

// depth returns the maximum container nesting of decoded JSON.
func depth(v any) int {
	max := 0
	switch t := v.(type) {
	case map[string]any:
		for _, c := range t {
			if d := depth(c); d > max {
				max = d
			}
		}
		return max + 1
	case []any:
		for _, c := range t {
			if d := depth(c); d > max {
				max = d
			}
		}
		return max + 1
	}
	return 0
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
