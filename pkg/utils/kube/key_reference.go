package kube

import (
	"fmt"
	"strings"
)

// KeySecretReferencePrefix is the cosign scheme for a public key read from a
// Kubernetes Secret, formatted as k8s://<namespace>/<name>.
const KeySecretReferencePrefix = "k8s://"

// IsKeySecretReference reports whether cosign would treat keyRef as a
// Kubernetes Secret reference. The match ignores case and surrounding space so
// a near-miss spelling is confined rather than passed through unchecked.
func IsKeySecretReference(keyRef string) bool {
	keyRef = strings.TrimSpace(keyRef)
	return len(keyRef) >= len(KeySecretReferencePrefix) && strings.EqualFold(keyRef[:len(KeySecretReferencePrefix)], KeySecretReferencePrefix)
}

// ScopeKeyReference confines a cosign key reference to one namespace. Inline
// keys and other schemes, such as KMS URIs, are returned unchanged. A
// k8s://<namespace>/<name> reference must name a Secret in namespace and is
// returned in canonical form.
func ScopeKeyReference(keyRef, namespace string) (string, error) {
	if !IsKeySecretReference(keyRef) {
		return keyRef, nil
	}
	ref := strings.TrimSpace(keyRef)[len(KeySecretReferencePrefix):]
	secretNamespace, secretName, ok := strings.Cut(ref, "/")
	if !ok || secretNamespace == "" {
		return "", fmt.Errorf("key reference %q must use the format %s<namespace>/<name>", keyRef, KeySecretReferencePrefix)
	}
	scoped, err := ScopeSecretReferences([]string{secretNamespace + "/" + secretName}, namespace)
	if err != nil {
		return "", fmt.Errorf("key reference %q: %w", keyRef, err)
	}
	return KeySecretReferencePrefix + scoped[0], nil
}

// ScopeKeySecret confines a keys.secret reference to one namespace and returns
// its cosign key reference. An empty secret namespace resolves to namespace.
func ScopeKeySecret(secretNamespace, secretName, namespace string) (string, error) {
	if secretNamespace == "" {
		secretNamespace = namespace
	}
	scoped, err := ScopeSecretReferences([]string{secretNamespace + "/" + secretName}, namespace)
	if err != nil {
		return "", err
	}
	return KeySecretReferencePrefix + scoped[0], nil
}
