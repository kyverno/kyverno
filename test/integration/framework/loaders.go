package framework

import (
	"fmt"
	"os"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	extyaml "github.com/kyverno/kyverno/ext/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	sigsyaml "sigs.k8s.io/yaml"
)

// LoadGeneratingPolicy reads a single-document YAML file and decodes it into a
// GeneratingPolicy. It fails the test on a missing file, an empty or
// multi-document file, a decode error, or a policy with no name, so a malformed
// fixture surfaces immediately instead of as a confusing downstream failure.
func LoadGeneratingPolicy(t *testing.T, path string) *policiesv1beta1.GeneratingPolicy {
	t.Helper()
	policy := &policiesv1beta1.GeneratingPolicy{}
	if err := decodeTypedResource(path, policy, "GeneratingPolicy"); err != nil {
		t.Fatalf("%v", err)
	}
	return policy
}

// LoadResource reads a single-document YAML file and decodes it into obj (for
// example a *corev1.Secret or *corev1.ConfigMap). Same failure guarantees as
// LoadGeneratingPolicy. obj must be a pointer whose apiVersion/kind matches the
// document.
func LoadResource(t *testing.T, path string, obj client.Object) {
	t.Helper()
	if err := decodeSingleResource(path, obj); err != nil {
		t.Fatalf("%v", err)
	}
}

// decodeTypedResource decodes into obj and verifies its kind matches wantKind.
// The kind check matters because sigsyaml.Unmarshal is lenient: any document
// with a metadata.name decodes into a typed policy with all spec fields zeroed
// (both share metav1.ObjectMeta), so without this guard LoadGeneratingPolicy
// would silently accept a Secret fixture and return an empty, no-op policy.
func decodeTypedResource(path string, obj client.Object, wantKind string) error {
	if err := decodeSingleResource(path, obj); err != nil {
		return err
	}
	if got := obj.GetObjectKind().GroupVersionKind().Kind; got != wantKind {
		return fmt.Errorf("load %s: expected kind %s, got %q", path, wantKind, got)
	}
	return nil
}

// decodeSingleResource holds the decode logic and returns an error rather than
// failing a test, so the loaders can be unit tested directly. It asserts the
// file holds exactly one YAML document (sigsyaml.Unmarshal would otherwise
// silently drop everything after the first) and that the decoded object has a
// name (a guard against returning a silent zero-value object on a bad fixture).
func decodeSingleResource(path string, obj client.Object) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("load %s: %w", path, err)
	}
	docs, err := extyaml.SplitDocuments(data)
	if err != nil {
		return fmt.Errorf("load %s: %w", path, err)
	}
	switch len(docs) {
	case 0:
		return fmt.Errorf("load %s: file is empty", path)
	case 1:
		// expected
	default:
		return fmt.Errorf("load %s: file has %d documents, single-document loaders expect exactly 1", path, len(docs))
	}
	if err := sigsyaml.Unmarshal(docs[0], obj); err != nil {
		return fmt.Errorf("load %s: %w", path, err)
	}
	if obj.GetName() == "" {
		return fmt.Errorf("load %s: decoded object has no metadata.name (empty or invalid fixture?)", path)
	}
	return nil
}
