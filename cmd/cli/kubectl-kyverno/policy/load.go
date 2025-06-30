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

	"github.com/go-git/go-billy/v5"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/source"
	"github.com/kyverno/kyverno/ext/resource/convert"
	resourceloader "github.com/kyverno/kyverno/ext/resource/loader"
	extyaml "github.com/kyverno/kyverno/ext/yaml"
	"github.com/kyverno/kyverno/pkg/utils/git"
	"github.com/pkg/errors"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	"sigs.k8s.io/kubectl-validate/pkg/openapiclient"
)

var (
	policyV1           = kyvernov1.SchemeGroupVersion.WithKind("Policy")
	policyV2           = kyvernov2beta1.SchemeGroupVersion.WithKind("Policy")
	clusterPolicyV1    = kyvernov1.SchemeGroupVersion.WithKind("ClusterPolicy")
	clusterPolicyV2    = kyvernov2beta1.SchemeGroupVersion.WithKind("ClusterPolicy")
	vapV1              = admissionregistrationv1.SchemeGroupVersion.WithKind("ValidatingAdmissionPolicy")
	vapBindingV1       = admissionregistrationv1.SchemeGroupVersion.WithKind("ValidatingAdmissionPolicyBinding")
	vpV1alpha1         = policiesv1alpha1.SchemeGroupVersion.WithKind("ValidatingPolicy")
	ivpV1alpha1        = policiesv1alpha1.SchemeGroupVersion.WithKind("ImageValidatingPolicy")
	gpsV1alpha1        = policiesv1alpha1.SchemeGroupVersion.WithKind("GeneratingPolicy")
	dpV1alpha1         = policiesv1alpha1.SchemeGroupVersion.WithKind("DeletingPolicy")
	mpV1alpha1         = policiesv1alpha1.SchemeGroupVersion.WithKind("MutatingPolicy")
	mapV1alpha1        = admissionregistrationv1alpha1.SchemeGroupVersion.WithKind("MutatingAdmissionPolicy")
	mapBindingV1alpha1 = admissionregistrationv1alpha1.SchemeGroupVersion.WithKind("MutatingAdmissionPolicyBinding")
	defaultLoader      = kubectlValidateLoader
)

type LoaderError struct {
	Path  string
	Error error
}

type LoaderResults struct {
	Policies                []kyvernov1.PolicyInterface
	VAPs                    []admissionregistrationv1.ValidatingAdmissionPolicy
	VAPBindings             []admissionregistrationv1.ValidatingAdmissionPolicyBinding
	MAPs                    []admissionregistrationv1alpha1.MutatingAdmissionPolicy
	MAPBindings             []admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding
	ValidatingPolicies      []policiesv1alpha1.ValidatingPolicy
	ImageValidatingPolicies []policiesv1alpha1.ImageValidatingPolicy
	GeneratingPolicies      []policiesv1alpha1.GeneratingPolicy
	DeletingPolicies        []policiesv1alpha1.DeletingPolicy
	MutatingPolicies        []policiesv1alpha1.MutatingPolicy
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

func kubectlValidateLoader(path string, content []byte) (*LoaderResults, error) {
	documents, err := extyaml.SplitDocuments(content)
	if err != nil {
		return nil, err
	}
	results := &LoaderResults{}
	crds, err := data.Crds()
	if err != nil {
		return nil, err
	}
	factory, err := resourceloader.New(openapiclient.NewComposite(
		openapiclient.NewHardcodedBuiltins("1.32"),
		openapiclient.NewLocalCRDFiles(crds),
	))
	if err != nil {
		return nil, err
	}
	for _, document := range documents {
		gvk, untyped, err := factory.Load(document)
		if err != nil {
			msg := err.Error()
			if strings.Contains(msg, "Invalid value: value provided for unknown field") {
				return nil, err
			}
			// skip non-Kubernetes YAMLs and invalid types
			results.addError(path, err)
			continue
		}
		switch gvk {
		case policyV1, policyV2:
			typed, err := convert.To[kyvernov1.Policy](untyped)
			if err != nil {
				return nil, err
			}
			results.Policies = append(results.Policies, typed)
		case clusterPolicyV1, clusterPolicyV2:
			typed, err := convert.To[kyvernov1.ClusterPolicy](untyped)
			if err != nil {
				return nil, err
			}
			results.Policies = append(results.Policies, typed)
		case vapV1:
			typed, err := convert.To[admissionregistrationv1.ValidatingAdmissionPolicy](untyped)
			if err != nil {
				return nil, err
			}
			results.VAPs = append(results.VAPs, *typed)
		case vapBindingV1:
			typed, err := convert.To[admissionregistrationv1.ValidatingAdmissionPolicyBinding](untyped)
			if err != nil {
				return nil, err
			}
			results.VAPBindings = append(results.VAPBindings, *typed)
		case vpV1alpha1:
			typed, err := convert.To[policiesv1alpha1.ValidatingPolicy](untyped)
			if err != nil {
				return nil, err
			}
			results.ValidatingPolicies = append(results.ValidatingPolicies, *typed)
		case ivpV1alpha1:
			typed, err := convert.To[policiesv1alpha1.ImageValidatingPolicy](untyped)
			if err != nil {
				return nil, err
			}
			results.ImageValidatingPolicies = append(results.ImageValidatingPolicies, *typed)
		case mapV1alpha1:
			typed, err := convert.To[admissionregistrationv1alpha1.MutatingAdmissionPolicy](untyped)
			if err != nil {
				return nil, err
			}
			results.MAPs = append(results.MAPs, *typed)
		case mapBindingV1alpha1:
			typed, err := convert.To[admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding](untyped)
			if err != nil {
				return nil, err
			}
			results.MAPBindings = append(results.MAPBindings, *typed)
		case gpsV1alpha1:
			typed, err := convert.To[policiesv1alpha1.GeneratingPolicy](untyped)
			if err != nil {
				return nil, err
			}
			results.GeneratingPolicies = append(results.GeneratingPolicies, *typed)
		case dpV1alpha1:
			typed, err := convert.To[policiesv1alpha1.DeletingPolicy](untyped)
			if err != nil {
				return nil, err
			}
			results.DeletingPolicies = append(results.DeletingPolicies, *typed)
		case mpV1alpha1:
			typed, err := convert.To[policiesv1alpha1.MutatingPolicy](untyped)
			if err != nil {
				return nil, err
			}
			results.MutatingPolicies = append(results.MutatingPolicies, *typed)
		default:
			return nil, fmt.Errorf("policy type not supported %s", gvk)
		}
	}
	return results, nil
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
		return nil, fmt.Errorf("failed to process %v: %v", path, err)
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
