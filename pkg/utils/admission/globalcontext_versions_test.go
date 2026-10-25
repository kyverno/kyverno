package admission

import (
	"fmt"
	"testing"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
)

func TestGlobalContextEntryDecodesToStableVersion(t *testing.T) {
	t.Parallel()

	for _, version := range []string{"v2", "v2beta1", "v2alpha1"} {
		t.Run(version, func(t *testing.T) {
			t.Parallel()

			raw := []byte(fmt.Sprintf(`{"apiVersion":"kyverno.io/%s","kind":"GlobalContextEntry","metadata":{"name":"cache"},"spec":{"kubernetesResource":{"version":"v1","resource":"configmaps"}}}`, version))
			entry, err := UnmarshalGlobalContextEntry(raw)
			if err != nil {
				t.Fatal(err)
			}
			stable, ok := any(entry).(*kyvernov2.GlobalContextEntry)
			if !ok {
				t.Fatalf("admission decoded %s as %T, want *v2.GlobalContextEntry", version, entry)
			}
			if stable.Name != "cache" || stable.Spec.KubernetesResource.Resource != "configmaps" {
				t.Fatalf("admission changed entry fields: %#v", stable)
			}
		})
	}
}
