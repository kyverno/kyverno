package v1alpha1

import (
	"github.com/kyverno/kyverno-json/pkg/apis/policy/v1alpha1"
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

	// Policy Exceptions are the policy exceptions to be used in the test
	PolicyExceptions []string `json:"exceptions,omitempty"`
}

type CheckResult struct {
	// Match tells how to match relevant rule responses
	Match CheckMatch `json:"match,omitempty"`

	// Assert contains assertion to be performed on the relevant rule responses
	Assert v1alpha1.Any `json:"assert"`

	// Error contains negative assertion to be performed on the relevant rule responses
	Error v1alpha1.Any `json:"error"`
}

type CheckMatch struct {
	// Resource filters engine responses
	Resource *v1alpha1.Any `json:"resource,omitempty"`

	// Policy filters engine responses
	Policy *v1alpha1.Any `json:"policy,omitempty"`

	// Rule filters rule responses
	Rule *v1alpha1.Any `json:"rule,omitempty"`
}
