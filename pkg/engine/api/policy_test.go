package api

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenericPolicy(t *testing.T) {
	kyvernoPolicy := &KyvernoPolicy{
		policy: &kyvernov1.Policy{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "test-policy",
				Namespace:       "default",
				ResourceVersion: "v1",
				Annotations: map[string]string{
					"key": "value",
				},
			},
		},
	}

	validatingAdmissionPolicy := &ValidatingAdmissionPolicy{
		policy: admissionregistrationv1beta1.ValidatingAdmissionPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "test-validating-policy",
				Namespace:       "default",
				ResourceVersion: "v1",
				Annotations: map[string]string{
					"key": "value",
				},
			},
		},
	}

	tests := []struct {
		name                 string
		policy               GenericPolicy
		expectedPolicyType   PolicyType
		expectedAPIVersion   string
		expectedName         string
		expectedNamespace    string
		expectedKind         string
		expectedResourceVer  string
		expectedAnnotations  map[string]string
		expectedIsNamespaced bool
	}{
		{
			name:                 "KyvernoPolicy - GetType",
			policy:               kyvernoPolicy,
			expectedPolicyType:   KyvernoPolicyType,
			expectedAPIVersion:   "kyverno.io/v1",
			expectedName:         "test-policy",
			expectedNamespace:    "default",
			expectedKind:         "Policy",
			expectedResourceVer:  "v1",
			expectedAnnotations:  map[string]string{"key": "value"},
			expectedIsNamespaced: true,
		},
		{
			name:                 "ValidatingAdmissionPolicy - GetType",
			policy:               validatingAdmissionPolicy,
			expectedPolicyType:   ValidatingAdmissionPolicyType,
			expectedAPIVersion:   "admissionregistration.k8s.io/v1beta1",
			expectedName:         "test-validating-policy",
			expectedNamespace:    "default",
			expectedKind:         "ValidatingAdmissionPolicy",
			expectedResourceVer:  "v1",
			expectedAnnotations:  map[string]string{"key": "value"},
			expectedIsNamespaced: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedPolicyType, tt.policy.GetType())
			assert.Equal(t, tt.expectedAPIVersion, tt.policy.GetAPIVersion())
			assert.Equal(t, tt.expectedName, tt.policy.GetName())
			assert.Equal(t, tt.expectedNamespace, tt.policy.GetNamespace())
			assert.Equal(t, tt.expectedKind, tt.policy.GetKind())
			assert.Equal(t, tt.expectedResourceVer, tt.policy.GetResourceVersion())
			assert.Equal(t, tt.expectedAnnotations, tt.policy.GetAnnotations())
			assert.Equal(t, tt.expectedIsNamespaced, tt.policy.IsNamespaced())
		})
	}
}

func TestAsKyvernoPolicy(t *testing.T) {
	kyvernoPolicy := &KyvernoPolicy{
		policy: &kyvernov1.Policy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-policy",
				Namespace: "default",
			},
		},
	}

	validatingAdmissionPolicy := &ValidatingAdmissionPolicy{
		policy: admissionregistrationv1beta1.ValidatingAdmissionPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-validating-policy",
				Namespace: "default",
			},
		},
	}

	tests := []struct {
		name      string
		policy    GenericPolicy
		isKyverno bool
	}{
		{
			name:      "KyvernoPolicy - AsKyvernoPolicy",
			policy:    kyvernoPolicy,
			isKyverno: true,
		},
		{
			name:      "ValidatingAdmissionPolicy - AsKyvernoPolicy",
			policy:    validatingAdmissionPolicy,
			isKyverno: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.isKyverno {
				assert.NotNil(t, tt.policy.AsKyvernoPolicy())
			} else {
				assert.Nil(t, tt.policy.AsKyvernoPolicy())
			}
		})
	}
}

func TestAsValidatingAdmissionPolicy(t *testing.T) {
	kyvernoPolicy := &KyvernoPolicy{
		policy: &kyvernov1.Policy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-policy",
				Namespace: "default",
			},
		},
	}

	validatingAdmissionPolicy := &ValidatingAdmissionPolicy{
		policy: admissionregistrationv1beta1.ValidatingAdmissionPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-validating-policy",
				Namespace: "default",
			},
		},
	}

	tests := []struct {
		name                        string
		policy                      GenericPolicy
		isValidatingAdmissionPolicy bool
	}{
		{
			name:                        "KyvernoPolicy - AsValidatingAdmissionPolicy",
			policy:                      kyvernoPolicy,
			isValidatingAdmissionPolicy: false,
		},
		{
			name:                        "ValidatingAdmissionPolicy - AsValidatingAdmissionPolicy",
			policy:                      validatingAdmissionPolicy,
			isValidatingAdmissionPolicy: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.isValidatingAdmissionPolicy {
				assert.NotNil(t, tt.policy.AsValidatingAdmissionPolicy())
			} else {
				assert.Nil(t, tt.policy.AsValidatingAdmissionPolicy())
			}
		})
	}
}
