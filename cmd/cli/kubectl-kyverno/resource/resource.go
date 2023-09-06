package resource

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/source"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/scheme"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	yamlutils "github.com/kyverno/kyverno/pkg/utils/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func GetUnstructuredResources(resourceBytes []byte) ([]*unstructured.Unstructured, error) {
	var resources []*unstructured.Unstructured
	documents, err := yamlutils.SplitDocuments(resourceBytes)
	if err != nil {
		return nil, err
	}
	for _, document := range documents {
		resource, err := YamlToUnstructured(document)
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

func YamlToUnstructured(resourceYaml []byte) (*unstructured.Unstructured, error) {
	decode := scheme.Codecs.UniversalDeserializer().Decode
	_, metaData, decodeErr := decode(resourceYaml, nil, nil)
	if decodeErr != nil {
		if !strings.Contains(decodeErr.Error(), "no kind") {
			return nil, decodeErr
		}
	}
	resourceJSON, err := yaml.YAMLToJSON(resourceYaml)
	if err != nil {
		return nil, err
	}
	resource, err := kubeutils.BytesToUnstructured(resourceJSON)
	if err != nil {
		return nil, err
	}
	if decodeErr == nil {
		resource.SetGroupVersionKind(*metaData)
	}
	if resource.GetNamespace() == "" {
		resource.SetNamespace("default")
	}
	return resource, nil
}

func GetResourceFromPath(fs billy.Filesystem, path string) (*unstructured.Unstructured, error) {
	var resourceBytes []byte
	if fs == nil {
		data, err := GetFileBytes(path)
		if err != nil {
			return nil, err
		}
		resourceBytes = data
	} else {
		file, err := fs.Open(path)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			return nil, err
		}
		resourceBytes = data
	}
	resources, err := GetUnstructuredResources(resourceBytes)
	if err != nil {
		return nil, err
	}
	if len(resources) != 1 {
		return nil, fmt.Errorf("exactly one resource expected, found %d", len(resources))
	}
	return resources[0], nil
}

func GetFileBytes(path string) ([]byte, error) {
	if source.IsHttp(path) {
		// We accept here that a random URL might be called based on user provided input.
		req, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, err
		}
		file, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return file, nil
	} else {
		path = filepath.Clean(path)
		// We accept the risk of including a user provided file here.
		file, err := os.ReadFile(path) // #nosec G304
		if err != nil {
			return nil, err
		}
		return file, nil
	}
}
