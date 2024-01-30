package policy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/experimental"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/source"
	"github.com/kyverno/kyverno/ext/resource/convert"
	resourceloader "github.com/kyverno/kyverno/ext/resource/loader"
	extyaml "github.com/kyverno/kyverno/ext/yaml"
	"github.com/kyverno/kyverno/pkg/utils/git"
	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
	"k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kubectl-validate/pkg/openapiclient"
)

var (
	factory, _ = resourceloader.New(openapiclient.NewComposite(
		openapiclient.NewHardcodedBuiltins("1.28"),
		openapiclient.NewLocalCRDFiles(data.Crds(), data.CrdsFolder),
	))
	policyV1              = schema.GroupVersion(kyvernov1.GroupVersion).WithKind("Policy")
	policyV2              = schema.GroupVersion(kyvernov2beta1.GroupVersion).WithKind("Policy")
	clusterPolicyV1       = schema.GroupVersion(kyvernov1.GroupVersion).WithKind("ClusterPolicy")
	clusterPolicyV2       = schema.GroupVersion(kyvernov2beta1.GroupVersion).WithKind("ClusterPolicy")
	vapV1alpha1           = v1alpha1.SchemeGroupVersion.WithKind("ValidatingAdmissionPolicy")
	vapV1Beta1            = v1beta1.SchemeGroupVersion.WithKind("ValidatingAdmissionPolicy")
	vapBidningV1alpha1    = v1alpha1.SchemeGroupVersion.WithKind("ValidatingAdmissionPolicyBinding")
	vapBidningV1beta1     = v1beta1.SchemeGroupVersion.WithKind("ValidatingAdmissionPolicyBinding")
	exceptionV2beta1      = schema.GroupVersion(kyvernov2beta1.GroupVersion).WithKind("PolicyException")
	exceptionV2           = schema.GroupVersion(kyvernov2.GroupVersion).WithKind("PolicyException")
	LegacyLoader          = yamlutils.GetPolicy
	KubectlValidateLoader = kubectlValidateLoader
	defaultLoader         = func(bytes []byte) ([]kyvernov1.PolicyInterface, []v1alpha1.ValidatingAdmissionPolicy, []v1alpha1.ValidatingAdmissionPolicyBinding, []kyvernov2beta1.PolicyException, error) {
		if experimental.UseKubectlValidate() {
			return KubectlValidateLoader(bytes)
		} else {
			return LegacyLoader(bytes)
		}
	}
)

type loader = func([]byte) ([]kyvernov1.PolicyInterface, []v1alpha1.ValidatingAdmissionPolicy, []v1alpha1.ValidatingAdmissionPolicyBinding, []kyvernov2beta1.PolicyException, error)

func Load(fs billy.Filesystem, resourcePath string, paths ...string) ([]kyvernov1.PolicyInterface, []v1alpha1.ValidatingAdmissionPolicy, []v1alpha1.ValidatingAdmissionPolicyBinding, []kyvernov2beta1.PolicyException, error) {
	return LoadWithLoader(nil, fs, resourcePath, paths...)
}

func LoadWithLoader(loader loader, fs billy.Filesystem, resourcePath string, paths ...string) ([]kyvernov1.PolicyInterface, []v1alpha1.ValidatingAdmissionPolicy, []v1alpha1.ValidatingAdmissionPolicyBinding, []kyvernov2beta1.PolicyException, error) {
	if loader == nil {
		loader = defaultLoader
	}
	var pols []kyvernov1.PolicyInterface
	var vaps []v1alpha1.ValidatingAdmissionPolicy
	var vapBindings []v1alpha1.ValidatingAdmissionPolicyBinding
	var polex []kyvernov2beta1.PolicyException
	for _, path := range paths {
		if source.IsStdin(path) {
			p, v, b, pe, err := stdinLoad(loader)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			pols = append(pols, p...)
			vaps = append(vaps, v...)
			vapBindings = append(vapBindings, b...)
			polex = append(polex, pe...)
		} else if fs != nil {
			p, v, b, pe, err := gitLoad(loader, fs, filepath.Join(resourcePath, path))
			if err != nil {
				return nil, nil, nil, nil, err
			}
			pols = append(pols, p...)
			vaps = append(vaps, v...)
			vapBindings = append(vapBindings, b...)
			polex = append(polex, pe...)
		} else if source.IsHttp(path) {
			p, v, b, pe, err := httpLoad(loader, path)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			pols = append(pols, p...)
			vaps = append(vaps, v...)
			vapBindings = append(vapBindings, b...)
			polex = append(polex, pe...)
		} else {
			p, v, b, pe, err := fsLoad(loader, path)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			pols = append(pols, p...)
			vaps = append(vaps, v...)
			vapBindings = append(vapBindings, b...)
			polex = append(polex, pe...)
		}
	}
  
	// It's hard to use apply with the fake client, so disable all server side
	// https://github.com/kubernetes/kubernetes/issues/99953
	for _, policy := range pols {
		policy.GetSpec().UseServerSideApply = false
	}

	return pols, vaps, vapBindings, polex, nil
}

