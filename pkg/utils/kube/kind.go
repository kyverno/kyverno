package kube

import (
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

var versionRegex = regexp.MustCompile(`^(v\d((alpha|beta)\d)?|\*)$`)

func ParseKindSelector(input string) (string, string, string, string) {
	parts := strings.Split(input, "/")
	if len(parts) > 0 {
		parts = append(parts[:len(parts)-1], strings.Split(parts[len(parts)-1], ".")...)
	}
	switch len(parts) {
	case 1:
		// we have only kind
		return "*", "*", parts[0], ""
	case 2:
		// `*/*` means all resources and subresources
		if parts[0] == "*" && parts[1] == "*" {
			return "*", "*", "*", "*"
		}
		// detect the `*/subresource` case when part[1] is all lowercase
		if parts[0] == "*" && strings.ToLower(parts[1]) == parts[1] {
			return "*", "*", parts[0], parts[1]
		}
		// if the first part is `*` or a version we have version/kind
		if versionRegex.MatchString(parts[0]) {
			return "*", parts[0], parts[1], ""
		}
		// we have kind/subresource
		return "*", "*", parts[0], parts[1]
	case 3:
		// if the first part is `*` or a version we have version/kind/subresource
		if versionRegex.MatchString(parts[0]) {
			return "*", parts[0], parts[1], parts[2]
		}
		// we have group/version/kind
		return parts[0], parts[1], parts[2], ""
	case 4:
		// we have group/version/kind/subresource
		return parts[0], parts[1], parts[2], parts[3]
	default:
		return "", "", "", ""
	}
}

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
