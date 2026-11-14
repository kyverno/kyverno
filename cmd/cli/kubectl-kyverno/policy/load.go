package policy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-git/go-billy/v5"
	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	policiesv1alpha1 "github.com/kyverno/api/api/policies.kyverno.io/v1alpha1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/exception"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/source"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils"
	"github.com/kyverno/kyverno/ext/resource/convert"
	resourceloader "github.com/kyverno/kyverno/ext/resource/loader"
	extyaml "github.com/kyverno/kyverno/ext/yaml"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	pkgdeprecations "github.com/kyverno/kyverno/pkg/deprecations"
	"github.com/kyverno/kyverno/pkg/utils/git"
	"github.com/pkg/errors"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/kubectl-validate/pkg/openapiclient"
	"sigs.k8s.io/yaml"
)

var (
	vapV1              = admissionregistrationv1.SchemeGroupVersion.WithKind("ValidatingAdmissionPolicy")
	vapBindingV1       = admissionregistrationv1.SchemeGroupVersion.WithKind("ValidatingAdmissionPolicyBinding")
	vpV1alpha1         = schema.GroupVersion(policiesv1alpha1.GroupVersion).WithKind("ValidatingPolicy")
	vpV1beta1          = schema.GroupVersion(policiesv1beta1.GroupVersion).WithKind("ValidatingPolicy")
	vpV1               = schema.GroupVersion(policiesv1.GroupVersion).WithKind("ValidatingPolicy")
	nvpV1beta1         = schema.GroupVersion(policiesv1beta1.GroupVersion).WithKind("NamespacedValidatingPolicy")
	nvpV1              = schema.GroupVersion(policiesv1.GroupVersion).WithKind("NamespacedValidatingPolicy")
	ivpV1alpha1        = schema.GroupVersion(policiesv1alpha1.GroupVersion).WithKind("ImageValidatingPolicy")
	ivpV1beta1         = schema.GroupVersion(policiesv1beta1.GroupVersion).WithKind("ImageValidatingPolicy")
	ivpV1              = schema.GroupVersion(policiesv1.GroupVersion).WithKind("ImageValidatingPolicy")
	nivpV1beta1        = schema.GroupVersion(policiesv1beta1.GroupVersion).WithKind("NamespacedImageValidatingPolicy")
	nivpV1             = schema.GroupVersion(policiesv1.GroupVersion).WithKind("NamespacedImageValidatingPolicy")
	gpsV1alpha1        = schema.GroupVersion(policiesv1alpha1.GroupVersion).WithKind("GeneratingPolicy")
	gpsV1beta1         = schema.GroupVersion(policiesv1beta1.GroupVersion).WithKind("GeneratingPolicy")
	gpsV1              = schema.GroupVersion(policiesv1.GroupVersion).WithKind("GeneratingPolicy")
	ngpsV1beta1        = schema.GroupVersion(policiesv1beta1.GroupVersion).WithKind("NamespacedGeneratingPolicy")
	ngpsV1             = schema.GroupVersion(policiesv1.GroupVersion).WithKind("NamespacedGeneratingPolicy")
	dpV1alpha1         = schema.GroupVersion(policiesv1alpha1.GroupVersion).WithKind("DeletingPolicy")
	dpV1beta1          = schema.GroupVersion(policiesv1beta1.GroupVersion).WithKind("DeletingPolicy")
	dpV1               = schema.GroupVersion(policiesv1.GroupVersion).WithKind("DeletingPolicy")
	ndpV1beta1         = schema.GroupVersion(policiesv1beta1.GroupVersion).WithKind("NamespacedDeletingPolicy")
	ndpV1              = schema.GroupVersion(policiesv1.GroupVersion).WithKind("NamespacedDeletingPolicy")
	mpV1alpha1         = schema.GroupVersion(policiesv1alpha1.GroupVersion).WithKind("MutatingPolicy")
	mpV1beta1          = schema.GroupVersion(policiesv1beta1.GroupVersion).WithKind("MutatingPolicy")
	mpV1               = schema.GroupVersion(policiesv1.GroupVersion).WithKind("MutatingPolicy")
	nmpV1beta1         = schema.GroupVersion(policiesv1beta1.GroupVersion).WithKind("NamespacedMutatingPolicy")
	nmpV1              = schema.GroupVersion(policiesv1.GroupVersion).WithKind("NamespacedMutatingPolicy")
	mapV1              = admissionregistrationv1.SchemeGroupVersion.WithKind("MutatingAdmissionPolicy")
	mapV1alpha1        = admissionregistrationv1alpha1.SchemeGroupVersion.WithKind("MutatingAdmissionPolicy")
	mapV1beta1         = admissionregistrationv1beta1.SchemeGroupVersion.WithKind("MutatingAdmissionPolicy")
	mapBindingV1       = admissionregistrationv1.SchemeGroupVersion.WithKind("MutatingAdmissionPolicyBinding")
	mapBindingV1alpha1 = admissionregistrationv1alpha1.SchemeGroupVersion.WithKind("MutatingAdmissionPolicyBinding")
	mapBindingV1beta1  = admissionregistrationv1beta1.SchemeGroupVersion.WithKind("MutatingAdmissionPolicyBinding")
)

