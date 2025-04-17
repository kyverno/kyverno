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
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/experimental"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/source"
	"github.com/kyverno/kyverno/ext/resource/convert"
	resourceloader "github.com/kyverno/kyverno/ext/resource/loader"
	extyaml "github.com/kyverno/kyverno/ext/yaml"
	"github.com/kyverno/kyverno/pkg/utils/git"
	"github.com/pkg/errors"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kubectl-validate/pkg/openapiclient"
)

var (
	policyV1              = schema.GroupVersion(kyvernov1.GroupVersion).WithKind("Policy")
	policyV2              = schema.GroupVersion(kyvernov2beta1.GroupVersion).WithKind("Policy")
	clusterPolicyV1       = schema.GroupVersion(kyvernov1.GroupVersion).WithKind("ClusterPolicy")
	clusterPolicyV2       = schema.GroupVersion(kyvernov2beta1.GroupVersion).WithKind("ClusterPolicy")
	vapV1Beta1            = admissionregistrationv1beta1.SchemeGroupVersion.WithKind("ValidatingAdmissionPolicy")
	vapBindingV1beta1     = admissionregistrationv1beta1.SchemeGroupVersion.WithKind("ValidatingAdmissionPolicyBinding")
	vapV1                 = admissionregistrationv1.SchemeGroupVersion.WithKind("ValidatingAdmissionPolicy")
	vapBindingV1          = admissionregistrationv1.SchemeGroupVersion.WithKind("ValidatingAdmissionPolicyBinding")
	LegacyLoader          = legacyLoader
	KubectlValidateLoader = kubectlValidateLoader
	defaultLoader         = func(path string, bytes []byte) (*LoaderResults, error) {
		if experimental.UseKubectlValidate() {
			return KubectlValidateLoader(path, bytes)
		} else {
			return LegacyLoader(path, bytes)
		}
	}
)

type LoaderError struct {
	Path  string
	Error error
}

type LoaderResults struct {
	Policies       []kyvernov1.PolicyInterface
	VAPs           []admissionregistrationv1beta1.ValidatingAdmissionPolicy
	VAPBindings    []admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding
	NonFatalErrors []LoaderError
}

func (l *LoaderResults) merge(results *LoaderResults) {
	if results == nil {
		return
	}
	l.Policies = append(l.Policies, results.Policies...)
	l.VAPs = append(l.VAPs, results.VAPs...)
	l.VAPBindings = append(l.VAPBindings, results.VAPBindings...)
	l.NonFatalErrors = append(l.NonFatalErrors, results.NonFatalErrors...)
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
		openapiclient.NewHardcodedBuiltins("1.30"),
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
		case vapV1Beta1, vapV1:
			typed, err := convert.To[admissionregistrationv1beta1.ValidatingAdmissionPolicy](untyped)
			if err != nil {
				return nil, err
			}
			results.VAPs = append(results.VAPs, *typed)
		case vapBindingV1beta1, vapBindingV1:
			typed, err := convert.To[admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding](untyped)
			if err != nil {
				return nil, err
			}
			results.VAPBindings = append(results.VAPBindings, *typed)
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
