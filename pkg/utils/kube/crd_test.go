package kube

import (
	"errors"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiserver "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ktesting "k8s.io/client-go/testing"
)

func TestCRDsInstalled(t *testing.T) {
	tests := []struct {
		name          string
		crdNames      []string
		existingCRDs  []string
		expectedError bool
		errorContains string
	}{
		{
			name:          "all CRDs exist",
			crdNames:      []string{"clusterpolicies.kyverno.io", "policies.kyverno.io"},
			existingCRDs:  []string{"clusterpolicies.kyverno.io", "policies.kyverno.io"},
			expectedError: false,
		},
		{
			name:          "some CRDs missing",
			crdNames:      []string{"clusterpolicies.kyverno.io", "updaterequests.kyverno.io"},
			existingCRDs:  []string{"clusterpolicies.kyverno.io"},
			expectedError: true,
			errorContains: "updaterequests.kyverno.io",
		},
		{
			name:          "all CRDs missing",
			crdNames:      []string{"updaterequests.kyverno.io", "policyexceptions.policies.kyverno.io"},
			existingCRDs:  []string{},
			expectedError: true,
			errorContains: "updaterequests.kyverno.io",
		},
		{
			name:          "no CRDs to check",
			crdNames:      []string{},
			existingCRDs:  []string{},
			expectedError: false,
		},
		{
			name:          "single CRD exists",
			crdNames:      []string{"clusterpolicies.kyverno.io"},
			existingCRDs:  []string{"clusterpolicies.kyverno.io"},
			expectedError: false,
		},
		{
			name:          "single CRD missing",
			crdNames:      []string{"generatingpolicies.policies.kyverno.io"},
			existingCRDs:  []string{},
			expectedError: true,
			errorContains: "generatingpolicies.policies.kyverno.io",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake client with existing CRDs
			var objects []runtime.Object
			for _, crdName := range tt.existingCRDs {
				crd := &apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: crdName,
					},
				}
				objects = append(objects, crd)
			}

			client := apiextensionsfake.NewSimpleClientset(objects...)

			// Test the function
			err := CRDsInstalled(client, tt.crdNames...)

			if tt.expectedError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errorContains != "" && !containsString(err.Error(), tt.errorContains) {
					t.Errorf("expected error to contain %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestIsCRDInstalled(t *testing.T) {
	tests := []struct {
		name          string
		crdName       string
		crdExists     bool
		clientError   error
		expectedError bool
	}{
		{
			name:          "CRD exists",
			crdName:       "clusterpolicies.kyverno.io",
			crdExists:     true,
			expectedError: false,
		},
		{
			name:          "CRD does not exist",
			crdName:       "nonexistent.kyverno.io",
			crdExists:     false,
			expectedError: true,
		},
		{
			name:          "client error",
			crdName:       "test.kyverno.io",
			clientError:   errors.New("connection error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var client apiserver.Interface

			if tt.clientError != nil {
				// Create a client that returns an error
				fakeClient := &apiextensionsfake.Clientset{}
				fakeClient.AddReactor("get", "customresourcedefinitions", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, tt.clientError
				})
				client = fakeClient
			} else if tt.crdExists {
				// Create a client with the CRD
				crd := &apiextensionsv1.CustomResourceDefinition{
					ObjectMeta: metav1.ObjectMeta{
						Name: tt.crdName,
					},
				}
				client = apiextensionsfake.NewSimpleClientset(crd)
			} else {
				// Create a client without the CRD
				client = apiextensionsfake.NewSimpleClientset()
			}

			err := isCRDInstalled(client, tt.crdName)

			if tt.expectedError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestCRDsInstalledWithAPIServerError(t *testing.T) {
	// Test behavior when API server returns errors
	fakeClient := &apiextensionsfake.Clientset{}
	fakeClient.AddReactor("get", "customresourcedefinitions", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
		getAction := action.(ktesting.GetAction)
		if getAction.GetName() == "existing.kyverno.io" {
			return true, &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{Name: "existing.kyverno.io"},
			}, nil
		}
		return true, nil, apierrors.NewNotFound(schema.GroupResource{
			Group:    "apiextensions.k8s.io",
			Resource: "customresourcedefinitions",
		}, getAction.GetName())
	})

	err := CRDsInstalled(fakeClient, "existing.kyverno.io", "missing.kyverno.io")

	if err == nil {
		t.Errorf("expected error for missing CRD")
		return
	}

	if !containsString(err.Error(), "missing.kyverno.io") {
		t.Errorf("expected error to mention missing CRD, got: %v", err)
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(substr) <= len(s) && func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}()))
}