type LoaderError struct {
	Path  string
	Error error
}

type LoaderWarning struct {
	Path    string
	Warning string
}

type LoaderResults struct {
	VAPs                    []admissionregistrationv1.ValidatingAdmissionPolicy
	VAPBindings             []admissionregistrationv1.ValidatingAdmissionPolicyBinding
	MAPs                    []admissionregistrationv1beta1.MutatingAdmissionPolicy
	MAPBindings             []admissionregistrationv1beta1.MutatingAdmissionPolicyBinding
	ValidatingPolicies      []policiesv1beta1.ValidatingPolicyLike
	EnvoyPolicies           []*policiesv1beta1.ValidatingPolicy
	HTTPPolicies            []*policiesv1beta1.ValidatingPolicy
	ImageValidatingPolicies []policiesv1beta1.ImageValidatingPolicyLike
	GeneratingPolicies      []policiesv1beta1.GeneratingPolicyLike
	DeletingPolicies        []policiesv1beta1.DeletingPolicyLike
	MutatingPolicies        []policiesv1beta1.MutatingPolicyLike
	PolicyCelExceptions     []*policiesv1beta1.PolicyException
	NonFatalErrors          []LoaderError
	Warnings                []LoaderWarning
}

func (l *LoaderResults) Merge(results *LoaderResults) {
	if results == nil {
		return
	}
	l.VAPs = append(l.VAPs, results.VAPs...)
	l.VAPBindings = append(l.VAPBindings, results.VAPBindings...)
	l.ValidatingPolicies = append(l.ValidatingPolicies, results.ValidatingPolicies...)
	l.EnvoyPolicies = append(l.EnvoyPolicies, results.EnvoyPolicies...)
	l.HTTPPolicies = append(l.HTTPPolicies, results.HTTPPolicies...)
	l.MAPs = append(l.MAPs, results.MAPs...)
	l.MAPBindings = append(l.MAPBindings, results.MAPBindings...)
	l.ImageValidatingPolicies = append(l.ImageValidatingPolicies, results.ImageValidatingPolicies...)
	l.GeneratingPolicies = append(l.GeneratingPolicies, results.GeneratingPolicies...)
	l.NonFatalErrors = append(l.NonFatalErrors, results.NonFatalErrors...)
	l.Warnings = append(l.Warnings, results.Warnings...)
	l.DeletingPolicies = append(l.DeletingPolicies, results.DeletingPolicies...)
	l.PolicyCelExceptions = append(l.PolicyCelExceptions, results.PolicyCelExceptions...)
	l.MutatingPolicies = append(l.MutatingPolicies, results.MutatingPolicies...)
}

func (l *LoaderResults) addError(path string, err error) {
	l.NonFatalErrors = append(l.NonFatalErrors, LoaderError{
		Path:  path,
		Error: err,
	})
}

// addWarning records a warning, such as a deprecation notice, for the document at path.
func (l *LoaderResults) addWarning(path, warning string) {
	l.Warnings = append(l.Warnings, LoaderWarning{
		Path:    path,
		Warning: warning,
	})
}

type loader = func(string, []byte) (*LoaderResults, error)

