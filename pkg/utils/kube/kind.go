package kube

import "strings"

// GetKindFromGVK - get kind and APIVersion from GVK
func GetKindFromGVK(str string) (apiVersion string, kind string) {
	parts := strings.Split(str, "/")
	count := len(parts)
	if count == 2 {
		return parts[0], formatSubresource(parts[1])
	}

	if count == 3 {
		if parts[1] == "*" {
			return "", formatSubresource(parts[2])
		}

		return parts[0] + "/" + parts[1], formatSubresource(parts[2])
	}

	if count == 4 {
		return parts[0] + "/" + parts[1], parts[2] + "/" + parts[3]
	}

	return "", formatSubresource(str)
}

func formatSubresource(s string) string {
	return strings.Replace(s, ".", "/", 1)
}

// GetGroupFromGVK - get group GVK
func GetGroupFromGVK(str string) (group string) {
	parts := strings.Split(str, "/")
	count := len(parts)
	if count == 3 {
		if parts[1] == "*" {
			return parts[0]
		}
	}
	return ""
}

func SplitSubresource(s string) (kind string, subresource string) {
	normalized := strings.Replace(s, ".", "/", 1)
	parts := strings.Split(normalized, "/")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}

	return s, ""
}

func ContainsKind(list []string, kind string) bool {
	for _, e := range list {
		if _, k := GetKindFromGVK(e); k == kind {
			return true
		}
	}
	return false
}

// SkipSubResources skip list of resources which don't have an API group.
func SkipSubResources(kind string) bool {
	s := []string{"PodExecOptions", "PodAttachOptions", "PodProxyOptions", "ServiceProxyOptions", "NodeProxyOptions"}
	return ContainsKind(s, kind)
}
