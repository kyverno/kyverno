package dclient

import (
	"fmt"
	"strings"

	pathvalidation "k8s.io/apimachinery/pkg/api/validation/path"
)

// Kubernetes API requests issued through client-go's dynamic client are built
// by joining path segments (group, version, namespace, resource, name,
// subresources...) and passing them to (*rest.Request).AbsPath, which joins
// them with path.Join - and path.Join resolves "." and ".." segments. A
// caller-supplied argument that is exactly "." or ".." (or otherwise injects
// a "/") is therefore not just a malformed resource/subresource/name, it can
// silently redirect the actual HTTP request to a different resource, a
// different namespace, or even a different API group/version than the one
// that was ever validated (GHSA-4pqm-wqq6-h9q2). Every value that becomes one
// of these path segments must be checked before it reaches the dynamic
// client. This validator is shared rather than per-caller, but it is not a
// complete chokepoint on its own: code reaching the dynamic client directly
// through GetDynamicInterface() has to call it explicitly.

// ValidatePathSegment reports whether value is safe to use as a single
// Kubernetes API URL path segment. It uses the same check client-go's
// dynamic client applies to namespace and name
// (k8s.io/apimachinery/pkg/api/validation/path.IsValidPathSegmentName), which
// rejects "." and ".." and any segment containing "/" or "%". That helper
// does not reject the empty string or "\", so both are checked explicitly
// here.
func ValidatePathSegment(kind, value string) error {
	if value == "" {
		return fmt.Errorf("invalid %s %q: must not be empty", kind, value)
	}
	if strings.Contains(value, "\\") {
		return fmt.Errorf("invalid %s %q: must not contain a path separator", kind, value)
	}
	if problems := pathvalidation.IsValidPathSegmentName(value); len(problems) > 0 {
		return fmt.Errorf("invalid %s %q: %s", kind, value, strings.Join(problems, ", "))
	}
	return nil
}
