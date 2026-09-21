package push

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/oci/internal"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/policy"
	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	dpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/dpol/compiler"
	gpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	ivpolevaluator "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	"sigs.k8s.io/yaml"
)

const supportedAPIVersion = "policies.kyverno.io/v1beta1"

type options struct {
	imageRef string
}

func (o options) validate(dir string) error {
	if o.imageRef == "" {
		return errors.New("image is required")
	}
	if dir == "" {
		return errors.New("policy is required")
	}
	return nil
}

// toYAML serialises any JSON-marshalable value to YAML, honouring json struct tags.
func toYAML(v any) ([]byte, error) {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return yaml.JSONToYAML(jsonBytes)
}

// appendCELLayer serialises obj to YAML and appends it as an OCI layer to img.
func appendCELLayer(img v1.Image, obj internal.Object) (v1.Image, error) {
	b, err := toYAML(obj)
	if err != nil {
		return nil, fmt.Errorf("serialising %s %q: %w",
			obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err)
	}
	layer := static.NewLayer(b, internal.PolicyLayerMediaType)
	return mutate.Append(img, mutate.Addendum{
		Layer:       layer,
		Annotations: internal.Annotations(obj),
	})
}

// buildImage validates the loaded CEL resources (schema, duplicate identities,
// CEL expressions, and exception references) and constructs the OCI image.
// It rejects any non-CEL resources that leaked through the loader.
func buildImage(results *policy.LoaderResults) (v1.Image, error) {
	// Reject non-CEL kinds that policy.Load may populate even with allowLegacyPolicies=false.
	// A directory containing VAPs, MAPs, or legacy Policies must fail push explicitly.
	if len(results.Policies) > 0 {
		return nil, fmt.Errorf("push rejected: directory contains %d legacy kyverno.io policy resource(s); only policies.kyverno.io/v1beta1 CEL kinds are supported in OCI bundles", len(results.Policies))
	}
	if len(results.CleanupPolicies) > 0 {
		return nil, fmt.Errorf("push rejected: directory contains %d cleanup policy resource(s); only policies.kyverno.io/v1beta1 CEL kinds are supported in OCI bundles", len(results.CleanupPolicies))
	}
	if len(results.PolicyExceptions) > 0 {
		return nil, fmt.Errorf("push rejected: directory contains %d legacy kyverno.io PolicyException resource(s); use policies.kyverno.io/v1beta1 PolicyException instead", len(results.PolicyExceptions))
	}
	if len(results.VAPs) > 0 || len(results.VAPBindings) > 0 || len(results.MAPs) > 0 || len(results.MAPBindings) > 0 {
		return nil, fmt.Errorf("push rejected: directory contains native Kubernetes admission policy resources; only policies.kyverno.io/v1beta1 CEL kinds are supported in OCI bundles")
	}
	if len(results.NonFatalErrors) > 0 {
		return nil, fmt.Errorf("push rejected: %d file(s) could not be loaded: %v", len(results.NonFatalErrors), results.NonFatalErrors[0].Error)
	}

	seen := make(map[string]bool)
	checkResource := func(obj internal.Object) error {
		gvk := obj.GetObjectKind().GroupVersionKind()
		if gv := gvk.GroupVersion().String(); gv != supportedAPIVersion {
			return fmt.Errorf("unsupported resource %s/%s %q; only %s CEL kinds are supported in OCI bundles",
				gv, gvk.Kind, obj.GetName(), supportedAPIVersion)
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
			return nil, fmt.Errorf("ValidatingPolicy does not implement runtime.Object")
		}
		if err := checkResource(obj); err != nil {
			return nil, err
		}
		if _, errs := vCompiler.Compile(pol, nil); len(errs) > 0 {
			return nil, fmt.Errorf("validating CEL expression in %s %q: %v", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), errs.ToAggregate())
		}
	}
	for _, pol := range results.EnvoyPolicies {
		if err := checkResource(pol); err != nil {
			return nil, err
		}
		if _, errs := vCompiler.Compile(pol, nil); len(errs) > 0 {
			return nil, fmt.Errorf("validating CEL expression in EnvoyPolicy %q: %v", pol.GetName(), errs.ToAggregate())
		}
	}
	for _, pol := range results.HTTPPolicies {
		if err := checkResource(pol); err != nil {
			return nil, err
		}
		if _, errs := vCompiler.Compile(pol, nil); len(errs) > 0 {
			return nil, fmt.Errorf("validating CEL expression in HTTPPolicy %q: %v", pol.GetName(), errs.ToAggregate())
		}
	}

	mCompiler := mpolcompiler.NewCompiler()
	for _, pol := range results.MutatingPolicies {
		obj, ok := pol.(internal.Object)
		if !ok {
			return nil, fmt.Errorf("MutatingPolicy does not implement runtime.Object")
		}
		if err := checkResource(obj); err != nil {
			return nil, err
		}
		if _, errs := mCompiler.Compile(pol, nil); len(errs) > 0 {
			return nil, fmt.Errorf("validating CEL expression in %s %q: %v", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), errs.ToAggregate())
		}
	}

	gCompiler := gpolcompiler.NewCompiler()
	for _, pol := range results.GeneratingPolicies {
		obj, ok := pol.(internal.Object)
		if !ok {
			return nil, fmt.Errorf("GeneratingPolicy does not implement runtime.Object")
		}
		if err := checkResource(obj); err != nil {
			return nil, err
		}
		if _, errs := gCompiler.Compile(pol, nil); len(errs) > 0 {
			return nil, fmt.Errorf("validating CEL expression in %s %q: %v", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), errs.ToAggregate())
		}
	}

	dCompiler := dpolcompiler.NewCompiler()
	for _, pol := range results.DeletingPolicies {
		obj, ok := pol.(internal.Object)
		if !ok {
			return nil, fmt.Errorf("DeletingPolicy does not implement runtime.Object")
		}
		if err := checkResource(obj); err != nil {
			return nil, err
		}
		if _, errs := dCompiler.Compile(pol, nil); len(errs) > 0 {
			return nil, fmt.Errorf("validating CEL expression in %s %q: %v", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), errs.ToAggregate())
		}
	}

	// ImageValidatingPolicy: compile CEL expressions via the ivpol evaluator.
	// NewCompiler(nil) is safe for offline pre-push validation (no cluster secret lister needed).
	ivpCompiler := ivpolevaluator.NewCompiler(nil)
	for _, pol := range results.ImageValidatingPolicies {
		obj, ok := pol.(internal.Object)
		if !ok {
			return nil, fmt.Errorf("ImageValidatingPolicy does not implement runtime.Object")
		}
		if err := checkResource(obj); err != nil {
			return nil, err
		}
		if _, errs := ivpCompiler.Compile(pol, nil); len(errs) > 0 {
			return nil, fmt.Errorf("validating CEL expression in %s %q: %v", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), errs.ToAggregate())
		}
	}

	// Validate PolicyExceptions unconditionally — the hasPolicies gate was removed because
	// an exception-only bundle that references a policy not present in the bundle is
	// always a packaging error that must be caught before publication.
	for _, ex := range results.PolicyCelExceptions {
		if err := checkResource(ex); err != nil {
			return nil, err
		}
		if errs := ex.Validate(); len(errs) > 0 {
			return nil, fmt.Errorf("validating policy exception %q: %v", ex.GetName(), errs.ToAggregate())
		}
		if errs := celcompiler.CompilePolicyExceptionMatchConditions(ex.Spec.MatchConditions, nil); len(errs) > 0 {
			return nil, fmt.Errorf("validating CEL expression in policy exception %q: %v", ex.GetName(), errs.ToAggregate())
		}
		for _, ref := range ex.Spec.PolicyRefs {
			refKey := fmt.Sprintf("%s/%s/%s", ref.Kind, "", ref.Name)
			namespacedKey := fmt.Sprintf("%s/%s/%s", ref.Kind, ex.GetNamespace(), ref.Name)
			if !seen[refKey] && !seen[namespacedKey] {
				return nil, fmt.Errorf("policy exception %q references unknown policy %s/%s", ex.GetName(), ref.Kind, ref.Name)
			}
		}
	}

	img := mutate.MediaType(empty.Image, types.OCIManifestSchema1)
	img = mutate.ConfigMediaType(img, internal.PolicyConfigMediaType)

	appendAll := func(list []internal.Object) error {
		var err error
		for _, obj := range list {
			fmt.Fprintf(os.Stderr, "Adding %s [%s]\n", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
			if img, err = appendCELLayer(img, obj); err != nil {
				return err
			}
		}
		return nil
	}

	for _, pol := range results.ValidatingPolicies {
		if err := appendAll([]internal.Object{pol.(internal.Object)}); err != nil {
			return nil, err
		}
	}
	for _, pol := range results.EnvoyPolicies {
		if err := appendAll([]internal.Object{pol}); err != nil {
			return nil, err
		}
	}
	for _, pol := range results.HTTPPolicies {
		if err := appendAll([]internal.Object{pol}); err != nil {
			return nil, err
		}
	}
	for _, pol := range results.MutatingPolicies {
		if err := appendAll([]internal.Object{pol.(internal.Object)}); err != nil {
			return nil, err
		}
	}
	for _, pol := range results.GeneratingPolicies {
		if err := appendAll([]internal.Object{pol.(internal.Object)}); err != nil {
			return nil, err
		}
	}
	for _, pol := range results.DeletingPolicies {
		if err := appendAll([]internal.Object{pol.(internal.Object)}); err != nil {
			return nil, err
		}
	}
	for _, pol := range results.ImageValidatingPolicies {
		if err := appendAll([]internal.Object{pol.(internal.Object)}); err != nil {
			return nil, err
		}
	}
	for _, ex := range results.PolicyCelExceptions {
		if err := appendAll([]internal.Object{ex}); err != nil {
			return nil, err
		}
	}

	return img, nil
}

func (o options) execute(ctx context.Context, dir string, keychain authn.Keychain) error {
	// allowLegacyPolicies=false: kyverno.io/v1 Policy/ClusterPolicy and legacy
	// cleanup/exception kinds are rejected with actionable migration errors.
	results, err := policy.Load(nil, "", false, dir)
	if err != nil {
		return fmt.Errorf("loading policies from %s: %w", dir, err)
	}

	ref, err := name.ParseReference(o.imageRef)
	if err != nil {
		return fmt.Errorf("parsing image reference: %w", err)
	}

	img, err := buildImage(results)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Uploading [%s]...\n", ref.Name())
	if err = remote.Write(ref, img, remote.WithContext(ctx), remote.WithAuthFromKeychain(keychain)); err != nil {
		return fmt.Errorf("writing image: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Done.")
	return nil
}
