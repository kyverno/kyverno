package kube

import (
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

var versionRegex = regexp.MustCompile(`^v\d((alpha|beta)\d)?|\*$`)

// GetKindFromGVK - get kind and APIVersion from GVK
func GetKindFromGVK(str string) (string, string) {
	parts := strings.Split(str, "/")
	switch len(parts) {
	case 1:
		return "", formatSubresource(str)
	case 2:
		if parts[0] == "*" && parts[1] == "*" {
			return "", parts[0] + "/" + parts[1]
		}
		if versionRegex.MatchString(parts[0]) {
			return parts[0], formatSubresource(parts[1])
		}
		return "", parts[0] + "/" + parts[1]
	case 3:
		if versionRegex.MatchString(parts[0]) {
			return parts[0], parts[1] + "/" + parts[2]
		}
		return parts[0] + "/" + parts[1], formatSubresource(parts[2])
	case 4:
		return parts[0] + "/" + parts[1], parts[2] + "/" + parts[3]
	default:
		return "", ""
	}
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