// splitDocuments splits a file into YAML documents. It is a variable so tests can simulate a read
// error part way through a file.
var splitDocuments = extyaml.SplitDocuments

// ErrValidatorSetup is returned when the schema validator used by the loader cannot be built. It is
// not caused by any input file, so callers that skip invalid files must still treat it as fatal.
var ErrValidatorSetup = errors.New("failed to set up the policy schema validator")

// Load loads policies from the given paths. Any legacy kyverno.io policy kind (ClusterPolicy, Policy,
// CleanupPolicy, ClusterCleanupPolicy, PolicyException) is rejected with a migration hint, matching the
// 1.20 admission-time block.
func Load(fs billy.Filesystem, resourcePath string, paths ...string) (*LoaderResults, error) {
	return LoadWithLoader(nil, fs, resourcePath, paths...)
}

// LoadSkippingInvalidFiles behaves like Load, except that a file which fails to load does not
// abort the whole load. The failure is recorded in NonFatalErrors under that file's own path while
// any valid documents from the same file are kept, so a single broken policy no longer hides the
// valid policies around it, and the same goes for directory entries that cannot be read. Documents outside the
// policy API groups (resources, test manifests, values files and so on) are ignored rather than
// reported, since they are not policies to begin with. Legacy policy block errors and failures to
// set up the schema validator (ErrValidatorSetup) still abort the load, since they are not caused by a
// single file; they are the only errors this function returns.
func LoadSkippingInvalidFiles(fs billy.Filesystem, resourcePath string, paths ...string) (*LoaderResults, error) {
	if _, err := loaderDelegate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrValidatorSetup, err)
	}
	return loadWithLoader(skipInvalidFiles(validateLoader(true)), fs, resourcePath, true, paths...)
}

// skipInvalidFiles wraps a loader so that an ordinary error is recorded as a non fatal error for the
// file, keeping any partial results, while legacy policy block errors are still returned.
func skipInvalidFiles(l loader) loader {
	return func(path string, content []byte) (*LoaderResults, error) {
		results, err := l(path, content)
		if err == nil || pkgdeprecations.IsLegacyPolicyBlockError(err) {
			return results, err
		}
		// keep whatever the file did load (valid documents, warnings, document errors) and report
		// the failure next to it instead of dropping the whole file
		if results == nil {
			results = &LoaderResults{}
		}
		results.addError(path, err)
		return results, nil
	}
}

// LoadWithLoader loads policies from paths with the given loader, or the default validating loader
// when l is nil. The first path that fails to load fails the whole call.
func LoadWithLoader(l loader, fs billy.Filesystem, resourcePath string, paths ...string) (*LoaderResults, error) {
	if l == nil {
		l = kubectlValidateLoader()
	}
	return loadWithLoader(l, fs, resourcePath, false, paths...)
}

// loadWithLoader reads every path from stdin, git, http or the local filesystem and merges the
// results. With skipUnreadable set, a path that cannot be read is recorded under its own name instead
// of failing the call.
func loadWithLoader(l loader, fs billy.Filesystem, resourcePath string, skipUnreadable bool, paths ...string) (*LoaderResults, error) {
	aggregateResults := &LoaderResults{}
	for _, path := range paths {
		var err error
		var results *LoaderResults
		if source.IsStdin(path) {
			results, err = stdinLoad(l)
		} else if fs != nil {
			results, err = gitLoad(l, fs, filepath.Join(resourcePath, path))
		} else if source.IsHttp(path) {
			results, err = httpLoad(l, path)
		} else {
			results, err = fsLoad(l, path, skipUnreadable)
		}
		if err != nil {
			// one path that cannot be read must not drop what the other paths loaded
			if skipUnreadable && !pkgdeprecations.IsLegacyPolicyBlockError(err) {
				aggregateResults.addError(path, err)
				continue
			}
			return nil, err
		}
		aggregateResults.Merge(results)
	}
	return aggregateResults, nil
}

var loaderDelegate = sync.OnceValues(func() (resourceloader.Loader, error) {
	crds, err := data.Crds()
	if err != nil {
		return nil, err
	}
	factory, err := resourceloader.New(openapiclient.NewComposite(
		openapiclient.NewHardcodedBuiltins("1.32"),
		openapiclient.NewLocalCRDFiles(crds),
	))
	return factory, err
})

