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
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/source"
	"github.com/kyverno/kyverno/ext/resource/convert"
	resourceloader "github.com/kyverno/kyverno/ext/resource/loader"
	extyaml "github.com/kyverno/kyverno/ext/yaml"
	"github.com/kyverno/kyverno/pkg/utils/git"
	"github.com/pkg/errors"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kubectl-validate/pkg/openapiclient"
	"sigs.k8s.io/yaml"
)

var (
	policyV1           = schema.GroupVersion(kyvernov1.GroupVersion).WithKind("Policy")
	policyV2           = schema.GroupVersion(kyvernov2beta1.GroupVersion).WithKind("Policy")
	clusterPolicyV1    = schema.GroupVersion(kyvernov1.GroupVersion).WithKind("ClusterPolicy")
	clusterPolicyV2    = schema.GroupVersion(kyvernov2beta1.GroupVersion).WithKind("ClusterPolicy")
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
	polexv2            = schema.GroupVersion(kyvernov2.GroupVersion).WithKind("PolicyException")
	polexv1beta1       = schema.GroupVersion(kyvernov2beta1.GroupVersion).WithKind("PolicyException")
	polexcelv1beta1    = schema.GroupVersion(policiesv1beta1.GroupVersion).WithKind("PolicyException")
	mpV1beta1          = schema.GroupVersion(policiesv1beta1.GroupVersion).WithKind("MutatingPolicy")
	mpV1               = schema.GroupVersion(policiesv1.GroupVersion).WithKind("MutatingPolicy")
	nmpV1beta1         = schema.GroupVersion(policiesv1beta1.GroupVersion).WithKind("NamespacedMutatingPolicy")
	nmpV1              = schema.GroupVersion(policiesv1.GroupVersion).WithKind("NamespacedMutatingPolicy")
	mapV1alpha1        = admissionregistrationv1alpha1.SchemeGroupVersion.WithKind("MutatingAdmissionPolicy")
	mapV1beta1         = admissionregistrationv1beta1.SchemeGroupVersion.WithKind("MutatingAdmissionPolicy")
	mapBindingV1alpha1 = admissionregistrationv1alpha1.SchemeGroupVersion.WithKind("MutatingAdmissionPolicyBinding")
	mapBindingV1beta1  = admissionregistrationv1beta1.SchemeGroupVersion.WithKind("MutatingAdmissionPolicyBinding")
	defaultLoader      = kubectlValidateLoader
)

type LoaderError struct {
	Path  string
	Error error
}

type LoaderResults struct {
	Policies                []kyvernov1.PolicyInterface
	PolicyExceptions        []*kyvernov2.PolicyException
	PolicyCELExceptions     []*policiesv1beta1.PolicyException
	VAPs                    []admissionregistrationv1.ValidatingAdmissionPolicy
	VAPBindings             []admissionregistrationv1.ValidatingAdmissionPolicyBinding
	MAPs                    []admissionregistrationv1beta1.MutatingAdmissionPolicy
	MAPBindings             []admissionregistrationv1beta1.MutatingAdmissionPolicyBinding
	ValidatingPolicies      []policiesv1beta1.ValidatingPolicyLike
	ImageValidatingPolicies []policiesv1beta1.ImageValidatingPolicyLike
	GeneratingPolicies      []policiesv1beta1.GeneratingPolicyLike
	DeletingPolicies        []policiesv1beta1.DeletingPolicyLike
	MutatingPolicies        []policiesv1beta1.MutatingPolicyLike
	PolicyCelExceptions     []*policiesv1beta1.PolicyException
	NonFatalErrors          []LoaderError
}

func (l *LoaderResults) merge(results *LoaderResults) {
	if results == nil {
		return
	}
	l.Policies = append(l.Policies, results.Policies...)
	l.VAPs = append(l.VAPs, results.VAPs...)
	l.VAPBindings = append(l.VAPBindings, results.VAPBindings...)
	l.ValidatingPolicies = append(l.ValidatingPolicies, results.ValidatingPolicies...)
	l.MAPs = append(l.MAPs, results.MAPs...)
	l.MAPBindings = append(l.MAPBindings, results.MAPBindings...)
	l.ImageValidatingPolicies = append(l.ImageValidatingPolicies, results.ImageValidatingPolicies...)
	l.GeneratingPolicies = append(l.GeneratingPolicies, results.GeneratingPolicies...)
	l.NonFatalErrors = append(l.NonFatalErrors, results.NonFatalErrors...)
	l.DeletingPolicies = append(l.DeletingPolicies, results.DeletingPolicies...)
	l.PolicyExceptions = append(l.PolicyExceptions, results.PolicyExceptions...)
	l.PolicyCelExceptions = append(l.PolicyCelExceptions, results.PolicyCelExceptions...)
	l.MutatingPolicies = append(l.MutatingPolicies, results.MutatingPolicies...)
}

func (l *LoaderResults) addError(path string, err error) {
	l.NonFatalErrors = append(l.NonFatalErrors, LoaderError{
		Path:  path,
		Error: err,
	})
}

type loader = func(string, []byte) (*LoaderResults, error)

