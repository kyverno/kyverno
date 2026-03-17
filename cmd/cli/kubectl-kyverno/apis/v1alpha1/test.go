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

	// JSONPayload is the JSON payload to be used in the test
	JSONPayload string `json:"jsonPayload,omitempty"`

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

	// Checks are the verifications to be checked in the test
	Checks []CheckResult `json:"checks,omitempty"`

	// Values are the values to be used in the test
	Values *ValuesSpec `json:"values,omitempty"`

	// PolicyExceptions are the policy exceptions to be used in the test
	PolicyExceptions []string `json:"exceptions,omitempty"`

	// Context file containing context data for CEL policies
	Context string `json:"context,omitempty"`

	// ClusterResources are the cluster resources to be used in the test
	ClusterResources []string `json:"clusterResources,omitempty"`

	// MockAPICallResponses provides static mock responses for APICall service
	// context entries, allowing offline testing of policies that call external
	// HTTP endpoints.
	MockAPICallResponses []MockAPICallResponse `json:"mockAPICallResponses,omitempty"`

	// MockGlobalContextEntries provides static mock data for GlobalContextEntry
	// references, allowing offline testing of policies that depend on global
	// context entries.
	MockGlobalContextEntries []MockGlobalContextEntry `json:"mockGlobalContextEntries,omitempty"`
}

// MockAPICallResponse provides a static mock response for an APICall service
// context entry, keyed by the service URL path.
type MockAPICallResponse struct {
	// URLPath is the service URL to match against APICall context entries.
	URLPath string `json:"urlPath"`

	// Response is the mock HTTP response to return.
	Response MockResponse `json:"response"`
}

// MockResponse represents a mock HTTP response body.
type MockResponse struct {
	// StatusCode is the HTTP status code (informational only; not enforced by the engine).
	StatusCode int `json:"statusCode,omitempty"`

	// Body is the response body that will be injected as the context entry value.
	Body interface{} `json:"body"`
}

// MockGlobalContextEntry provides static mock data for a GlobalContextEntry
// reference by name.
type MockGlobalContextEntry struct {
	// Name is the name of the GlobalContextEntry resource to mock.
	Name string `json:"name"`

	// Data is the static data to return for this global context entry.
	Data interface{} `json:"data"`
}

type CheckResult struct {
	// Match tells how to match relevant rule responses
	Match CheckMatch `json:"match,omitempty"`

	// Assert contains assertion to be performed on the relevant rule responses
	Assert v1alpha1.Any `json:"assert"`

	// Error contains negative assertion to be performed on the relevant rule responses
	Error v1alpha1.Any `json:"error"`
}

type TestResourceSpec struct {
	Group       string `json:"group,omitempty"`
	Version     string `json:"version,omitempty"`
	Kind        string `json:"kind,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	Subresource string `json:"subresource,omitempty"`
	Name        string `json:"name,omitempty"`
}

type CheckMatch struct {
	// Resource filters engine responses
	Resource *v1alpha1.Any `json:"resource,omitempty"`

	// Policy filters engine responses
	Policy *v1alpha1.Any `json:"policy,omitempty"`

	// Rule filters rule responses
	Rule *v1alpha1.Any `json:"rule,omitempty"`
}
