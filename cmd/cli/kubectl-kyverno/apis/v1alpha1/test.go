package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope="Cluster"

// Test declares a test
type Test struct {
	metav1.TypeMeta   `json:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Name is the name of the test
	Name string `json:"name"`

	// Policies are the policies to be used in the test
	Policies []string `json:"policies,omitempty"`

	// Policies are the resource to be used in the test
	Resources []string `json:"resources,omitempty"`

	// Variables is the values to be used in the test
	Variables string `json:"variables,omitempty"`

	// UserInfo is the user info to be used in the test
	UserInfo string `json:"userinfo,omitempty"`

	// Results are the results to be checked in the test
	Results []TestResult `json:"results,omitempty"`

	// Values are the values to be used in the test
	Values *ValuesSpec `json:"values,omitempty"`
}
