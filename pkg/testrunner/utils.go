package testrunner

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/golang/glog"
	client "github.com/nirmata/kyverno/pkg/dclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kscheme "k8s.io/client-go/kubernetes/scheme"
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

type resourceInfo struct {
	rawResource []byte
	gvk         *metav1.GroupVersionKind
}

func (ri resourceInfo) isSame(other resourceInfo) bool {
	// compare gvk
	if *ri.gvk != *other.gvk {
		return false
	}
	// compare rawResource
	return bytes.Equal(ri.rawResource, other.rawResource)
}

// compare patched resources
func compareResource(er *resourceInfo, pr *resourceInfo) bool {
	if !er.isSame(*pr) {
		return false
	}
	return true
}

func createClient(resources []*resourceInfo) (*client.Client, error) {
	scheme := runtime.NewScheme()
	objects := []runtime.Object{}
	// registered group versions
	regResources := []schema.GroupVersionResource{}

	for _, r := range resources {
		// registered gvr
		gv := schema.GroupVersion{Group: r.gvk.Group, Version: r.gvk.Version}
		gvr := gv.WithResource(getResourceFromKind(r.gvk.Kind))
		regResources = append(regResources, gvr)
		decode := kscheme.Codecs.UniversalDeserializer().Decode
		obj, _, err := decode([]byte(r.rawResource), nil, nil)
		rdata, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&obj)
		if err != nil {
			glog.Errorf("failed to load resource. err %s", err)
		}
		unstr := unstructured.Unstructured{Object: rdata}
		objects = append(objects, &unstr)
	}
	// Mock Client
	c, err := client.NewMockClient(scheme, objects...)
	if err != nil {
		return nil, err
	}
	c.SetDiscovery(client.NewFakeDiscoveryClient(regResources))

	return c, nil
}

var kindToResource = map[string]string{
	"ConfigMap":     "configmaps",
	"Endpoints":     "endpoints",
	"Namespace":     "namespaces",
	"Secret":        "secrets",
	"Deployment":    "deployments",
	"NetworkPolicy": "networkpolicies",
}

func getResourceFromKind(kind string) string {
	if resource, ok := kindToResource[kind]; ok {
		return resource
	}
	return ""
}

//ParseNameFromObject extracts resource name from JSON obj
func ParseNameFromObject(bytes []byte) string {
	var objectJSON map[string]interface{}
	json.Unmarshal(bytes, &objectJSON)

	meta := objectJSON["metadata"].(map[string]interface{})

	if name, ok := meta["name"].(string); ok {
		return name
	}
	return ""
}

// ParseNamespaceFromObject extracts the namespace from the JSON obj
func ParseNamespaceFromObject(bytes []byte) string {
	var objectJSON map[string]interface{}
	json.Unmarshal(bytes, &objectJSON)

	meta := objectJSON["metadata"].(map[string]interface{})

	if namespace, ok := meta["namespace"].(string); ok {
		return namespace
	}
	return ""
}
