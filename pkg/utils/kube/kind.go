package kube

import "strings"

// GetKindFromGVK - get kind and APIVersion from GVK
func GetKindFromGVK(str string) (apiVersion string, kind string) {
	if strings.Count(str, "/") == 0 {
		return "", str
	}
	splitString := strings.Split(str, "/")
	if strings.Count(str, "/") == 1 {
		return splitString[0], splitString[1]
	}
	if splitString[1] == "*" {
		return "", splitString[2]
	}
	return splitString[0] + "/" + splitString[1], splitString[2]
}

func ContainsKind(list []string, kind string) bool {
	for _, e := range list {
		if _, k := GetKindFromGVK(e); k == kind {
			return true
		}
	}
	return false
}

// SkipSubResources check to skip list of resources which don't have group.
func SkipSubResources(kind string) bool {
	s := []string{"PodExecOptions", "PodAttachOptions", "PodProxyOptions", "ServiceProxyOptions", "NodeProxyOptions"}
	return ContainsKind(s, kind)
}

func GetFormatedKind(str string) (kind string) {
	if strings.Count(str, "/") == 0 {
		return strings.Title(str)
	}
	splitString := strings.Split(str, "/")
	if strings.Count(str, "/") == 1 {
		return splitString[0] + "/" + strings.Title(splitString[1])
	}
	return splitString[0] + "/" + splitString[1] + "/" + strings.Title(splitString[2])
}
