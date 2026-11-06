package evaluator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	pkgdeprecations "github.com/kyverno/kyverno/pkg/deprecations"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

// PolicySet is the set of policies (and policy-scoped CEL exceptions) an evaluation runs against.
// It is built once (by apply, per invocation, or by serve, at startup) and is treated as read-only
// by Evaluate.
type PolicySet struct {
	policy.LoaderResults
}

// Add merges the policies of one loader result into the set. Non-fatal loader errors and warnings
// are kept so callers can decide whether to report or reject them.
func (s *PolicySet) Add(results *policy.LoaderResults) {
	s.Merge(results)
}

// GenericPolicies returns the policies the resource loader needs to know about (to decide which
// resource kinds to fetch from a cluster), in the same order apply has always used.
func (s *PolicySet) GenericPolicies() []engineapi.GenericPolicy {
	out := make([]engineapi.GenericPolicy, 0, len(s.VAPs)+len(s.MAPs)+len(s.ValidatingPolicies)+len(s.ImageValidatingPolicies)+len(s.GeneratingPolicies)+len(s.DeletingPolicies)+len(s.MutatingPolicies))
	for i := range s.VAPs {
		out = append(out, engineapi.NewValidatingAdmissionPolicy(&s.VAPs[i]))
	}
	for i := range s.MAPs {
		out = append(out, engineapi.NewMutatingAdmissionPolicy(&s.MAPs[i]))
	}
	for _, pol := range s.ValidatingPolicies {
		out = append(out, engineapi.NewValidatingPolicyFromLike(pol))
	}
	for _, pol := range s.ImageValidatingPolicies {
		out = append(out, engineapi.NewImageValidatingPolicyFromLike(pol))
	}
	for _, pol := range s.GeneratingPolicies {
		out = append(out, engineapi.NewGeneratingPolicyFromLike(pol))
	}
	for i := range s.DeletingPolicies {
		out = append(out, engineapi.NewDeletingPolicyFromLike(s.DeletingPolicies[i]))
	}
	for i := range s.MutatingPolicies {
		out = append(out, engineapi.NewMutatingPolicyFromLike(s.MutatingPolicies[i]))
	}
	return out
}

// PolicyCount is the number of evaluable policies in the set (bindings and exceptions excluded).
func (s *PolicySet) PolicyCount() int {
	return len(s.VAPs) + len(s.ValidatingPolicies) + len(s.ImageValidatingPolicies) + len(s.MAPs) +
		len(s.GeneratingPolicies) + len(s.DeletingPolicies) + len(s.MutatingPolicies) +
		len(s.EnvoyPolicies) + len(s.HTTPPolicies)
}

// PolicyRef identifies one loaded policy and the payload modes it can be evaluated against.
type PolicyRef struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace,omitempty"`
	Modes      []Mode `json:"modes"`
}

type identified interface {
	GetName() string
	GetNamespace() string
}

func ref(obj identified, apiVersion, kind string, modes ...Mode) PolicyRef {
	return PolicyRef{APIVersion: apiVersion, Kind: kind, Name: obj.GetName(), Namespace: obj.GetNamespace(), Modes: modes}
}

func typeOf(obj any, fallbackAPIVersion, fallbackKind string) (string, string) {
	if t, ok := obj.(interface {
		GetAPIVersion() string
		GetKind() string
	}); ok && t.GetKind() != "" {
		return t.GetAPIVersion(), t.GetKind()
	}
	return fallbackAPIVersion, fallbackKind
}

