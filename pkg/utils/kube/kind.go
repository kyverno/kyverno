package kube

import (
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GetKindFromGVK - get kind and APIVersion from GVK
func GetKindFromGVK(str string) (groupVersion string, kind string) {
	parts := strings.Split(str, "/")
	count := len(parts)
	versionRegex := regexp.MustCompile(`v\d((alpha|beta)\d)?`)

	if count == 2 {
		if versionRegex.MatchString(parts[0]) || parts[0] == "*" {
			return parts[0], formatSubresource(parts[1])
		} else {
			return "", parts[0] + "/" + parts[1]
		}
	} else if count == 3 {
		if versionRegex.MatchString(parts[0]) || parts[0] == "*" {
			return parts[0], parts[1] + "/" + parts[2]
		} else {
			return parts[0] + "/" + parts[1], formatSubresource(parts[2])
		}
	} else if count == 4 {
		return parts[0] + "/" + parts[1], parts[2] + "/" + parts[3]
	}
	return "", formatSubresource(str)
}

func formatSubresource(s string) string {
	return strings.Replace(s, ".", "/", 1)
}

// SplitSubresource - split subresource from kind
func SplitSubresource(s string) (kind string, subresource string) {
	parts := strings.Split(s, "/")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}

	return s, ""
}

// ContainsKind - check if kind is in list
func ContainsKind(list []string, kind string) bool {
	for _, e := range list {
		_, k := GetKindFromGVK(e)
		k, _ = SplitSubresource(k)
		if k == kind {
			return true
		}
	}
	return false
}

// GroupVersionMatches - check if the given group version matches the server resource group version.
// If the group version contains a wildcard, it will match any version, but the group must match. Returns false if the
// supplied group version is empty, that condition should be checked before calling this function.
func GroupVersionMatches(groupVersion, serverResourceGroupVersion string) bool {
	if strings.Contains(groupVersion, "*") {
		return strings.HasPrefix(serverResourceGroupVersion, strings.TrimSuffix(groupVersion, "*"))
	}

	gv, err := schema.ParseGroupVersion(groupVersion)
	if err == nil {
		serverResourceGV, _ := schema.ParseGroupVersion(serverResourceGroupVersion)
		return gv.Group == serverResourceGV.Group && gv.Version == serverResourceGV.Version
	}

	return false
}

// IsSubresource returns true if the resource is a subresource
func IsSubresource(resourceName string) bool {
	return strings.Contains(resourceName, "/")
}
