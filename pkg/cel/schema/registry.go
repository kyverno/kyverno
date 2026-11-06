package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/cel-go/cel"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

// The types below mirror the shapes in the proposal (PayloadSchema,
// ValidatingPolicy with evaluation.mode: Schema, PayloadBinding). They are
// local spike types, not the future kyverno/api types.

type Ref struct {
	Name string `json:"name"`
}

type Discriminator struct {
	Pointer string `json:"pointer"`
	Equals  string `json:"equals"`
}

type Match struct {
	Discriminators []Discriminator `json:"discriminators"`
	RequiredPaths  []string        `json:"requiredPaths"`
}

type PayloadSchemaSpec struct {
	Dialect              string          `json:"dialect"`
	CompatibilityVersion string          `json:"compatibilityVersion"`
	Schema               json.RawMessage `json:"schema"`
	Match                Match           `json:"match"`
	Exposure             Exposure        `json:"exposure"`
	Computed             []Computed      `json:"computed"`
}

type Validation struct {
	Expression string `json:"expression"`
	Message    string `json:"message"`
}

type PolicySpec struct {
	Evaluation struct {
		Mode      string `json:"mode"`
		SchemaRef Ref    `json:"schemaRef"`
	} `json:"evaluation"`
	ValidationActions []string     `json:"validationActions"`
	Validations       []Validation `json:"validations"`
}

// Limits bound the work a single request may cause. MaxCELCost is parsed for
// API parity but not enforced yet (see FINDINGS.md).
type Limits struct {
	MaxBodyBytes  int64 `json:"maxBodyBytes"`
	MaxDepth      int   `json:"maxDepth"`
	MaxCandidates int   `json:"maxCandidates"`
	MaxCELCost    int64 `json:"maxCELCost"`
	TimeoutMillis int   `json:"timeoutMillis"`
}

const (
	defaultMaxBodyBytes  = 1 << 20
	defaultMaxDepth      = 64
	defaultMaxCandidates = 16
)

type BindingSpec struct {
	Route struct {
		Path    string   `json:"path"`
		Methods []string `json:"methods"`
	} `json:"route"`
	Authorization struct {
		AllowedPrincipals []string `json:"allowedPrincipals"`
	} `json:"authorization"`
	Selection struct {
		SchemaRef  *Ref  `json:"schemaRef"`
		Candidates []Ref `json:"candidates"`
	} `json:"selection"`
	RequiredPolicies []Ref `json:"requiredPolicies"`
	Enforcement      struct {
		RequireApplicablePolicy *bool  `json:"requireApplicablePolicy"`
		DefaultOnNoMatch        string `json:"defaultOnNoMatch"`
	} `json:"enforcement"`
	Limits   Limits `json:"limits"`
	Response struct {
		StatusCode     int    `json:"statusCode"`
		BodyExpression string `json:"bodyExpression"`
	} `json:"response"`
}

type document struct {
	Kind     string `json:"kind"`
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec json.RawMessage `json:"spec"`
}

type compiledValidation struct {
	expr    *Expr
	message string
}

type compiledPolicy struct {
	name        string
	enforcing   bool // has a Deny action; audit-only policies never block
	schema      *compiledSchema
	validations []compiledValidation
}

type compiledSchema struct {
	name     string
	digest   string
	schema   *Schema
	match    Match
	policies []*compiledPolicy
}

type compiledBinding struct {
	name     string
	spec     BindingSpec
	explicit *compiledSchema
	cands    []*compiledSchema
	required []*compiledPolicy
	limits   Limits
	allowed  map[string]bool
	response cel.Program
}

// Registry is an immutable, fully compiled set of schemas, policies and
// bindings, indexed for the request path.
type Registry struct {
	opts     Options
	schemas  map[string]*compiledSchema
	bindings map[string]*compiledBinding // key: path
}

// Options configure the serving side. With a nil Principal the registry runs
// unauthenticated (spike/demo only) and ignores authorization settings.
type Options struct {
	// Principal returns the authenticated caller for a request, taken from the
	// deployment's listener (e.g. mTLS), never from caller-supplied headers.
	Principal func(*http.Request) (string, bool)
}

// SchemaDigest returns the canonical spec digest recorded for a schema.
func (r *Registry) SchemaDigest(name string) string {
	if cs, ok := r.schemas[name]; ok {
		return cs.digest
	}
	return ""
}

// specDigest hashes the complete executable spec over canonical JSON (sorted keys).
func specDigest(raw json.RawMessage) (string, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", err
	}
	canon, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canon)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// LoadDir loads every .yaml/.yml/.json file in dir and compiles the result.
