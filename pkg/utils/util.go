package utils

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/golang/glog"

	"github.com/minio/minio/pkg/wildcard"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type K8Resource struct {
	Kind      string //TODO: as we currently only support one GVK version, we use the kind only. But if we support multiple GVK, then GV need to be added
	Namespace string
	Name      string
}

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

//SkipFilteredResourcesReq checks if request is to be skipped based on filtered kinds
func SkipFilteredResourcesReq(request *v1beta1.AdmissionRequest, filterK8Resources []K8Resource) bool {
	kind := request.Kind.Kind
	namespace := request.Namespace
	name := request.Name
	for _, r := range filterK8Resources {
		if wildcard.Match(r.Kind, kind) && wildcard.Match(r.Namespace, namespace) && wildcard.Match(r.Name, name) {
			return true
		}
	}
	return false
}

//SkipFilteredResources checks if the resource is to be skipped based on filtered kinds
func SkipFilteredResources(kind, namespace, name string, filterK8Resources []K8Resource) bool {
	for _, r := range filterK8Resources {
		if wildcard.Match(r.Kind, kind) && wildcard.Match(r.Namespace, namespace) && wildcard.Match(r.Name, name) {
			return true
		}
	}
	return false
}

//ParseKinds parses the kinds if a single string contains comma seperated kinds
// {"1,2,3","4","5"} => {"1","2","3","4","5"}
func ParseKinds(list string) []K8Resource {
	resources := []K8Resource{}
	var resource K8Resource
	re := regexp.MustCompile(`\[([^\[\]]*)\]`)
	submatchall := re.FindAllString(list, -1)
	for _, element := range submatchall {
		element = strings.Trim(element, "[")
		element = strings.Trim(element, "]")
		elements := strings.Split(element, ",")
		//TODO: wildcards for namespace and name
		if len(elements) == 0 {
			continue
		}
		if len(elements) == 3 {
			resource = K8Resource{Kind: elements[0], Namespace: elements[1], Name: elements[2]}
		}
		if len(elements) == 2 {
			resource = K8Resource{Kind: elements[0], Namespace: elements[1]}
		}
		if len(elements) == 1 {
			resource = K8Resource{Kind: elements[0]}
		}
		resources = append(resources, resource)
	}
	return resources
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
	if !check("ClusterPolicy") || !check("ClusterPolicyViolation") {
		return false
	}
	return true
}
