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
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/source"
	"github.com/kyverno/kyverno/pkg/utils/git"
	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
	"k8s.io/api/admissionregistration/v1alpha1"
)

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
		p, v, err := yamlutils.GetPolicy(fileBytes)
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
	return yamlutils.GetPolicy(fileBytes)
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
	return yamlutils.GetPolicy(fileBytes)
}

func stdinLoad() ([]kyvernov1.PolicyInterface, []v1alpha1.ValidatingAdmissionPolicy, error) {
	policyStr := ""
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		policyStr = policyStr + scanner.Text() + "\n"
	}
	return yamlutils.GetPolicy([]byte(policyStr))
}
