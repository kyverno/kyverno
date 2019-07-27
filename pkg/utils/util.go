package utils

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/minio/minio/pkg/wildcard"
	"k8s.io/api/admission/v1beta1"
)

func Contains(list []string, element string) bool {
	for _, e := range list {
		if e == element {
			return true
		}
	}
	return false
}

type K8Resource struct {
	Kind      string //TODO: as we currently only support one GVK version, we use the kind only. But if we support multiple GVK, then GV need to be added
	Namespace string
	Name      string
}

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

func SkipFilteredResources(kind, namespace, name string, filterK8Resources []K8Resource) bool {
	for _, r := range filterK8Resources {
		if wildcard.Match(r.Kind, kind) && wildcard.Match(r.Namespace, namespace) && wildcard.Match(r.Name, name) {
			return true
		}
	}
	return false
}

//parseKinds parses the kinds if a single string contains comma seperated kinds
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
		fmt.Println(resource)
		resources = append(resources, resource)
	}
	return resources
}
