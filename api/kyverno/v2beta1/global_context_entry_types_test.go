package v2beta1

import (
	"testing"
	"time"

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

func TestGlobalContextEntrySpec(t *testing.T) {
	t.Run("IsAPICall", func(t *testing.T) {
		spec := &GlobalContextEntrySpec{
			APICall: &ExternalAPICall{},
		}
		if !spec.IsAPICall() {
			t.Error("Expected IsAPICall to return true when APICall is set")
		}

		spec.APICall = nil
		if spec.IsAPICall() {
			t.Error("Expected IsAPICall to return false when APICall is nil")
		}
	})

	t.Run("IsResource", func(t *testing.T) {
		spec := &GlobalContextEntrySpec{
			KubernetesResource: &KubernetesResource{},
		}
		if !spec.IsResource() {
			t.Error("Expected IsResource to return true when KubernetesResource is set")
		}

		spec.KubernetesResource = nil
		if spec.IsResource() {
			t.Error("Expected IsResource to return false when KubernetesResource is nil")
		}
	})

	t.Run("Validate", func(t *testing.T) {
		// Test neither APICall nor KubernetesResource set
		spec := &GlobalContextEntrySpec{}
		errs := spec.Validate(nil, "test")
		if len(errs) == 0 {
			t.Error("Expected validation error when neither APICall nor KubernetesResource is set")
		}

		// Test both APICall and KubernetesResource set
		spec.APICall = &ExternalAPICall{
			RefreshInterval: &metav1.Duration{Duration: 5 * time.Minute},
		}
		spec.KubernetesResource = &KubernetesResource{}
		errs = spec.Validate(nil, "test")
		if len(errs) == 0 {
			t.Error("Expected validation error when both APICall and KubernetesResource are set")
		}

		// Test valid KubernetesResource only
		spec.APICall = nil
		spec.KubernetesResource = &KubernetesResource{
			Version:  "v1",
			Resource: "pods",
		}
		spec.Projections = []GlobalContextEntryProjection{
			{Name: "test-proj", JMESPath: "metadata.name"},
		}
		errs = spec.Validate(nil, "test")
		if len(errs) != 0 {
			t.Errorf("Expected no validation errors for valid KubernetesResource, got: %v", errs)
		}

		// Test valid APICall only
		spec.KubernetesResource = nil
		spec.APICall = &ExternalAPICall{
			APICall: kyvernov1.APICall{
				URLPath: "https://example.com/api",
			},
			RefreshInterval: &metav1.Duration{Duration: 5 * time.Minute},
		}
		errs = spec.Validate(nil, "test")
		if len(errs) != 0 {
			t.Errorf("Expected no validation errors for valid APICall, got: %v", errs)
		}
	})
}

func TestKubernetesResourceValidation(t *testing.T) {
	t.Run("ValidResource", func(t *testing.T) {
		kr := &KubernetesResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
		}
		errs := kr.Validate(nil)
		if len(errs) != 0 {
			t.Errorf("Expected no validation errors, got: %v", errs)
		}
	})

	t.Run("CoreGroupResource", func(t *testing.T) {
		kr := &KubernetesResource{
			Version:  "v1",
			Resource: "pods",
		}
		errs := kr.Validate(nil)
		if len(errs) != 0 {
			t.Errorf("Expected no validation errors for core group resource, got: %v", errs)
		}
	})

	t.Run("MissingVersion", func(t *testing.T) {
		kr := &KubernetesResource{
			Group:    "apps",
			Resource: "deployments",
		}
		errs := kr.Validate(nil)
		if len(errs) == 0 {
			t.Error("Expected validation error for missing version")
		}
	})

	t.Run("MissingResource", func(t *testing.T) {
		kr := &KubernetesResource{
			Group:   "apps",
			Version: "v1",
		}
		errs := kr.Validate(nil)
		if len(errs) == 0 {
			t.Error("Expected validation error for missing resource")
		}
	})

	t.Run("MissingGroupForNonCore", func(t *testing.T) {
		kr := &KubernetesResource{
			Version:  "v1beta1",
			Resource: "customresources",
		}
		errs := kr.Validate(nil)
		if len(errs) == 0 {
			t.Error("Expected validation error for missing group in non-core resource")
		}
	})
}

