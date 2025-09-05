package v2beta1

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGlobalContextEntryValidation(t *testing.T) {
	gce := &GlobalContextEntry{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-gce",
		},
		Spec: GlobalContextEntrySpec{
			APICall: &ExternalAPICall{
				APICall: kyvernov1.APICall{
					URLPath: "https://example.com/api",
					Method:  "GET",
				},
				RefreshInterval: &metav1.Duration{Duration: metav1.Duration{Duration: 300000000000}.Duration}, // 5 minutes
				RetryLimit:      3,
			},
		},
	}

	errs := gce.Validate()
	if len(errs) != 0 {
		t.Errorf("Expected no validation errors, got %d errors: %v", len(errs), errs)
	}

	// Test validation with both KubernetesResource and APICall set (should fail)
	gce.Spec.KubernetesResource = &KubernetesResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}

	errs = gce.Validate()
	if len(errs) == 0 {
		t.Error("Expected validation errors when both KubernetesResource and APICall are set")
	}
}
