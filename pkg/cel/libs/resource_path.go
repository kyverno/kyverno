package libs

import (
	"strings"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Kubernetes API requests issued through client-go's dynamic client are built
// by joining path segments (group, version, namespace, resource, name,
// subresources...) and passing them to (*rest.Request).AbsPath, which joins
// them with path.Join - and path.Join resolves "." and ".." segments. A CEL
// resource.Get/List/Post argument that is exactly "." or ".." (or otherwise
// injects a "/") is therefore not just a malformed resource/subresource/name,
// it can silently redirect the actual HTTP request to a different resource,
// a different namespace, or even a different API group/version than the one
// that was ever validated (GHSA-4pqm-wqq6-h9q2). validatePathSegment delegates
// to the dclient chokepoint validator so every caller of the dynamic client
// shares one definition of "safe path segment".
func validatePathSegment(kind, value string) error {
	return dclient.ValidatePathSegment(kind, value)
}

// validateGroupVersion rejects a parsed apiVersion whose group or version
// could be used to escape the API group/version path segments. An empty
// group is valid (the legacy core API group) and is not validated further.
func validateGroupVersion(gv schema.GroupVersion) error {
	if gv.Group != "" {
		if err := validatePathSegment("apiVersion group", gv.Group); err != nil {
			return err
		}
	}
	return validatePathSegment("apiVersion version", gv.Version)
}

// validateNamespaceSegment rejects a namespace argument that could be used
// to escape the namespace path segment. An empty namespace is valid (a
// cluster-scoped resource) and is not validated further.
func validateNamespaceSegment(namespace string) error {
	if namespace == "" {
		return nil
	}
	return validatePathSegment("namespace", namespace)
}

// splitResourcePath splits a CEL "resource" argument of the form
// "resource[/subresource[/subresource...]]" and validates every resulting
// path segment, so neither the resource name nor a subresource can smuggle
// traversal or an otherwise-unsafe path component.
func splitResourcePath(resource string) (string, []string, error) {
	parts := strings.Split(resource, "/")
	if err := validatePathSegment("resource", parts[0]); err != nil {
		return "", nil, err
	}
	subresources := parts[1:]
	for _, s := range subresources {
		if err := validatePathSegment("subresource", s); err != nil {
			return "", nil, err
		}
	}
	return parts[0], subresources, nil
}
