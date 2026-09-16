package exception

import (
	"fmt"
	"os"
	"path/filepath"

	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/kyverno/kyverno/ext/resource/convert"
	resourceloader "github.com/kyverno/kyverno/ext/resource/loader"
	yamlutils "github.com/kyverno/kyverno/ext/yaml"
	pkgdeprecations "github.com/kyverno/kyverno/pkg/deprecations"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kubectl-validate/pkg/openapiclient"
)

var (
	exceptionV2beta1     = schema.GroupVersion(kyvernov2beta1.GroupVersion).WithKind("PolicyException")
	exceptionV2          = schema.GroupVersion(kyvernov2.GroupVersion).WithKind("PolicyException")
	celExceptionV1alpha1 = schema.GroupVersion(policiesv1alpha1.GroupVersion).WithKind("PolicyException")
	celExceptionV1beta1  = schema.GroupVersion(policiesv1beta1.GroupVersion).WithKind("PolicyException")
	celExceptionV1       = schema.GroupVersion(policiesv1.GroupVersion).WithKind("PolicyException")
)

type LoaderResults struct {
	Exceptions    []*kyvernov2.PolicyException
	CELExceptions []*policiesv1beta1.PolicyException
	Warnings      []string
}

// Load loads policy exceptions from the given paths. When allowLegacyPolicies is false, a legacy
// kyverno.io PolicyException is rejected with a migration hint, matching the 1.20 admission-time block.
func Load(allowLegacyPolicies bool, paths ...string) (*LoaderResults, error) {
	loaderResults := &LoaderResults{}
	for _, path := range paths {
		bytes, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return nil, fmt.Errorf("unable to read yaml (%w)", err)
		}
		results, err := load(bytes, allowLegacyPolicies)
		if err != nil {
			return nil, fmt.Errorf("unable to load exceptions (%w)", err)
		}
		loaderResults.Exceptions = append(loaderResults.Exceptions, results.Exceptions...)
		loaderResults.CELExceptions = append(loaderResults.CELExceptions, results.CELExceptions...)
		for _, warning := range results.Warnings {
			loaderResults.Warnings = append(loaderResults.Warnings, fmt.Sprintf("%s: %s", path, warning))
		}
	}
	return loaderResults, nil
}

func load(content []byte, allowLegacyPolicies bool) (*LoaderResults, error) {
	results := &LoaderResults{}
	documents, err := yamlutils.SplitDocuments(content)
	if err != nil {
		return nil, err
	}
	crds, err := data.Crds()
	if err != nil {
		return nil, err
	}

	factory, err := resourceloader.New(openapiclient.NewComposite(openapiclient.NewLocalCRDFiles(crds)))
	if err != nil {
		return nil, err
	}

	// pendingErr holds the first "ordinary" (non-legacy-block) fatal error seen so far. Scanning
	// always continues past it so a legacy exception appearing later in the same multi-document
	// file still gets a chance to be blocked; a legacy-block error always takes priority and
	// returns immediately. If nothing later supersedes it, pendingErr is what the load ultimately
	// fails with, preserving the guarantee that a genuinely broken document fails the load rather
	// than being silently dropped.
	var pendingErr error
	for _, document := range documents {
		gvk, untyped, err := factory.Load(document)
		if err != nil {
			// The loader returns the parsed GVK alongside a schema-validation error, so a
			// malformed legacy exception must still be blocked with the migration hint rather
			// than surfacing only a generic validation error.
			if !allowLegacyPolicies {
				if blockErr, ok := pkgdeprecations.BuildKindError(gvk.Group, gvk.Version, gvk.Kind); ok {
					return nil, blockErr
				}
			}
			if pendingErr == nil {
				pendingErr = err
			}
			continue
		}
		switch gvk {
		case exceptionV2beta1, exceptionV2:
			if warning, ok := pkgdeprecations.BuildKindWarning(gvk.Group, gvk.Version, gvk.Kind); ok {
				results.Warnings = append(results.Warnings, warning.Message)
			}
			if !allowLegacyPolicies {
				if err, ok := pkgdeprecations.BuildKindError(gvk.Group, gvk.Version, gvk.Kind); ok {
					return nil, err
				}
			}
			exception, err := convert.To[kyvernov2.PolicyException](untyped)
			if err != nil {
				if pendingErr == nil {
					pendingErr = err
				}
				continue
			}
			results.Exceptions = append(results.Exceptions, exception)
		case celExceptionV1alpha1, celExceptionV1beta1, celExceptionV1:
			exception, err := convert.To[policiesv1beta1.PolicyException](untyped)
			if err != nil {
				if pendingErr == nil {
					pendingErr = err
				}
				continue
			}
			results.CELExceptions = append(results.CELExceptions, exception)
		default:
			if pendingErr == nil {
				pendingErr = fmt.Errorf("policy exception type not supported %s", gvk)
			}
		}
	}
	return results, pendingErr
}

// SelectFrom picks policy exceptions out of a slice of already-loaded resources (used for
// --exceptions-within-resources/--inline-exceptions). When allowLegacyPolicies is false, a legacy
// kyverno.io PolicyException among those resources is rejected with a migration hint, the same as
// Load does for exceptions loaded from --exception files.
func SelectFrom(resources []*unstructured.Unstructured, allowLegacyPolicies bool) (*LoaderResults, error) {
	results := &LoaderResults{}
	// pendingErr holds the first conversion error seen so far, matching load()'s behavior: keep
	// scanning the rest of the resources rather than aborting the whole batch on one malformed
	// exception, but still surface it in the end instead of silently dropping it.
	var pendingErr error
	for _, resource := range resources {
		gvk := resource.GroupVersionKind()
		switch gvk {
		case exceptionV2beta1, exceptionV2:
			if !allowLegacyPolicies {
				if err, ok := pkgdeprecations.BuildKindError(gvk.Group, gvk.Version, gvk.Kind); ok {
					return nil, err
				}
			}
			exception, err := convert.To[kyvernov2.PolicyException](*resource)
			if err != nil {
				if pendingErr == nil {
					pendingErr = err
				}
				continue
			}
			results.Exceptions = append(results.Exceptions, exception)
		case celExceptionV1alpha1, celExceptionV1beta1, celExceptionV1:
			celException, err := convert.To[policiesv1beta1.PolicyException](*resource)
			if err != nil {
				if pendingErr == nil {
					pendingErr = err
				}
				continue
			}
			results.CELExceptions = append(results.CELExceptions, celException)
		}
	}

	return results, pendingErr
}