// Policies lists the loaded policies sorted by kind, namespace and name.
func (s *PolicySet) Policies() []PolicyRef {
	refs := make([]PolicyRef, 0, s.PolicyCount())
	for i := range s.VAPs {
		refs = append(refs, ref(&s.VAPs[i], "admissionregistration.k8s.io/v1", "ValidatingAdmissionPolicy", ModeKubernetes))
	}
	for i := range s.MAPs {
		refs = append(refs, ref(&s.MAPs[i], "admissionregistration.k8s.io/v1beta1", "MutatingAdmissionPolicy", ModeKubernetes))
	}
	for _, p := range s.ValidatingPolicies {
		av, k := typeOf(p, "policies.kyverno.io/v1beta1", "ValidatingPolicy")
		mode := ModeKubernetes
		if p.GetValidatingPolicySpec().EvaluationMode() == policieskyvernoio.EvaluationModeJSON {
			mode = ModeJSON
		}
		refs = append(refs, ref(p, av, k, mode))
	}
	for _, p := range s.ImageValidatingPolicies {
		av, k := typeOf(p, "policies.kyverno.io/v1beta1", "ImageValidatingPolicy")
		refs = append(refs, ref(p, av, k, ModeKubernetes, ModeJSON))
	}
	for _, p := range s.MutatingPolicies {
		av, k := typeOf(p, "policies.kyverno.io/v1beta1", "MutatingPolicy")
		mode := ModeKubernetes
		if p.GetSpec().EvaluationMode() == policieskyvernoio.EvaluationModeJSON {
			mode = ModeJSON
		}
		refs = append(refs, ref(p, av, k, mode))
	}
	for _, p := range s.GeneratingPolicies {
		av, k := typeOf(p, "policies.kyverno.io/v1beta1", "GeneratingPolicy")
		refs = append(refs, ref(p, av, k, ModeKubernetes))
	}
	for _, p := range s.DeletingPolicies {
		av, k := typeOf(p, "policies.kyverno.io/v1beta1", "DeletingPolicy")
		refs = append(refs, ref(p, av, k, ModeKubernetes, ModeJSON))
	}
	for _, p := range s.HTTPPolicies {
		av, k := typeOf(p, "policies.kyverno.io/v1beta1", "ValidatingPolicy")
		refs = append(refs, ref(p, av, k, ModeHTTP))
	}
	for _, p := range s.EnvoyPolicies {
		av, k := typeOf(p, "policies.kyverno.io/v1beta1", "ValidatingPolicy")
		refs = append(refs, ref(p, av, k, ModeEnvoy))
	}
	sort.SliceStable(refs, func(i, j int) bool {
		if refs[i].Kind != refs[j].Kind {
			return refs[i].Kind < refs[j].Kind
		}
		if refs[i].Namespace != refs[j].Namespace {
			return refs[i].Namespace < refs[j].Namespace
		}
		return refs[i].Name < refs[j].Name
	})
	return refs
}

// Digest returns a stable sha256 over the loaded policies, bindings and exceptions, independent of
// file layout and load order, so a response can be tied to the exact policy set that produced it.
func (s *PolicySet) Digest() (string, error) {
	var docs []string
	add := func(kind string, obj any) error {
		b, err := json.Marshal(obj)
		if err != nil {
			return fmt.Errorf("failed to marshal %s for digest: %w", kind, err)
		}
		docs = append(docs, kind+"\x00"+string(b))
		return nil
	}
	var errs []error
	collect := func(kind string, objs ...any) {
		for _, o := range objs {
			if err := add(kind, o); err != nil {
				errs = append(errs, err)
			}
		}
	}
	for i := range s.VAPs {
		collect("vap", s.VAPs[i])
	}
	for i := range s.VAPBindings {
		collect("vapb", s.VAPBindings[i])
	}
	for i := range s.MAPs {
		collect("map", s.MAPs[i])
	}
	for i := range s.MAPBindings {
		collect("mapb", s.MAPBindings[i])
	}
	for _, p := range s.ValidatingPolicies {
		collect("vpol", p)
	}
	for _, p := range s.ImageValidatingPolicies {
		collect("ivpol", p)
	}
	for _, p := range s.MutatingPolicies {
		collect("mpol", p)
	}
	for _, p := range s.GeneratingPolicies {
		collect("gpol", p)
	}
	for _, p := range s.DeletingPolicies {
		collect("dpol", p)
	}
	for _, p := range s.HTTPPolicies {
		collect("http", p)
	}
	for _, p := range s.EnvoyPolicies {
		collect("envoy", p)
	}
	for _, p := range s.PolicyCelExceptions {
		collect("polex", p)
	}
	if len(errs) > 0 {
		return "", errs[0]
	}
	sort.Strings(docs)
	h := sha256.New()
	for _, d := range docs {
		h.Write([]byte(d))
		h.Write([]byte{'\n'})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// LoadStrict loads policies from local files or directories and fails on anything it cannot
// load: an unreadable path, a document that is not a supported policy, or a legacy kyverno.io
// kind. It is the loader for long-running callers (kyverno serve), where silently skipping a
// broken policy would turn into a silent allow for every later request.
func LoadStrict(paths ...string) (*PolicySet, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("at least one policy path is required")
	}
	set := &PolicySet{}
	for _, path := range paths {
		results, err := policy.Load(nil, "", path)
		if err != nil {
			if pkgdeprecations.IsLegacyPolicyBlockError(err) {
				return nil, err
			}
			return nil, fmt.Errorf("failed to load policies from %s: %w", path, err)
		}
		if results != nil && len(results.NonFatalErrors) > 0 {
			e := results.NonFatalErrors[0]
			return nil, fmt.Errorf("failed to load policies from %s: %s: %w", path, e.Path, e.Error)
		}
		set.Add(results)
	}
	if set.PolicyCount() == 0 {
		return nil, fmt.Errorf("no policies found in %v", paths)
	}
	return set, nil
}
