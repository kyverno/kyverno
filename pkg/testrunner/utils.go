package testrunner

import (
	"io/ioutil"
	"os"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	defaultYamlSeparator = "---"
	projectPath          = "src/github.com/nirmata/kyverno"
)

// LoadFile loads file in byte buffer
func LoadFile(path string) ([]byte, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, err
	}
	return ioutil.ReadFile(path)
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

func ConvertToUnstructured(data []byte) (*unstructured.Unstructured, error) {
	resource := &unstructured.Unstructured{}
	err := resource.UnmarshalJSON(data)
	if err != nil {
		glog.V(4).Infof("failed to unmarshall resource: %v", err)
		return nil, err
	}
	return resource, nil
}