// legacyPolicyKinds are the policy kinds of the kyverno.io group, which also holds kinds that are not
// policies (GlobalContextEntry, UpdateRequest) and gets no new policy kinds.
var legacyPolicyKinds = sets.New("Policy", "ClusterPolicy", "CleanupPolicy", "ClusterCleanupPolicy", "PolicyException")

// isPolicyGroupKind reports whether gvk is something the loader reads policies from. For
// policies.kyverno.io it matches the whole group rather than the exact supported kinds and versions,
// so a policy written for a newer CLI is still reported as a policy that could not be loaded.
func isPolicyGroupKind(gvk schema.GroupVersionKind) bool {
	switch gvk.Group {
	case policiesv1.GroupVersion.Group:
		return true
	case "kyverno.io":
		return legacyPolicyKinds.Has(gvk.Kind)
	case admissionregistrationv1.GroupName:
		return strings.Contains(gvk.Kind, "AdmissionPolicy")
	}
	return false
}

// documentError prefixes err with the kind and name of the document it belongs to, when they can be
// read, so several broken policies in the same file can be told apart.
func documentError(document []byte, err error) error {
	var meta struct {
		Kind     string `json:"kind"`
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
	}
	if yaml.Unmarshal(document, &meta) != nil {
		return err
	}
	return namedError(meta.Kind, meta.Metadata.Name, err)
}

// namedError prefixes err with a document's kind and name, or returns it unchanged when either is
// unknown.
func namedError(kind, name string, err error) error {
	if kind == "" || name == "" {
		return err
	}
	return fmt.Errorf("%s %s: %w", kind, name, err)
}

// isValidYAML reports whether document parses as YAML.
func isValidYAML(document []byte) bool {
	_, err := yaml.YAMLToJSON(document)
	return err == nil
}

// kubectlValidateLoader returns the strict loader, which rejects legacy kyverno.io policy kinds with a
// migration-hint error, matching the 1.20 admission-time block. The first failing document fails the
// whole file.
func kubectlValidateLoader() loader {
	return validateLoader(false)
}

// validateLoader builds the loader. Legacy kyverno.io policy kinds are always rejected. In lenient mode
// every policy document that fails is recorded as a non fatal error and the rest of the file still loads,
// and documents outside the policy API groups are ignored. Otherwise the first failing document fails
// the whole file.
func validateLoader(lenient bool) loader {
	return func(path string, content []byte) (*LoaderResults, error) {
		results := &LoaderResults{}
		documents, err := splitDocuments(content)
		if err != nil {
			if !lenient {
				return nil, err
			}
			// keep the documents that were read before the error and report the rest of the file
			results.addError(path, err)
		}
		factory, err := loaderDelegate()
		if err != nil {
			return nil, err
		}
		// pendingErr holds the first "ordinary" (non-legacy-block) fatal error seen so far.
		// Scanning always continues past it so a legacy kind appearing later in the same
		// multi-document file still gets a chance to be blocked; a legacy-block error always
		// takes priority and returns immediately. If nothing later supersedes it, pendingErr is
		// what the whole file ultimately fails with, preserving the pre-existing guarantee that
		// a genuinely broken document fails the load rather than being silently dropped.
		var pendingErr error
		for _, document := range documents {
			gvk, untyped, err := factory.Load(document)
			if err != nil {
				// Check if this is a List object and handle it explicitly
				if gvk.Kind == "List" && gvk.Version == "v1" {
					if err := handleListItems(document, path, results, lenient); err != nil {
						if pkgdeprecations.IsLegacyPolicyBlockError(err) {
							return nil, err
						}
						// a List whose shape is broken holds no policy we can identify, policy items that
						// fail are reported by handleListItems itself
						if !lenient {
							results.addError(path, fmt.Errorf("failed to process List: %w", err))
						}
					}
					continue
				}
				// The loader returns the parsed GVK alongside a schema-validation error, so a
				// malformed legacy manifest must still be blocked with the migration hint rather
				// than surfacing only a generic validation error.
				if blockErr, ok := pkgdeprecations.BuildKindError(gvk.Group, gvk.Version, gvk.Kind); ok {
					return nil, blockErr
				}
				// a document that is not even valid YAML could be a broken policy, so only skip
				// documents we could read and that turned out not to be policies
				if lenient && !isPolicyGroupKind(gvk) && isValidYAML(document) {
					continue
				}
				msg := err.Error()
				if strings.Contains(msg, "Invalid value: value provided for unknown field") {
					if lenient {
						results.addError(path, documentError(document, err))
						continue
					}
					if pendingErr == nil {
						pendingErr = err
					}
					continue
				}
				// skip non-Kubernetes YAMLs and invalid types
				if lenient {
					err = documentError(document, err)
				}
				results.addError(path, err)
				continue
			}

			// Process regular documents (non-List). A legacy-policy-block error always aborts
			// the whole file immediately; any other error (unsupported kind, conversion failure)
			// is kept as the pending fatal error while scanning continues, so a later document in
			// the same multi-document file still gets a chance to be scanned and, if it's a
			// legacy kind, blocked ahead of it.
			if err := processDocumentItem(path, gvk, &untyped, results); err != nil {
				wrapped := fmt.Errorf("failed to process %s: %w", gvk, err)
				if pkgdeprecations.IsLegacyPolicyBlockError(err) {
					return nil, wrapped
				}
				if lenient {
					if isPolicyGroupKind(gvk) {
						results.addError(path, documentError(document, wrapped))
					}
					continue
				}
				if pendingErr == nil {
					pendingErr = wrapped
				}
			}
		}
		return results, pendingErr
	}
}

