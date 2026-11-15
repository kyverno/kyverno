package kube

import (
	"encoding/json"
	"fmt"
	"strings"

	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
)

// ParseSecretReference resolves a secret name or namespace/name reference.
func ParseSecretReference(secretRef, defaultNamespace string) (namespace, name string) {
	secretRef = strings.TrimPrefix(secretRef, "/")
	if namespace, name, ok := strings.Cut(secretRef, "/"); ok {
		return namespace, name
	}
	return defaultNamespace, secretRef
}

// ScopeSecretReferences confines secret references to one namespace and
// returns canonical namespace/name references.
func ScopeSecretReferences(secretRefs []string, namespace string) ([]string, error) {
	if namespace == "" {
		return nil, fmt.Errorf("policy namespace must not be empty")
	}
	if len(validation.IsDNS1123Label(namespace)) != 0 {
		return nil, fmt.Errorf("policy namespace %q is invalid", namespace)
	}
	if len(secretRefs) == 0 {
		return nil, nil
	}
	scoped := make([]string, len(secretRefs))
	for i, secretRef := range secretRefs {
		secretNamespace, secretName := ParseSecretReference(secretRef, namespace)
		if secretName == "" {
			return nil, fmt.Errorf("secret reference %q has an empty name", secretRef)
		}
		if len(validation.IsDNS1123Subdomain(secretName)) != 0 {
			return nil, fmt.Errorf("secret reference %q has an invalid name", secretRef)
		}
		if secretNamespace != namespace {
			return nil, fmt.Errorf("secret reference %q uses namespace %q instead of policy namespace %q", secretRef, secretNamespace, namespace)
		}
		scoped[i] = secretNamespace + "/" + secretName
	}
	return scoped, nil
}

// RedactSecret masks keys of data and metadata.annotation fields of Secrets.
func RedactSecret(resource *unstructured.Unstructured) (unstructured.Unstructured, error) {
	var secret *corev1.Secret
	data, err := json.Marshal(resource.Object)
	if err != nil {
		return *resource, err
	}
	err = json.Unmarshal(data, &secret)
	if err != nil {
		return *resource, fmt.Errorf("unable to convert object to secret: %w", err)
	}
	stringSecret := struct {
		Data map[string]string `json:"string_data"`
		*corev1.Secret
	}{
		Data:   make(map[string]string),
		Secret: secret,
	}
	for key := range secret.Data {
		secret.Data[key] = []byte("**REDACTED**")
		stringSecret.Data[key] = string(secret.Data[key])
	}
	for key := range secret.Annotations {
		secret.Annotations[key] = "**REDACTED**"
	}
	updateSecret := map[string]interface{}{}
	raw, err := json.Marshal(stringSecret)
	if err != nil {
		return *resource, nil
	}
	err = json.Unmarshal(raw, &updateSecret)
	if err != nil {
		return *resource, fmt.Errorf("unable to convert object from secret: %w", err)
	}
	if secret.Data != nil {
		v := updateSecret["string_data"].(map[string]interface{})
		err = unstructured.SetNestedMap(resource.Object, v, "data")
		if err != nil {
			return *resource, fmt.Errorf("failed to set secret.data: %w", err)
		}
	}
	if secret.Annotations != nil {
		metadata, err := datautils.ToMap(resource.Object["metadata"])
		if err != nil {
			return *resource, fmt.Errorf("unable to convert metadata to map: %w", err)
		}
		updatedMeta := updateSecret["metadata"].(map[string]interface{})
		err = unstructured.SetNestedMap(metadata, updatedMeta["annotations"].(map[string]interface{}), "annotations")
		if err != nil {
			return *resource, fmt.Errorf("failed to set secret.annotations: %w", err)
		}
	}
	return *resource, nil
}
