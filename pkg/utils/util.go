package utils

import (
	"reflect"

	"github.com/golang/glog"

	"github.com/minio/minio/pkg/wildcard"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

//Contains Check if strint is contained in a list of string
func contains(list []string, element string, fn func(string, string) bool) bool {
	for _, e := range list {
		if fn(e, element) {
			return true
		}
	}
	return false
}

//ContainsNamepace check if namespace satisfies any list of pattern(regex)
func ContainsNamepace(patterns []string, ns string) bool {
	return contains(patterns, ns, compareNamespaces)
}

//ContainsString check if the string is contains in a list
func ContainsString(list []string, element string) bool {
	return contains(list, element, compareString)
}

func compareNamespaces(pattern, ns string) bool {
	return wildcard.Match(pattern, ns)
}

func compareString(str, name string) bool {
	return str == name
}

//NewKubeClient returns a new kubernetes client
func NewKubeClient(config *rest.Config) (kubernetes.Interface, error) {
	kclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return kclient, nil
}

//Btoi converts boolean to int
func Btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

//CRDInstalled to check if the CRD is installed or not
func CRDInstalled(discovery client.IDiscovery) bool {
	check := func(kind string) bool {
		gvr := discovery.GetGVRFromKind(kind)
		if reflect.DeepEqual(gvr, (schema.GroupVersionResource{})) {
			glog.Errorf("%s CRD not installed", kind)
			return false
		}
		glog.Infof("CRD %s found ", kind)
		return true
	}
	if !check("ClusterPolicy") || !check("ClusterPolicyViolation") || !check("PolicyViolation") {
		return false
	}
	return true
}