// handleListItems processes a v1.List object by extracting and processing its items
func handleListItems(document []byte, path string, results *LoaderResults, lenient bool) error {
	var jsonData []byte
	var parseErr error

	var listUnstructured unstructured.Unstructured
	if parseErr = listUnstructured.UnmarshalJSON(document); parseErr != nil {
		if jsonData, parseErr = yaml.YAMLToJSON(document); parseErr == nil {
			parseErr = listUnstructured.UnmarshalJSON(jsonData)
		}
	}

	if parseErr != nil {
		return parseErr
	}

	listItems, found, listErr := unstructured.NestedSlice(listUnstructured.Object, "items")
	if listErr != nil {
		return listErr
	}
	if !found || len(listItems) == 0 {
		return nil
	}

	for i, item := range listItems {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			// an item that is not an object cannot be a policy
			if !lenient {
				results.addError(path, fmt.Errorf("List item %d is not a valid object", i))
			}
			continue
		}

		itemUnstructured := &unstructured.Unstructured{Object: itemMap}
		itemGVK := itemUnstructured.GroupVersionKind()

		if err := processDocumentItem(path, itemGVK, itemUnstructured, results); err != nil {
			if pkgdeprecations.IsLegacyPolicyBlockError(err) {
				return fmt.Errorf("List item %d (%s): %w", i, itemGVK, err)
			}
			if lenient && !isPolicyGroupKind(itemGVK) {
				continue
			}
			itemErr := fmt.Errorf("failed to process List item %d (%s): %w", i, itemGVK, err)
			if lenient {
				// name the item like other documents; the strict loader keeps its error text as it was
				itemErr = namedError(itemGVK.Kind, itemUnstructured.GetName(), itemErr)
			}
			results.addError(path, itemErr)
		}
	}

	return nil
}

