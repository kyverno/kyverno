package v1alpha1

import (
	"github.com/kyverno/kyverno-json/pkg/apis/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope="Cluster"

// Test declares a test
type Test struct {
	metav1.TypeMeta   `json:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Name is the name of the test.
	// This field is deprecated, use `metadata.name` instead
	Name string `json:"name,omitempty"`

	// Policies are the policies to be used in the test
	Policies []string `json:"policies,omitempty"`

	// Resources are the resource to be used in the test
	Resources []string `json:"resources,omitempty"`

	// Variables is the values to be used in the test
	Variables string `json:"variables,omitempty"`

	// UserInfo is the user info to be used in the test
	UserInfo string `json:"userinfo,omitempty"`

	// Results are the results to be checked in the test
	Results []TestResult `json:"results,omitempty"`

	// Checks are the verifications to be checked in the test
	Checks []CheckResult `json:"checks,omitempty"`

	// Values are the values to be used in the test
	Values *ValuesSpec `json:"values,omitempty"`
}

type CheckResult struct {
	// Match tells how to match relevant rule responses
	Match CheckMatch `json:"match,omitempty"`

	// Results contains assertion to be performed on the relevant rule responses
	Results *v1alpha1.Any `json:"results,omitempty"`
}

type CheckMatch struct {
	Resource *v1alpha1.Any `json:"resource,omitempty"`
	Policy   *v1alpha1.Any `json:"policy,omitempty"`
	Rule     *v1alpha1.Any `json:"rule,omitempty"`
}
