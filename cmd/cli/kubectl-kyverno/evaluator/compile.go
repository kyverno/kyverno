package evaluator

import (
	"fmt"

	authv3 "github.com/envoyproxy/go-control-plane/envoy/service/auth/v3"
	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	authzhttp "github.com/kyverno/kyverno-authz/pkg/cel/libs/authz/http"
	authzcompiler "github.com/kyverno/kyverno-authz/pkg/engine/compiler"
	dpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/dpol/compiler"
	gpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	eval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/dynamic"
)

// Compile compiles every Kyverno CEL policy in the set, with the set's exceptions, and returns the
// first failure. Loading only checks a policy's schema; an invalid CEL expression would otherwise
// surface on the first evaluation. VAPs and MAPs are compiled by the Kubernetes admission
// libraries at evaluation time and are not checked here.
func (s *PolicySet) Compile() error {
	exceptions := s.PolicyCelExceptions
	check := func(kind, name string, errs field.ErrorList) error {
		return fmt.Errorf("failed to compile %s %s: %w", kind, name, errs.ToAggregate())
	}
	vpols := vpolcompiler.NewCompiler()
	for _, p := range s.ValidatingPolicies {
		if _, errs := vpols.Compile(p, exceptions); len(errs) > 0 {
			return check(p.GetKind(), p.GetName(), errs)
		}
	}
	mpols := mpolcompiler.NewCompiler()
	for _, p := range s.MutatingPolicies {
		if _, errs := mpols.Compile(p, exceptions); len(errs) > 0 {
			return check(p.GetKind(), p.GetName(), errs)
		}
	}
	gpols := gpolcompiler.NewCompiler()
	for _, p := range s.GeneratingPolicies {
		if _, errs := gpols.Compile(p, exceptions); len(errs) > 0 {
			return check(p.GetKind(), p.GetName(), errs)
		}
	}
	dpols := dpolcompiler.NewCompiler()
	for _, p := range s.DeletingPolicies {
		if _, errs := dpols.Compile(p, exceptions); len(errs) > 0 {
			return check(p.GetKind(), p.GetName(), errs)
		}
	}
	ivpols := eval.NewCompiler(nil)
	for _, p := range s.ImageValidatingPolicies {
		if _, errs := ivpols.Compile(p, exceptions); len(errs) > 0 {
			return check(p.GetKind(), p.GetName(), errs)
		}
	}
	envoy := authzcompiler.NewCompiler[dynamic.Interface, *authv3.CheckRequest, *authv3.CheckResponse](nil)
	for _, p := range s.EnvoyPolicies {
		if _, errs := envoy.Compile(authzPolicy(p), nil); len(errs) > 0 {
			return check("Envoy ValidatingPolicy", p.GetName(), errs)
		}
	}
	httpPolicies := authzcompiler.NewCompiler[dynamic.Interface, *authzhttp.CheckRequest, *authzhttp.CheckResponse](nil)
	for _, p := range s.HTTPPolicies {
		if _, errs := httpPolicies.Compile(authzPolicy(p), nil); len(errs) > 0 {
			return check("HTTP ValidatingPolicy", p.GetName(), errs)
		}
	}
	return nil
}

// authzPolicy converts a policy the way the authz processor does before compiling it.
func authzPolicy(p *policiesv1beta1.ValidatingPolicy) *policiesv1.ValidatingPolicy {
	return &policiesv1.ValidatingPolicy{TypeMeta: p.TypeMeta, ObjectMeta: p.ObjectMeta, Spec: p.Spec}
}
