package bundle

import (
	"fmt"
	"strings"
	"time"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	dpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/dpol/compiler"
	gpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	ivpolevaluator "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	exceptionvalidation "github.com/kyverno/kyverno/pkg/validation/exception"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// nativeAdmissionKinds are the admissionregistration.k8s.io kinds validateResults rejects with
// its own specific "native Kubernetes admission policy resources" message (a pure move of
// buildImage's check). validateDocuments must not shadow that message with its own generic
// "unsupported resource" one, so it recognizes and steps over these kinds, deferring to
// validateResults.
var nativeAdmissionKinds = map[string]bool{
	"ValidatingAdmissionPolicy":        true,
	"ValidatingAdmissionPolicyBinding": true,
	"MutatingAdmissionPolicy":          true,
	"MutatingAdmissionPolicyBinding":   true,
}

func isNativeAdmissionDocument(doc Document) bool {
	group := doc.APIVersion
	if i := strings.IndexByte(group, '/'); i >= 0 {
		group = group[:i]
	}
	return group == "admissionregistration.k8s.io" && nativeAdmissionKinds[doc.Kind]
}

// Validate is the format's validation entry point for a writer (push). The two checks it runs,
// validateDocuments and validateResults, are each an individually unchanged, pure move: the
// former is pull/options.go's extractAndSavePolicies per-document logic, unmodified; the latter
// is push/options.go's buildImage logic, unmodified, including push's own strict default of
// resolving every PolicyException's policy references within the bundle (bundle-spec.md section
// 6, "Sets": "kyverno oci push enforces in-bundle resolution as its own strict default"; writer
// MUST 12). What changed in this move is composition, not the checks themselves: both check sets
// now run on both the push and read paths (Read calls ValidateForRead, below), where
// historically each ran on only one. That composition has two visible, harmless consequences,
// worth stating rather than leaving for a reader of the diff to rediscover:
//
//   - Which message surfaces for a given bad input can differ from history. validateDocuments
//     runs first, so its generic "unsupported resource" message can preempt a more specific one
//     validateResults would have given lower in the function for the same input (see
//     isNativeAdmissionDocument below for the one case this package corrects, VAP/MAP). A
//     "resource missing kind or metadata.name" from validateDocuments can likewise preempt what
//     would otherwise be an "unsupported resource" from the same function, for an input (such as
//     a kustomization.yaml) that has a kind but no metadata.name.
//   - On push, Assemble's own per-file policy.Load call already hard-rejects every legacy
//     kyverno.io kind and kyverno.io/v2 PolicyException before Validate ever runs (that block
//     predates this change), which makes validateDocuments' internal.LegacyKinds branch and
//     validateResults' three legacy-resource branches unreachable via the real Assemble-driven
//     pipeline. They are kept, not deleted, because Validate is also called directly against a
//     hand-built Bundle (this package's own tests do this, bypassing Assemble entirely), and for
//     that caller these branches are the only thing that rejects a legacy kind.
//
// None of the above is a validation bypass: every input that was rejected before this change is
// still rejected after it, by the same package, just not always by the same one of the two
// checks. #17664 extends this with the accepted-version table, kyvernoVersion constraint
// checking (see descriptor.go), writer-policy selector/expiry/compensating-control checks, and
// widening the single-version pin to AcceptedAPIVersions; this move doesn't add or remove a
// check.
func Validate(b *Bundle) error {
	return validate(b, true, Options{})
}

// Options configures the writer-policy checks ValidateWithOptions runs in addition to the
// format checks Validate always runs. The zero value is Validate's own behavior: Now defaults to
// time.Now and Warn discards warnings.
type Options struct {
	// Now returns the time expiry is evaluated against. Nil means time.Now. Deliberately not
	// SOURCE_DATE_EPOCH: expiry is about the reader's clock, not the bundle's reproducibility.
	Now func() time.Time
	// Warn receives a warning message for content that is accepted but suspect (an expired
	// exception, a compensating control naming a kind the engine ignores). Nil discards warnings.
	Warn func(string)
}

func (o Options) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

func (o Options) warn(msg string) {
	if o.Warn != nil {
		o.Warn(msg)
	}
}

// ValidateWithOptions is Validate plus the writer-policy checks: offline selector well-formedness,
// exception expiry (warning only; promoting it to a failure is validate --strict's job, not this
// package's), and PolicyException compensating-control checks. It is for push and
// `kyverno oci validate`, never for the read path (see ValidateForRead).
func ValidateWithOptions(b *Bundle, opts Options) error {
	return validate(b, true, opts)
}

// ValidateForRead is Read's validation entry point: the same format checks as Validate, minus
// exception-to-policy reference resolution and the writer-policy checks ValidateWithOptions adds.
// The spec is explicit that in-bundle resolution is push's own strict default, not a format
// requirement (bundle-spec.md section 6, "Sets"; writer MUST 12), and reader MUST 3 lists only
// the raw-document rejection, the kind and version checks, the identity check, and CEL
// compilation as what a reader re-runs — not reference resolution, selectors, expiry, or
// compensating controls. A bundle whose PolicyException references a policy outside the bundle,
// or whose exception has expired, is format-legal and must round-trip through Read.
func ValidateForRead(b *Bundle) error {
	return validate(b, false, Options{})
}

func validate(b *Bundle, strict bool, opts Options) error {
	if err := validateDocuments(b.Documents); err != nil {
		return err
	}
	return validateResults(b.Results, strict, opts)
}

// validateDocuments is an unchanged, pure move of pull/options.go's extractAndSavePolicies
// per-document checks (kind/name presence, legacy-kind rejection, accepted-version and kind
// rejection, duplicate identity), now running over every document in the bundle instead of one
// layer at a time, and now also running on the push path (see Validate's docstring for what that
// composition changes). Native admissionregistration.k8s.io documents are stepped over here (see
// isNativeAdmissionDocument) so validateResults's more specific rejection message is the one
// that surfaces, matching buildImage's original behavior for those kinds.
func validateDocuments(documents []Document) error {
	seen := make(map[string]bool)
	for _, doc := range documents {
		if strings.TrimSpace(doc.Kind) == "" || strings.TrimSpace(doc.Name) == "" {
			return fmt.Errorf("resource missing kind or metadata.name")
		}
		if internal.LegacyKinds[doc.Kind] {
			return fmt.Errorf("legacy policy kind %q (apiVersion: %s) is no longer supported in OCI bundles; migrate to %s CEL policy kinds", doc.Kind, doc.APIVersion, internal.AcceptedAPIVersionsText())
		}
		if isNativeAdmissionDocument(doc) {
			continue
		}
		if !internal.IsAcceptedAPIVersion(doc.APIVersion) || !internal.SupportedCELKinds[doc.Kind] {
			return fmt.Errorf("unsupported resource %s/%s %q; only %s CEL policy kinds are supported in OCI bundles", doc.APIVersion, doc.Kind, doc.Name, internal.AcceptedAPIVersionsText())
		}

		identity := fmt.Sprintf("%s/%s/%s", doc.Kind, doc.Namespace, doc.Name)
		if seen[identity] {
			return fmt.Errorf("duplicate resource identity %s", identity)
		}
		seen[identity] = true
	}
	return nil
}

// checkSelectors reports an error if mc's NamespaceSelector or ObjectSelector is structurally
// malformed, via the same offline check pkg/cel/matching/criteria.go uses live. A selector that is
// well-formed but matches nothing is not checkable offline and is out of scope (bundle-spec.md
// section 9). mc may be nil (DeletingPolicySpec.MatchConstraints is a nilable pointer).
func checkSelectors(kind, name string, mc *admissionregistrationv1.MatchResources) error {
	if mc == nil {
		return nil
	}
	if mc.NamespaceSelector != nil {
		if _, err := metav1.LabelSelectorAsSelector(mc.NamespaceSelector); err != nil {
			return fmt.Errorf("validating %s %q: malformed namespaceSelector: %v", kind, name, err)
		}
	}
	if mc.ObjectSelector != nil {
		if _, err := metav1.LabelSelectorAsSelector(mc.ObjectSelector); err != nil {
			return fmt.Errorf("validating %s %q: malformed objectSelector: %v", kind, name, err)
		}
	}
	return nil
}

// validateResults is an unchanged, pure move of push/options.go's buildImage checks (rejecting
// legacy and native-Kubernetes resources, non-fatal loader errors, unsupported versions,
// duplicate identity, CEL compilation, and exception-to-policy reference resolution); buildImage
// itself is replaced by the Assemble/Validate/Write split. Only the error messages' prefix
// changed, from "push rejected:" to the path-agnostic "bundle rejected:", because this function
// now also runs on the read path (see Validate's docstring): a message that says "push" while
// executing inside `kyverno oci pull` would be actively misleading. strict gates the
// exception-to-policy reference resolution loop at the end plus the writer-policy checks this
// issue adds (selectors, expiry, compensating controls): true for push (and ValidateWithOptions),
// false for Read (see ValidateForRead).
func validateResults(results *policy.LoaderResults, strict bool, opts Options) error {
	if results == nil {
		results = &policy.LoaderResults{}
	}
	// Legacy kyverno.io policy/cleanup-policy/exception kinds are already rejected by policy.Load
	// itself (migration-hint error) before results are ever returned, so no further check is needed here.
	if len(results.VAPs) > 0 || len(results.VAPBindings) > 0 || len(results.MAPs) > 0 || len(results.MAPBindings) > 0 {
		return fmt.Errorf("bundle rejected: directory contains native Kubernetes admission policy resources; only %s CEL kinds are supported in OCI bundles", internal.AcceptedAPIVersionsText())
	}
	if len(results.NonFatalErrors) > 0 {
		return fmt.Errorf("bundle rejected: %d file(s) could not be loaded: %v", len(results.NonFatalErrors), results.NonFatalErrors[0].Error)
	}

	seen := make(map[string]bool)
	checkResource := func(obj internal.Object) error {
		gvk := obj.GetObjectKind().GroupVersionKind()
		if gv := gvk.GroupVersion().String(); !internal.IsAcceptedAPIVersion(gv) {
			return fmt.Errorf("unsupported resource %s/%s %q; only %s CEL kinds are supported in OCI bundles",
				gv, gvk.Kind, obj.GetName(), internal.AcceptedAPIVersionsText())
		}
		key := fmt.Sprintf("%s/%s/%s", gvk.Kind, obj.GetNamespace(), obj.GetName())
		if seen[key] {
			return fmt.Errorf("duplicate resource identity %s", key)
		}
		seen[key] = true
		return nil
	}

	vCompiler := vpolcompiler.NewCompiler()
	for _, pol := range results.ValidatingPolicies {
		obj, ok := pol.(internal.Object)
		if !ok {
			return fmt.Errorf("ValidatingPolicy does not implement runtime.Object")
		}
		if err := checkResource(obj); err != nil {
			return err
		}
		if strict {
			mc := pol.GetMatchConstraints()
			if err := checkSelectors("ValidatingPolicy", obj.GetName(), &mc); err != nil {
				return err
			}
		}
		if _, errs := vCompiler.Compile(pol, nil); len(errs) > 0 {
			return fmt.Errorf("validating CEL expression in %s %q: %v", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), errs.ToAggregate())
		}
	}
	for _, pol := range results.EnvoyPolicies {
		if err := checkResource(pol); err != nil {
			return err
		}
		if strict {
			mc := pol.GetMatchConstraints()
			if err := checkSelectors("EnvoyPolicy", pol.GetName(), &mc); err != nil {
				return err
			}
		}
		if _, errs := vCompiler.Compile(pol, nil); len(errs) > 0 {
			return fmt.Errorf("validating CEL expression in EnvoyPolicy %q: %v", pol.GetName(), errs.ToAggregate())
		}
	}
	for _, pol := range results.HTTPPolicies {
		if err := checkResource(pol); err != nil {
			return err
		}
		if strict {
			mc := pol.GetMatchConstraints()
			if err := checkSelectors("HTTPPolicy", pol.GetName(), &mc); err != nil {
				return err
			}
		}
		if _, errs := vCompiler.Compile(pol, nil); len(errs) > 0 {
			return fmt.Errorf("validating CEL expression in HTTPPolicy %q: %v", pol.GetName(), errs.ToAggregate())
		}
	}

	mCompiler := mpolcompiler.NewCompiler()
	for _, pol := range results.MutatingPolicies {
		obj, ok := pol.(internal.Object)
		if !ok {
			return fmt.Errorf("MutatingPolicy does not implement runtime.Object")
		}
		if err := checkResource(obj); err != nil {
			return err
		}
		if strict {
			mc := pol.GetMatchConstraints()
			if err := checkSelectors(obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), &mc); err != nil {
				return err
			}
		}
		if _, errs := mCompiler.Compile(pol, nil); len(errs) > 0 {
			return fmt.Errorf("validating CEL expression in %s %q: %v", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), errs.ToAggregate())
		}
	}

	gCompiler := gpolcompiler.NewCompiler()
	for _, pol := range results.GeneratingPolicies {
		obj, ok := pol.(internal.Object)
		if !ok {
			return fmt.Errorf("GeneratingPolicy does not implement runtime.Object")
		}
		if err := checkResource(obj); err != nil {
			return err
		}
		if strict {
			mc := pol.GetMatchConstraints()
			if err := checkSelectors(obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), &mc); err != nil {
				return err
			}
		}
		if _, errs := gCompiler.Compile(pol, nil); len(errs) > 0 {
			return fmt.Errorf("validating CEL expression in %s %q: %v", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), errs.ToAggregate())
		}
	}

	dCompiler := dpolcompiler.NewCompiler()
	for _, pol := range results.DeletingPolicies {
		obj, ok := pol.(internal.Object)
		if !ok {
			return fmt.Errorf("DeletingPolicy does not implement runtime.Object")
		}
		if err := checkResource(obj); err != nil {
			return err
		}
		if strict {
			if spec := pol.GetDeletingPolicySpec(); spec != nil {
				if err := checkSelectors(obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), spec.MatchConstraints); err != nil {
					return err
				}
			}
		}
		if _, errs := dCompiler.Compile(pol, nil); len(errs) > 0 {
			return fmt.Errorf("validating CEL expression in %s %q: %v", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), errs.ToAggregate())
		}
	}

	ivpCompiler := ivpolevaluator.NewCompiler(nil)
	for _, pol := range results.ImageValidatingPolicies {
		obj, ok := pol.(internal.Object)
		if !ok {
			return fmt.Errorf("ImageValidatingPolicy does not implement runtime.Object")
		}
		if err := checkResource(obj); err != nil {
			return err
		}
		if strict {
			mc := pol.GetMatchConstraints()
			if err := checkSelectors(obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), &mc); err != nil {
				return err
			}
		}
		if _, errs := ivpCompiler.Compile(pol, nil); len(errs) > 0 {
			return fmt.Errorf("validating CEL expression in %s %q: %v", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), errs.ToAggregate())
		}
	}

	for _, ex := range results.PolicyCelExceptions {
		if err := checkResource(ex); err != nil {
			return err
		}
		if errs := ex.Validate(); len(errs) > 0 {
			return fmt.Errorf("validating policy exception %q: %v", ex.GetName(), errs.ToAggregate())
		}
		if errs := celcompiler.CompilePolicyExceptionMatchConditions(ex.Spec.MatchConditions, nil); len(errs) > 0 {
			return fmt.Errorf("validating CEL expression in policy exception %q: %v", ex.GetName(), errs.ToAggregate())
		}
		if !strict {
			continue
		}
		if ex.Spec.ExpiresAt != nil && opts.now().After(ex.Spec.ExpiresAt.Time) {
			opts.warn(fmt.Sprintf("policy exception %q expired at %s", ex.GetName(), ex.Spec.ExpiresAt.Time))
		}
		if errs := exceptionvalidation.ValidateCompensatingControlExpressions(ex); len(errs) > 0 {
			return fmt.Errorf("validating compensating controls in policy exception %q: %v", ex.GetName(), errs.ToAggregate())
		}
		for _, warning := range exceptionvalidation.ValidateCompensatingControls(&ex.Spec) {
			opts.warn(fmt.Sprintf("policy exception %q: %s", ex.GetName(), warning))
		}
		for _, ref := range ex.Spec.PolicyRefs {
			// PolicyRef has no namespace field; check cluster-scoped and same-namespace forms.
			clusterKey := fmt.Sprintf("%s/%s/%s", ref.Kind, "", ref.Name)
			namespacedKey := fmt.Sprintf("%s/%s/%s", ref.Kind, ex.GetNamespace(), ref.Name)
			if !seen[clusterKey] && !seen[namespacedKey] {
				return fmt.Errorf("policy exception %q references unknown policy %s/%s", ex.GetName(), ref.Kind, ref.Name)
			}
		}
	}

	return nil
}