func TestExternalAPICallValidation(t *testing.T) {
	t.Run("ValidAPICall", func(t *testing.T) {
		apiCall := &ExternalAPICall{
			APICall: kyvernov1.APICall{
				URLPath: "https://example.com/api",
				Method:  "GET",
			},
			RefreshInterval: &metav1.Duration{Duration: 5 * time.Minute},
			RetryLimit:      3,
		}
		errs := apiCall.Validate(nil)
		if len(errs) != 0 {
			t.Errorf("Expected no validation errors, got: %v", errs)
		}
	})

	t.Run("ValidServiceCall", func(t *testing.T) {
		apiCall := &ExternalAPICall{
			APICall: kyvernov1.APICall{
				Service: &kyvernov1.ServiceCall{
					URL:      "http://my-service.default.svc.cluster.local",
					CABundle: "dGVzdA==",
				},
				Method: "GET",
			},
			RefreshInterval: &metav1.Duration{Duration: 10 * time.Minute},
		}
		errs := apiCall.Validate(nil)
		if len(errs) != 0 {
			t.Errorf("Expected no validation errors, got: %v", errs)
		}
	})

	t.Run("ZeroRefreshInterval", func(t *testing.T) {
		apiCall := &ExternalAPICall{
			APICall: kyvernov1.APICall{
				URLPath: "https://example.com/api",
			},
			RefreshInterval: &metav1.Duration{Duration: 0},
		}
		errs := apiCall.Validate(nil)
		if len(errs) == 0 {
			t.Error("Expected validation error for zero refresh interval")
		}
	})

	t.Run("BothServiceAndURLPath", func(t *testing.T) {
		apiCall := &ExternalAPICall{
			APICall: kyvernov1.APICall{
				URLPath: "https://example.com/api",
				Service: &kyvernov1.ServiceCall{
					URL: "http://my-service",
				},
			},
			RefreshInterval: &metav1.Duration{Duration: 5 * time.Minute},
		}
		errs := apiCall.Validate(nil)
		if len(errs) == 0 {
			t.Error("Expected validation error for both Service and URLPath set")
		}
	})

	t.Run("NeitherServiceNorURLPath", func(t *testing.T) {
		apiCall := &ExternalAPICall{
			RefreshInterval: &metav1.Duration{Duration: 5 * time.Minute},
		}
		errs := apiCall.Validate(nil)
		if len(errs) == 0 {
			t.Error("Expected validation error for neither Service nor URLPath set")
		}
	})

	t.Run("NilRefreshInterval", func(t *testing.T) {
		apiCall := &ExternalAPICall{
			APICall: kyvernov1.APICall{
				URLPath: "https://example.com/api",
			},
			RefreshInterval: nil,
		}
		errs := apiCall.Validate(nil)
		if len(errs) == 0 {
			t.Error("Expected validation error for nil refresh interval")
		}
	})

	t.Run("DataWithoutPOST", func(t *testing.T) {
		data := []kyvernov1.RequestData{{Key: "test", Value: nil}}
		apiCall := &ExternalAPICall{
			APICall: kyvernov1.APICall{
				URLPath: "https://example.com/api",
				Method:  "GET",
				Data:    data,
			},
			RefreshInterval: &metav1.Duration{Duration: 5 * time.Minute},
		}
		errs := apiCall.Validate(nil)
		if len(errs) == 0 {
			t.Error("Expected validation error for data with non-POST method")
		}
	})

	t.Run("DataWithPOST", func(t *testing.T) {
		data := []kyvernov1.RequestData{{Key: "test", Value: nil}}
		apiCall := &ExternalAPICall{
			APICall: kyvernov1.APICall{
				URLPath: "https://example.com/api",
				Method:  "POST",
				Data:    data,
			},
			RefreshInterval: &metav1.Duration{Duration: 5 * time.Minute},
		}
		errs := apiCall.Validate(nil)
		if len(errs) != 0 {
			t.Errorf("Expected no validation errors for data with POST method, got: %v", errs)
		}
	})
}