// processDocumentItem handles the processing of individual documents based on their GVK
func processDocumentItem(path string, gvk schema.GroupVersionKind, untyped *unstructured.Unstructured, results *LoaderResults) error {
	if warning, ok := pkgdeprecations.BuildKindWarning(gvk.Group, gvk.Version, gvk.Kind); ok {
		results.addWarning(path, warning.Message)
	}
	if err, ok := pkgdeprecations.BuildKindError(gvk.Group, gvk.Version, gvk.Kind); ok {
		return err
	}
	switch {
	case exception.IsCELException(gvk):
		typed, err := convert.To[policiesv1beta1.PolicyException](*untyped)
		if err != nil {
			return err
		}
		results.PolicyCelExceptions = append(results.PolicyCelExceptions, typed)
	default:
		switch gvk {
		case vapV1:
			typed, err := convert.To[admissionregistrationv1.ValidatingAdmissionPolicy](*untyped)
			if err != nil {
				return err
			}
			results.VAPs = append(results.VAPs, *typed)
		case vapBindingV1:
			typed, err := convert.To[admissionregistrationv1.ValidatingAdmissionPolicyBinding](*untyped)
			if err != nil {
				return err
			}
			results.VAPBindings = append(results.VAPBindings, *typed)
		case vpV1alpha1, vpV1beta1, vpV1:
			typed, err := convert.To[policiesv1beta1.ValidatingPolicy](*untyped)
			if err != nil {
				return err
			}
			switch typed.Spec.EvaluationMode() {
			case "Envoy":
				results.EnvoyPolicies = append(results.EnvoyPolicies, typed)
			case "HTTP":
				results.HTTPPolicies = append(results.HTTPPolicies, typed)
			default:
				results.ValidatingPolicies = append(results.ValidatingPolicies, typed)
			}
		case nvpV1beta1, nvpV1:
			typed, err := convert.To[policiesv1beta1.NamespacedValidatingPolicy](*untyped)
			if err != nil {
				return err
			}
			results.ValidatingPolicies = append(results.ValidatingPolicies, typed)
		case ivpV1alpha1, ivpV1beta1, ivpV1:
			typed, err := convert.To[policiesv1beta1.ImageValidatingPolicy](*untyped)
			if err != nil {
				return err
			}
			results.ImageValidatingPolicies = append(results.ImageValidatingPolicies, typed)
		case nivpV1beta1, nivpV1:
			typed, err := convert.To[policiesv1beta1.NamespacedImageValidatingPolicy](*untyped)
			if err != nil {
				return err
			}
			results.ImageValidatingPolicies = append(results.ImageValidatingPolicies, typed)
		case mapV1:
			typed, err := convert.To[admissionregistrationv1.MutatingAdmissionPolicy](*untyped)
			if err != nil {
				return err
			}
			results.MAPs = append(results.MAPs, *admissionpolicy.ConvertMutatingAdmissionPolicyToBeta(typed))
		case mapV1alpha1, mapV1beta1:
			typed, err := convert.To[admissionregistrationv1beta1.MutatingAdmissionPolicy](*untyped)
			if err != nil {
				return err
			}
			results.MAPs = append(results.MAPs, *typed)
		case mapBindingV1:
			typed, err := convert.To[admissionregistrationv1.MutatingAdmissionPolicyBinding](*untyped)
			if err != nil {
				return err
			}
			results.MAPBindings = append(results.MAPBindings, *admissionpolicy.ConvertMutatingAdmissionPolicyBindingToBeta(typed))
		case mapBindingV1alpha1, mapBindingV1beta1:
			typed, err := convert.To[admissionregistrationv1beta1.MutatingAdmissionPolicyBinding](*untyped)
			if err != nil {
				return err
			}
			results.MAPBindings = append(results.MAPBindings, *typed)
		case gpsV1alpha1, gpsV1beta1, gpsV1:
			typed, err := convert.To[policiesv1beta1.GeneratingPolicy](*untyped)
			if err != nil {
				return err
			}
			results.GeneratingPolicies = append(results.GeneratingPolicies, typed)
		case ngpsV1beta1, ngpsV1:
			typed, err := convert.To[policiesv1beta1.NamespacedGeneratingPolicy](*untyped)
			if err != nil {
				return err
			}
			results.GeneratingPolicies = append(results.GeneratingPolicies, typed)
		case dpV1alpha1, dpV1beta1, dpV1:
			typed, err := convert.To[policiesv1beta1.DeletingPolicy](*untyped)
			if err != nil {
				return err
			}
			results.DeletingPolicies = append(results.DeletingPolicies, typed)
		case ndpV1beta1, ndpV1:
			typed, err := convert.To[policiesv1beta1.NamespacedDeletingPolicy](*untyped)
			if err != nil {
				return err
			}
			results.DeletingPolicies = append(results.DeletingPolicies, typed)
		case mpV1alpha1, mpV1beta1, mpV1:
			typed, err := convert.To[policiesv1beta1.MutatingPolicy](*untyped)
			if err != nil {
				return err
			}
			results.MutatingPolicies = append(results.MutatingPolicies, typed)
		case nmpV1beta1, nmpV1:
			typed, err := convert.To[policiesv1beta1.NamespacedMutatingPolicy](*untyped)
			if err != nil {
				return err
			}
			results.MutatingPolicies = append(results.MutatingPolicies, typed)
		default:
			return errors.New("policy type not supported")
		}
	}
	return nil
}

