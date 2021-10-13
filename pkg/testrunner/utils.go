package testrunner

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// LoadFile loads file in byte buffer
func LoadFile(path string) ([]byte, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, err
	}
	path = filepath.Clean(path)
	// We accept the risk of including a user provided file here.
	return ioutil.ReadFile(path) // #nosec G304
}

var kindToResource = map[string]string{
	"ConfigMap":     "configmaps",
	"Endpoints":     "endpoints",
	"Namespace":     "namespaces",
	"Secret":        "secrets",
	"Service":       "services",
	"Deployment":    "deployments",
	"NetworkPolicy": "networkpolicies",
}

func getResourceFromKind(kind string) string {
	if resource, ok := kindToResource[kind]; ok {
		return resource
	}
	return ""
}

//ConvertToUnstructured converts a resource to unstructured format
func ConvertToUnstructured(data []byte) (*unstructured.Unstructured, error) {
	resource := &unstructured.Unstructured{}
	err := resource.UnmarshalJSON(data)
	if err != nil {
		log.Log.Error(err, "failed to unmarshal resource")
		return nil, err
	}
	return resource, nil
}

func envOr(name, def string) string {
	if v, ok := os.LookupEnv(name); ok {
		return v
	}
	return def
}