func LoadDir(dir string, opts Options) (*Registry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var docs []document
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if e.IsDir() || (ext != ".yaml" && ext != ".yml" && ext != ".json") {
			continue
		}
		f, err := os.Open(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		dec := utilyaml.NewYAMLOrJSONDecoder(f, 4096)
		for {
			var d document
			if err := dec.Decode(&d); err != nil {
				f.Close()
				if errors.Is(err, io.EOF) {
					break
				}
				return nil, fmt.Errorf("%s: %w", e.Name(), err)
			}
			if d.Kind != "" {
				docs = append(docs, d)
			}
		}
	}
	return compile(docs, opts)
}

// compile resolves references and compiles every expression up front, so a
// bad schema, policy or binding is rejected at load time, never per request.
func compile(docs []document, opts Options) (*Registry, error) {
	r := &Registry{opts: opts, schemas: map[string]*compiledSchema{}, bindings: map[string]*compiledBinding{}}
	policyDocs := map[string]PolicySpec{}
	var policyOrder []string
	bindingDocs := map[string]BindingSpec{}
	var bindingOrder []string
	for _, d := range docs {
		name := d.Metadata.Name
		switch d.Kind {
		case "PayloadSchema":
			var sp PayloadSchemaSpec
			if err := json.Unmarshal(d.Spec, &sp); err != nil {
				return nil, fmt.Errorf("PayloadSchema %s: %w", name, err)
			}
			if sp.Dialect != DialectV1 {
				return nil, fmt.Errorf("PayloadSchema %s: dialect must be %s, got %q", name, DialectV1, sp.Dialect)
			}
			if sp.CompatibilityVersion != "" && sp.CompatibilityVersion != "1" {
				return nil, fmt.Errorf("PayloadSchema %s: unsupported compatibilityVersion %q", name, sp.CompatibilityVersion)
			}
			if err := checkProfile(sp.Schema); err != nil {
				return nil, fmt.Errorf("PayloadSchema %s: %w", name, err)
			}
			s, err := New(sp.Schema, sp.Computed)
			if err != nil {
				return nil, fmt.Errorf("PayloadSchema %s: %w", name, err)
			}
			if _, dup := r.schemas[name]; dup {
				return nil, fmt.Errorf("duplicate PayloadSchema %s", name)
			}
			digest, err := specDigest(d.Spec)
			if err != nil {
				return nil, err
			}
			r.schemas[name] = &compiledSchema{name: name, digest: digest, schema: s.WithExposure(sp.Exposure), match: sp.Match}
		case "ValidatingPolicy":
			var sp PolicySpec
			if err := json.Unmarshal(d.Spec, &sp); err != nil {
				return nil, fmt.Errorf("ValidatingPolicy %s: %w", name, err)
			}
			policyDocs[name] = sp
			policyOrder = append(policyOrder, name)
		case "PayloadBinding":
			var sp BindingSpec
			if err := json.Unmarshal(d.Spec, &sp); err != nil {
				return nil, fmt.Errorf("PayloadBinding %s: %w", name, err)
			}
			bindingDocs[name] = sp
			bindingOrder = append(bindingOrder, name)
		default:
			return nil, fmt.Errorf("unsupported kind %q", d.Kind)
		}
	}
	policies := map[string]*compiledPolicy{}
	for _, name := range policyOrder {
		sp := policyDocs[name]
		if sp.Evaluation.Mode != "Schema" {
			return nil, fmt.Errorf("ValidatingPolicy %s: evaluation.mode must be Schema", name)
		}
		cs, ok := r.schemas[sp.Evaluation.SchemaRef.Name]
		if !ok {
			return nil, fmt.Errorf("ValidatingPolicy %s: unknown schemaRef %q", name, sp.Evaluation.SchemaRef.Name)
		}
		cp := &compiledPolicy{name: name, schema: cs, enforcing: len(sp.ValidationActions) == 0}
		for _, a := range sp.ValidationActions {
			switch a {
			case "Deny":
				cp.enforcing = true
			case "Audit", "Warn":
			default:
				return nil, fmt.Errorf("ValidatingPolicy %s: unknown validationAction %q", name, a)
			}
		}
		for i, v := range sp.Validations {
			e, err := cs.schema.Compile(v.Expression)
			if err != nil {
				return nil, fmt.Errorf("ValidatingPolicy %s validation %d: %w", name, i, err)
			}
			if !e.IsBool() {
				return nil, fmt.Errorf("ValidatingPolicy %s validation %d: expression must return bool", name, i)
			}
			cp.validations = append(cp.validations, compiledValidation{expr: e, message: v.Message})
		}
		cs.policies = append(cs.policies, cp)
		policies[name] = cp
	}
	respEnv, err := cel.NewEnv(cel.Variable("decision", cel.MapType(cel.StringType, cel.DynType)))
	if err != nil {
		return nil, err
	}
	for _, name := range bindingOrder {
		sp := bindingDocs[name]
		b := &compiledBinding{name: name, spec: sp}
		switch {
		case sp.Selection.SchemaRef != nil && len(sp.Selection.Candidates) > 0:
			return nil, fmt.Errorf("PayloadBinding %s: set schemaRef or candidates, not both", name)
		case sp.Selection.SchemaRef != nil:
			cs, ok := r.schemas[sp.Selection.SchemaRef.Name]
			if !ok {
				return nil, fmt.Errorf("PayloadBinding %s: unknown schemaRef %q", name, sp.Selection.SchemaRef.Name)
			}
			b.explicit = cs
		case len(sp.Selection.Candidates) > 0:
			for _, c := range sp.Selection.Candidates {
				cs, ok := r.schemas[c.Name]
				if !ok {
					return nil, fmt.Errorf("PayloadBinding %s: unknown candidate %q", name, c.Name)
				}
				b.cands = append(b.cands, cs)
			}
		default:
			return nil, fmt.Errorf("PayloadBinding %s: selection needs schemaRef or candidates", name)
		}
		b.limits = sp.Limits
		if b.limits.MaxBodyBytes == 0 {
			b.limits.MaxBodyBytes = defaultMaxBodyBytes
		}
		if b.limits.MaxDepth == 0 {
			b.limits.MaxDepth = defaultMaxDepth
		}
		if b.limits.MaxCandidates == 0 {
			b.limits.MaxCandidates = defaultMaxCandidates
		}
		if len(b.cands) > b.limits.MaxCandidates {
			return nil, fmt.Errorf("PayloadBinding %s: %d candidates exceeds maxCandidates %d", name, len(b.cands), b.limits.MaxCandidates)
		}
		for _, rp := range sp.RequiredPolicies {
			cp, ok := policies[rp.Name]
			if !ok {
				return nil, fmt.Errorf("PayloadBinding %s: unknown required policy %q", name, rp.Name)
			}
			if !cp.enforcing {
				return nil, fmt.Errorf("PayloadBinding %s: required policy %q has no Deny action", name, rp.Name)
			}
			b.required = append(b.required, cp)
		}
		// Coverage floor: every selectable schema needs an enforcing required policy.
		targets := b.cands
		if b.explicit != nil {
			targets = []*compiledSchema{b.explicit}
		}
		for _, cs := range targets {
			covered := false
			for _, cp := range b.required {
				covered = covered || cp.schema == cs
			}
			if !covered {
				return nil, fmt.Errorf("PayloadBinding %s: schema %q has no required enforcing policy", name, cs.name)
			}
		}
		if opts.Principal != nil {
			if len(sp.Authorization.AllowedPrincipals) == 0 {
				return nil, fmt.Errorf("PayloadBinding %s: authorization.allowedPrincipals must not be empty", name)
			}
			if sp.Enforcement.RequireApplicablePolicy != nil && !*sp.Enforcement.RequireApplicablePolicy {
				return nil, fmt.Errorf("PayloadBinding %s: requireApplicablePolicy must stay true on authorization routes", name)
			}
			b.allowed = map[string]bool{}
			for _, p := range sp.Authorization.AllowedPrincipals {
				b.allowed[p] = true
			}
		}
		if sp.Route.Path == "" || len(sp.Route.Methods) == 0 {
			return nil, fmt.Errorf("PayloadBinding %s: route.path and route.methods are required", name)
		}
		if _, dup := r.bindings[sp.Route.Path]; dup {
			return nil, fmt.Errorf("PayloadBinding %s: route %s already registered", name, sp.Route.Path)
		}
		if expr := sp.Response.BodyExpression; expr != "" {
			ast, iss := respEnv.Compile(expr)
			if iss.Err() != nil {
				return nil, fmt.Errorf("PayloadBinding %s bodyExpression: %w", name, iss.Err())
			}
			if b.response, err = respEnv.Program(ast); err != nil {
				return nil, err
			}
		}
		r.bindings[sp.Route.Path] = b
	}
	return r, nil
}

// Summary lists what was loaded, for startup output.
func (r *Registry) Summary() []string {
	var out []string
	for _, b := range r.bindings {
		sel := "candidates"
		if b.explicit != nil {
			sel = "schema " + b.explicit.name
		}
		if b.explicit != nil {
			sel += " " + b.explicit.digest[:19]
		}
		out = append(out, fmt.Sprintf("%s %s -> %s (%s)", strings.Join(b.spec.Route.Methods, ","), b.spec.Route.Path, b.name, sel))
	}
	sort.Strings(out)
	return out
}
