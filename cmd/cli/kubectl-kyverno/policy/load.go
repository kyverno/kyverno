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
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/experimental"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/source"
	"github.com/kyverno/kyverno/pkg/utils/git"
	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
	"k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/kubectl-validate/pkg/openapiclient"
	"sigs.k8s.io/kubectl-validate/pkg/validatorfactory"
)

var (
	factory, _      = validatorfactory.New(client)
	policyV1        = schema.GroupVersion(kyvernov1.GroupVersion).WithKind("Policy")
	policyV2        = schema.GroupVersion(kyvernov2beta1.GroupVersion).WithKind("Policy")
	clusterPolicyV1 = schema.GroupVersion(kyvernov1.GroupVersion).WithKind("ClusterPolicy")
	clusterPolicyV2 = schema.GroupVersion(kyvernov2beta1.GroupVersion).WithKind("ClusterPolicy")
	vapV1           = v1alpha1.SchemeGroupVersion.WithKind("ValidatingAdmissionPolicy")
	client          = openapiclient.NewComposite(
		openapiclient.NewHardcodedBuiltins("1.27"),
		openapiclient.NewLocalCRDFiles(data.Crds(), data.CrdsFolder),
	)
)

func getPolicies(bytes []byte) ([]kyvernov1.PolicyInterface, []v1alpha1.ValidatingAdmissionPolicy, error) {
	if !experimental.UseKubectlValidate() {
		return yamlutils.GetPolicy(bytes)
	}
	var policies []kyvernov1.PolicyInterface
	var validatingAdmissionPolicies []v1alpha1.ValidatingAdmissionPolicy
	documents, err := yamlutils.SplitDocuments(bytes)
	if err != nil {
		return nil, nil, err
	}
	for _, document := range documents {
		var metadata metav1.TypeMeta
		if err := yaml.Unmarshal(document, &metadata); err != nil {
			return nil, nil, err
		}
		gvk := metadata.GetObjectKind().GroupVersionKind()
		validator, err := factory.ValidatorsForGVK(gvk)
		if err != nil {
			return nil, nil, err
		}
		decoder, err := validator.Decoder(gvk)
		if err != nil {
			return nil, nil, err
		}
		info, ok := runtime.SerializerInfoForMediaType(decoder.SupportedMediaTypes(), runtime.ContentTypeYAML)
		if !ok {
			return nil, nil, fmt.Errorf("failed to get serializer info for %s", gvk)
		}
		var untyped unstructured.Unstructured
		_, _, err = decoder.DecoderToVersion(info.StrictSerializer, gvk.GroupVersion()).Decode(document, &gvk, &untyped)
		if err != nil {
			return nil, nil, err
		}
		switch gvk {
		case policyV1, policyV2:
			var policy kyvernov1.Policy
			if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(untyped.UnstructuredContent(), &policy, true); err != nil {
				return nil, nil, err
			}
			policies = append(policies, &policy)
		case clusterPolicyV1, clusterPolicyV2:
			var policy kyvernov1.ClusterPolicy
			if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(untyped.UnstructuredContent(), &policy, true); err != nil {
				return nil, nil, err
			}
			policies = append(policies, &policy)
		case vapV1:
			var policy v1alpha1.ValidatingAdmissionPolicy
			if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(untyped.UnstructuredContent(), &policy, true); err != nil {
				return nil, nil, err
			}
			validatingAdmissionPolicies = append(validatingAdmissionPolicies, policy)
		default:
			return nil, nil, fmt.Errorf("policy type not supported %s", gvk)
		}
	}
	return policies, validatingAdmissionPolicies, nil
}

func Load(fs billy.Filesystem, resourcePath string, paths ...string) ([]kyvernov1.PolicyInterface, []v1alpha1.ValidatingAdmissionPolicy, error) {
	var pols []kyvernov1.PolicyInterface
	var vaps []v1alpha1.ValidatingAdmissionPolicy
	for _, path := range paths {
		if source.IsStdin(path) {
			p, v, err := stdinLoad()
			if err != nil {
				return nil, nil, err
			}
			pols = append(pols, p...)
			vaps = append(vaps, v...)
		} else if fs != nil {
			p, v, err := gitLoad(fs, filepath.Join(resourcePath, path))
			if err != nil {
				return nil, nil, err
			}
			pols = append(pols, p...)
			vaps = append(vaps, v...)
		} else if source.IsHttp(path) {
			p, v, err := httpLoad(path)
			if err != nil {
				return nil, nil, err
			}
			pols = append(pols, p...)
			vaps = append(vaps, v...)
		} else {
			p, v, err := fsLoad(path)
			if err != nil {
				return nil, nil, err
			}
			pols = append(pols, p...)
			vaps = append(vaps, v...)
		}
	}
	return pols, vaps, nil
}

func fsLoad(path string) ([]kyvernov1.PolicyInterface, []v1alpha1.ValidatingAdmissionPolicy, error) {
	var pols []kyvernov1.PolicyInterface
	var vaps []v1alpha1.ValidatingAdmissionPolicy
	fi, err := os.Stat(filepath.Clean(path))
	if err != nil {
		return nil, nil, err
	}
	if fi.IsDir() {
		files, err := os.ReadDir(path)
		if err != nil {
			return nil, nil, err
		}
		for _, file := range files {
			p, v, err := fsLoad(filepath.Join(path, file.Name()))
			if err != nil {
				return nil, nil, err
			}
			pols = append(pols, p...)
			vaps = append(vaps, v...)
		}
	} else if git.IsYaml(fi) {
		fileBytes, err := os.ReadFile(filepath.Clean(path)) // #nosec G304
		if err != nil {
			return nil, nil, err
		}
		p, v, err := getPolicies(fileBytes)
		if err != nil {
			return nil, nil, err
		}
		pols = append(pols, p...)
		vaps = append(vaps, v...)
	}
	return pols, vaps, nil
}

func httpLoad(path string) ([]kyvernov1.PolicyInterface, []v1alpha1.ValidatingAdmissionPolicy, error) {
	// We accept here that a random URL might be called based on user provided input.
	req, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, path, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to process %v: %v", path, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to process %v: %v", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("failed to process %v: %v", path, err)
	}
	fileBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to process %v: %v", path, err)
	}
	return getPolicies(fileBytes)
}

func gitLoad(fs billy.Filesystem, path string) ([]kyvernov1.PolicyInterface, []v1alpha1.ValidatingAdmissionPolicy, error) {
	file, err := fs.Open(path)
	if err != nil {
		return nil, nil, err
	}
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, nil, err
	}
	return getPolicies(fileBytes)
}

func stdinLoad() ([]kyvernov1.PolicyInterface, []v1alpha1.ValidatingAdmissionPolicy, error) {
	policyStr := ""
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		policyStr = policyStr + scanner.Text() + "\n"
	}
	return getPolicies([]byte(policyStr))
}
