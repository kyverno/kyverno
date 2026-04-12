package v1alpha1

import (
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

	// HTTPPayloads is the HTTP payloads to be used in the test
	HTTPPayloads []string `json:"httpPayloads,omitempty"`

	// EnvoyPayloads is the Envoy payloads to be used in the test
	EnvoyPayloads []string `json:"envoyPayloads,omitempty"`

	// Target Resources are for policies that have mutate existing
	TargetResources []string `json:"targetResources,omitempty"`

	// Variables is the values to be used in the test
	Variables string `json:"variables,omitempty"`

	// Resources that act as parameters for validating/mutating admission policies
	ParamResources []string `json:"paramResources,omitempty"`

	// UserInfo is the user info to be used in the test
	UserInfo string `json:"userinfo,omitempty"`

	// Results are the results to be checked in the test
	Results []TestResult `json:"results,omitempty"`

	// Values are the values to be used in the test
	Values *ValuesSpec `json:"values,omitempty"`

	// PolicyExceptions are the policy exceptions to be used in the test
	PolicyExceptions []string `json:"exceptions,omitempty"`

	// Context file containing context data for CEL policies
	Context string `json:"context,omitempty"`

	// ClusterResources are the cluster resources to be used in the test
	ClusterResources []string `json:"clusterResources,omitempty"`
}

type TestResourceSpec struct {
	Group       string `json:"group,omitempty"`
	Version     string `json:"version,omitempty"`
	Kind        string `json:"kind,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	Subresource string `json:"subresource,omitempty"`
	Name        string `json:"name,omitempty"`
}