// fsLoad loads a file or walks a directory. When skipUnreadable is set, a directory entry that
// cannot be read (a broken symlink, a permission error, a symlink back to a directory being walked)
// is recorded under its own path and skipped instead of aborting the whole directory.
func fsLoad(loader loader, path string, skipUnreadable bool) (*LoaderResults, error) {
	return fsLoadWalking(loader, path, skipUnreadable, sets.New[string]())
}

// fsLoadWalking is fsLoad given the real paths of the directories being walked, so that the lenient
// walk stops at a symlink loop instead of loading the same policies again on every level.
func fsLoadWalking(loader loader, path string, skipUnreadable bool, walking sets.Set[string]) (*LoaderResults, error) {
	fi, err := os.Stat(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(fi.Name(), ".") {
		// skip hidden files and dirs
		return nil, err
	}
	aggregateResults := &LoaderResults{}
	if fi.IsDir() {
		if skipUnreadable {
			// resolve from an absolute path, a relative one would never match a symlink with an absolute target
			absPath, err := filepath.Abs(path)
			if err != nil {
				return nil, err
			}
			realPath, err := filepath.EvalSymlinks(absPath)
			if err != nil {
				return nil, err
			}
			if walking.Has(realPath) {
				return nil, errors.Errorf("symlink loop, %s is already being loaded", realPath)
			}
			walking.Insert(realPath)
			defer walking.Delete(realPath)
		}
		files, err := os.ReadDir(path)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read %s", path)
		}
		for _, file := range files {
			entry := filepath.Join(path, file.Name())
			if skipUnreadable && strings.HasPrefix(file.Name(), ".") {
				// skip hidden entries before touching them, so a broken hidden symlink is not reported
				continue
			}
			results, err := fsLoadWalking(loader, entry, skipUnreadable, walking)
			if err != nil {
				if skipUnreadable && !pkgdeprecations.IsLegacyPolicyBlockError(err) {
					// only report entries that could have held policies, other files are ignored anyway;
					// a symlink is reported when it points at a directory, or when it has no extension
					// and cannot be followed, since it may have pointed at one
					ext := filepath.Ext(file.Name())
					symlink := file.Type()&os.ModeSymlink != 0
					if file.IsDir() || ext == ".yaml" || ext == ".yml" || (symlink && (ext == "" || isDirectory(entry))) {
						aggregateResults.addError(entry, err)
					}
					continue
				}
				return nil, errors.Wrapf(err, "failed to load %s", path)
			}
			aggregateResults.Merge(results)
		}
	} else if git.IsYaml(fi) {
		fileBytes, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read file %s", path)
		}
		results, err := loader(path, fileBytes)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to load file %s", path)
		}
		aggregateResults.Merge(results)
	}
	return aggregateResults, nil
}

// isDirectory reports whether path, following symlinks, is a directory.
func isDirectory(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}

var remoteHTTPTimeout = utils.RemoteHTTPTimeout

func httpLoad(loader loader, path string) (*LoaderResults, error) {
	// We accept here that a random URL might be called based on user provided input.
	ctx, cancel := context.WithTimeout(context.Background(), remoteHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to process %v: %w", path, err)
	}

	resp, err := utils.RemoteHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to process %v: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to process %v: HTTP %s", path, resp.Status)
	}
	fileBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to process %v: %w", path, err)
	}
	return loader(path, fileBytes)
}

func gitLoad(loader loader, fs billy.Filesystem, path string) (*LoaderResults, error) {
	file, err := fs.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return loader(path, fileBytes)
}

func stdinLoad(loader loader) (*LoaderResults, error) {
	policyStr := ""
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		policyStr = policyStr + scanner.Text() + "\n"
	}
	return loader("-", []byte(policyStr))
}