func TestGlobalContextEntryProjectionValidation(t *testing.T) {
	t.Run("ValidProjection", func(t *testing.T) {
		proj := &GlobalContextEntryProjection{
			Name:     "test-projection",
			JMESPath: "metadata.name",
		}
		errs := proj.Validate(nil, "gce-name")
		if len(errs) != 0 {
			t.Errorf("Expected no validation errors, got: %v", errs)
		}
	})

	t.Run("MissingName", func(t *testing.T) {
		proj := &GlobalContextEntryProjection{
			JMESPath: "metadata.name",
		}
		errs := proj.Validate(nil, "gce-name")
		if len(errs) == 0 {
			t.Error("Expected validation error for missing name")
		}
	})

	t.Run("NameEqualsGCEName", func(t *testing.T) {
		proj := &GlobalContextEntryProjection{
			Name:     "gce-name",
			JMESPath: "metadata.name",
		}
		errs := proj.Validate(nil, "gce-name")
		if len(errs) == 0 {
			t.Error("Expected validation error for name equal to GCE name")
		}
	})

	t.Run("MissingJMESPath", func(t *testing.T) {
		proj := &GlobalContextEntryProjection{
			Name: "test-projection",
		}
		errs := proj.Validate(nil, "gce-name")
		if len(errs) == 0 {
			t.Error("Expected validation error for missing JMESPath")
		}
	})

	t.Run("InvalidJMESPath", func(t *testing.T) {
		proj := &GlobalContextEntryProjection{
			Name:     "test-projection",
			JMESPath: "invalid[syntax",
		}
		errs := proj.Validate(nil, "gce-name")
		if len(errs) == 0 {
			t.Error("Expected validation error for invalid JMESPath syntax")
		}
	})
}

func TestGlobalContextEntryStatus(t *testing.T) {
	t.Run("SetReady_True", func(t *testing.T) {
		status := &GlobalContextEntryStatus{}
		status.SetReady(true, "Successfully loaded")

		if !status.IsReady() {
			t.Error("Expected IsReady to return true")
		}

		if len(status.Conditions) != 1 {
			t.Errorf("Expected 1 condition, got %d", len(status.Conditions))
		}

		condition := status.Conditions[0]
		if condition.Type != GlobalContextEntryConditionReady {
			t.Errorf("Expected condition type %s, got %s", GlobalContextEntryConditionReady, condition.Type)
		}
		if condition.Status != metav1.ConditionTrue {
			t.Errorf("Expected condition status True, got %s", condition.Status)
		}
		if condition.Reason != GlobalContextEntryReasonSucceeded {
			t.Errorf("Expected reason %s, got %s", GlobalContextEntryReasonSucceeded, condition.Reason)
		}
		if condition.Message != "Successfully loaded" {
			t.Errorf("Expected message 'Successfully loaded', got %s", condition.Message)
		}

		if status.Ready != nil {
			t.Error("Expected Ready field to be nil after SetReady")
		}
	})

	t.Run("SetReady_False", func(t *testing.T) {
		status := &GlobalContextEntryStatus{}
		status.SetReady(false, "Failed to load")

		if status.IsReady() {
			t.Error("Expected IsReady to return false")
		}

		condition := status.Conditions[0]
		if condition.Status != metav1.ConditionFalse {
			t.Errorf("Expected condition status False, got %s", condition.Status)
		}
		if condition.Reason != GlobalContextEntryReasonFailed {
			t.Errorf("Expected reason %s, got %s", GlobalContextEntryReasonFailed, condition.Reason)
		}
	})

	t.Run("UpdateRefreshTime", func(t *testing.T) {
		status := &GlobalContextEntryStatus{}
		originalTime := status.LastRefreshTime

		status.UpdateRefreshTime()

		if status.LastRefreshTime.Equal(&originalTime) {
			t.Error("Expected LastRefreshTime to be updated")
		}
		if status.LastRefreshTime.IsZero() {
			t.Error("Expected LastRefreshTime to be set to current time")
		}
	})

	t.Run("IsReady_NoConditions", func(t *testing.T) {
		status := &GlobalContextEntryStatus{}

		if status.IsReady() {
			t.Error("Expected IsReady to return false when no conditions are set")
		}
	})

	t.Run("IsReady_WrongConditionType", func(t *testing.T) {
		status := &GlobalContextEntryStatus{
			Conditions: []metav1.Condition{
				{
					Type:   "SomeOtherCondition",
					Status: metav1.ConditionTrue,
				},
			},
		}

		if status.IsReady() {
			t.Error("Expected IsReady to return false when no Ready condition exists")
		}
	})
}
