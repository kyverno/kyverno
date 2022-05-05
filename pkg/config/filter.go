package config

import (
	"regexp"
	"strings"
)

type filter struct {
	Kind      string //TODO: as we currently only support one GVK version, we use the kind only. But if we support multiple GVK, then GV need to be added
	Namespace string
	Name      string
}

// ParseKinds parses the kinds if a single string contains comma separated kinds
// {"1,2,3","4","5"} => {"1","2","3","4","5"}
func parseKinds(list string) []filter {
	resources := []filter{}
	var resource filter
	re := regexp.MustCompile(`\[([^\[\]]*)\]`)
	submatchall := re.FindAllString(list, -1)
	for _, element := range submatchall {
		element = strings.Trim(element, "[")
		element = strings.Trim(element, "]")
		elements := strings.Split(element, ",")
		if len(elements) == 0 {
			continue
		}
		if len(elements) == 3 {
			resource = filter{Kind: elements[0], Namespace: elements[1], Name: elements[2]}
		}
		if len(elements) == 2 {
			resource = filter{Kind: elements[0], Namespace: elements[1]}
		}
		if len(elements) == 1 {
			resource = filter{Kind: elements[0]}
		}
		resources = append(resources, resource)
	}
	return resources
}
