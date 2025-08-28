package v2beta1

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGlobalContextEntryConversion(t *testing.T) {
	// Create a v2alpha1 GlobalContextEntry
	v2alpha1GCE := &kyvernov2alpha1.GlobalContextEntry{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GlobalContextEntry",
			APIVersion: "kyverno.io/v2alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-gce",
		},
		Spec: kyvernov2alpha1.GlobalContextEntrySpec{
			KubernetesResource: &kyvernov2alpha1.KubernetesResource{
				Group:     "apps",
				Version:   "v1",
				Resource:  "deployments",
				Namespace: "default",
			},
			Projections: []kyvernov2alpha1.GlobalContextEntryProjection{
				{
					Name:     "deployment_names",
					JMESPath: "metadata.name",
				},
			},
		},
		Status: kyvernov2alpha1.GlobalContextEntryStatus{
			LastRefreshTime: metav1.Now(),
		},
	}

	// Convert from v2alpha1 to v2beta1
	v2beta1GCE := &GlobalContextEntry{}
	v2beta1GCE.ConvertFromV2Alpha1(v2alpha1GCE)

	// Verify conversion
	if v2beta1GCE.Name != v2alpha1GCE.Name {
		t.Errorf("Expected name %s, got %s", v2alpha1GCE.Name, v2beta1GCE.Name)
	}

	if v2beta1GCE.TypeMeta.APIVersion != "kyverno.io/v2beta1" {
		t.Errorf("Expected APIVersion 'kyverno.io/v2beta1', got %s", v2beta1GCE.TypeMeta.APIVersion)
	}

	if v2beta1GCE.Spec.KubernetesResource.Resource != "deployments" {
		t.Errorf("Expected resource 'deployments', got %s", v2beta1GCE.Spec.KubernetesResource.Resource)
	}

	if len(v2beta1GCE.Spec.Projections) != 1 {
		t.Errorf("Expected 1 projection, got %d", len(v2beta1GCE.Spec.Projections))
	}

	// Convert back from v2beta1 to v2alpha1
	convertedBack := &kyvernov2alpha1.GlobalContextEntry{}
	v2beta1GCE.ConvertToV2Alpha1(convertedBack)

	// Verify round-trip conversion
	if convertedBack.Name != v2alpha1GCE.Name {
		t.Errorf("Round-trip conversion failed: Expected name %s, got %s", v2alpha1GCE.Name, convertedBack.Name)
	}

	if convertedBack.TypeMeta.APIVersion != "kyverno.io/v2alpha1" {
		t.Errorf("Round-trip conversion failed: Expected APIVersion 'kyverno.io/v2alpha1', got %s", convertedBack.TypeMeta.APIVersion)
	}
}

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