func kubectlValidateLoader(content []byte) ([]kyvernov1.PolicyInterface, []v1alpha1.ValidatingAdmissionPolicy, []v1alpha1.ValidatingAdmissionPolicyBinding, []kyvernov2beta1.PolicyException, error) {
	documents, err := extyaml.SplitDocuments(content)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	var policies []kyvernov1.PolicyInterface
	var vaps []v1alpha1.ValidatingAdmissionPolicy
	var vapBindings []v1alpha1.ValidatingAdmissionPolicyBinding
	var exceptions []kyvernov2beta1.PolicyException
	for _, document := range documents {
		gvk, untyped, err := factory.Load(document)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		switch gvk {
		case policyV1, policyV2:
			typed, err := convert.To[kyvernov1.Policy](untyped)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			policies = append(policies, typed)
		case clusterPolicyV1, clusterPolicyV2:
			typed, err := convert.To[kyvernov1.ClusterPolicy](untyped)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			policies = append(policies, typed)
		case vapV1alpha1, vapV1Beta1:
			typed, err := convert.To[v1alpha1.ValidatingAdmissionPolicy](untyped)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			vaps = append(vaps, *typed)
		case vapBidningV1alpha1, vapBidningV1beta1:
			typed, err := convert.To[v1alpha1.ValidatingAdmissionPolicyBinding](untyped)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			vapBindings = append(vapBindings, *typed)
		case exceptionV2beta1, exceptionV2:
			typed, err := convert.To[kyvernov2beta1.PolicyException](untyped)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			exceptions = append(exceptions, *typed)
		default:
			return nil, nil, nil, nil, fmt.Errorf("policy type not supported %s", gvk)
		}
	}
	return policies, vaps, vapBindings, exceptions, err
}

func fsLoad(loader loader, path string) ([]kyvernov1.PolicyInterface, []v1alpha1.ValidatingAdmissionPolicy, []v1alpha1.ValidatingAdmissionPolicyBinding, []kyvernov2beta1.PolicyException, error) {
	var pols []kyvernov1.PolicyInterface
	var vaps []v1alpha1.ValidatingAdmissionPolicy
	var vapBindings []v1alpha1.ValidatingAdmissionPolicyBinding
	var polex []kyvernov2beta1.PolicyException
	fi, err := os.Stat(filepath.Clean(path))
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if fi.IsDir() {
		files, err := os.ReadDir(path)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		for _, file := range files {
			p, v, b, pe, err := fsLoad(loader, filepath.Join(path, file.Name()))
			if err != nil {
				return nil, nil, nil, nil, err
			}
			pols = append(pols, p...)
			vaps = append(vaps, v...)
			vapBindings = append(vapBindings, b...)
			polex = append(polex, pe...)
		}
	} else if git.IsYaml(fi) {
		fileBytes, err := os.ReadFile(filepath.Clean(path)) // #nosec G304
		if err != nil {
			return nil, nil, nil, nil, err
		}
		p, v, b, pe, err := loader(fileBytes)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		pols = append(pols, p...)
		vaps = append(vaps, v...)
		vapBindings = append(vapBindings, b...)
		polex = append(polex, pe...)
	}
	return pols, vaps, vapBindings, polex, nil
}

func httpLoad(loader loader, path string) ([]kyvernov1.PolicyInterface, []v1alpha1.ValidatingAdmissionPolicy, []v1alpha1.ValidatingAdmissionPolicyBinding, []kyvernov2beta1.PolicyException, error) {
	// We accept here that a random URL might be called based on user provided input.
	req, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, path, nil)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to process %v: %v", path, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to process %v: %v", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, nil, nil, fmt.Errorf("failed to process %v: %v", path, err)
	}
	fileBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to process %v: %v", path, err)
	}
	return loader(fileBytes)
}

func gitLoad(loader loader, fs billy.Filesystem, path string) ([]kyvernov1.PolicyInterface, []v1alpha1.ValidatingAdmissionPolicy, []v1alpha1.ValidatingAdmissionPolicyBinding, []kyvernov2beta1.PolicyException, error) {
	file, err := fs.Open(path)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return loader(fileBytes)
}

func stdinLoad(loader loader) ([]kyvernov1.PolicyInterface, []v1alpha1.ValidatingAdmissionPolicy, []v1alpha1.ValidatingAdmissionPolicyBinding, []kyvernov2beta1.PolicyException, error) {
	policyStr := ""
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		policyStr = policyStr + scanner.Text() + "\n"
	}
	return loader([]byte(policyStr))
}