func Load(fs billy.Filesystem, resourcePath string, paths ...string) (*LoaderResults, error) {
	return LoadWithLoader(nil, fs, resourcePath, paths...)
}

func LoadWithLoader(loader loader, fs billy.Filesystem, resourcePath string, paths ...string) (*LoaderResults, error) {
	if loader == nil {
		loader = defaultLoader
	}
	aggregateResults := &LoaderResults{}
	for _, path := range paths {
		var err error
		var results *LoaderResults
		if source.IsStdin(path) {
			results, err = stdinLoad(loader)
		} else if fs != nil {
			results, err = gitLoad(loader, fs, filepath.Join(resourcePath, path))
		} else if source.IsHttp(path) {
			results, err = httpLoad(loader, path)
		} else {
			results, err = fsLoad(loader, path)
		}
		if err != nil {
			return nil, err
		}
		aggregateResults.merge(results)
	}
	// It's hard to use apply with the fake client, so disable all server side
	// https://github.com/kubernetes/kubernetes/issues/99953
	for _, policy := range aggregateResults.Policies {
		policy.GetSpec().UseServerSideApply = false
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

func kubectlValidateLoader(path string, content []byte) (*LoaderResults, error) {
	documents, err := extyaml.SplitDocuments(content)
	if err != nil {
		return nil, err
	}
	results := &LoaderResults{}
	factory, err := loaderDelegate()
	if err != nil {
		return nil, err
	}
	for _, document := range documents {
		gvk, untyped, err := factory.Load(document)
		if err != nil {
			// Check if this is a List object and handle it explicitly
			if gvk.Kind == "List" && gvk.Version == "v1" {
				if err := handleListItems(document, path, results); err != nil {
					results.addError(path, fmt.Errorf("failed to process List: %w", err))
				}
				continue
			}
			msg := err.Error()
			if strings.Contains(msg, "Invalid value: value provided for unknown field") {
				return nil, err
			}
			// skip non-Kubernetes YAMLs and invalid types
			results.addError(path, err)
			continue
		}

		// Process regular documents (non-List)
		if err := processDocumentItem(gvk, &untyped, results); err != nil {
			return nil, fmt.Errorf("policy type not supported %s", gvk)
		}
	}
	return results, nil
}

// handleListItems processes a v1.List object by extracting and processing its items
func handleListItems(document []byte, path string, results *LoaderResults) error {
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
			results.addError(path, fmt.Errorf("List item %d is not a valid object", i))
			continue
		}

		itemUnstructured := &unstructured.Unstructured{Object: itemMap}
		itemGVK := itemUnstructured.GroupVersionKind()

		if err := processDocumentItem(itemGVK, itemUnstructured, results); err != nil {
			results.addError(path, fmt.Errorf("failed to process List item %d: %w", i, err))
		}
	}

	return nil
}

// processDocumentItem handles the processing of individual documents based on their GVK
func processDocumentItem(gvk schema.GroupVersionKind, untyped *unstructured.Unstructured, results *LoaderResults) error {
	switch gvk {
	case policyV1, policyV2:
		typed, err := convert.To[kyvernov1.Policy](*untyped)
		if err != nil {
			return err
		}
		results.Policies = append(results.Policies, typed)
	case clusterPolicyV1, clusterPolicyV2:
		typed, err := convert.To[kyvernov1.ClusterPolicy](*untyped)
		if err != nil {
			return err
		}
		results.Policies = append(results.Policies, typed)
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
	case polexv2, polexv1beta1:
		typed, err := convert.To[*kyvernov2.PolicyException](*untyped)
		if err != nil {
			return err
		}
		results.PolicyExceptions = append(results.PolicyExceptions, *typed)
	case polexcelv1beta1:
		typed, err := convert.To[*policiesv1beta1.PolicyException](*untyped)
		if err != nil {
			return err
		}
		results.PolicyCelExceptions = append(results.PolicyCelExceptions, *typed)
	case vpV1alpha1, vpV1beta1, vpV1:
		typed, err := convert.To[policiesv1beta1.ValidatingPolicy](*untyped)
		if err != nil {
			return err
		}
		results.ValidatingPolicies = append(results.ValidatingPolicies, typed)
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
	case mapV1alpha1, mapV1beta1:
		typed, err := convert.To[admissionregistrationv1beta1.MutatingAdmissionPolicy](*untyped)
		if err != nil {
			return err
		}
		results.MAPs = append(results.MAPs, *typed)
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
		return fmt.Errorf("policy type not supported %s", gvk)
	}
	return nil
}

func fsLoad(loader loader, path string) (*LoaderResults, error) {
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
		files, err := os.ReadDir(path)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read %s", path)
		}
		for _, file := range files {
			results, err := fsLoad(loader, filepath.Join(path, file.Name()))
			if err != nil {
				return nil, errors.Wrapf(err, "failed to load %s", path)
			}
			aggregateResults.merge(results)
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
		aggregateResults.merge(results)
	}
	return aggregateResults, nil
}

func httpLoad(loader loader, path string) (*LoaderResults, error) {
	// We accept here that a random URL might be called based on user provided input.
	req, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to process %v: %v", path, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to process %v: %v", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to process %v: HTTP %s", path, resp.Status)
	}
	fileBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to process %v: %v", path, err)
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
