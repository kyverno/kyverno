package v1alpha1

import (
	"encoding/json"

	"github.com/kyverno/kyverno-json/pkg/apis/policy/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

	// APICallResponses provides static responses for HTTP/API calls during offline testing.
	// Each entry maps a URL (and optional HTTP method) to a fixed response body,
	// intercepting CEL http.Get() calls so policies can be tested without a real server.
	APICallResponses []APICallResponseEntry `json:"apiCallResponses,omitempty"`

	// GlobalContextEntries provides static data for GlobalContextEntry references
	// during offline testing. Each entry maps a GlobalContextEntry name to arbitrary JSON data.
	GlobalContextEntries []GlobalContextEntryValue `json:"globalContextEntries,omitempty"`
}

// APICallResponseEntry maps a URL (and optional HTTP method) to a static response body.
type APICallResponseEntry struct {
	// URL is the request URL to match (e.g. "https://example.com/config").
	URL string `json:"url"`

	// Method is the HTTP method to match (GET or POST).
	// If empty, the entry matches any method via plain URL lookup.
	// +kubebuilder:validation:Enum=GET;POST;""
	// +kubebuilder:validation:Optional
	Method string `json:"method,omitempty"`

	// Response is the static HTTP response to return.
	Response APICallResponse `json:"response"`
}

// APICallResponse represents a static HTTP response.
type APICallResponse struct {
	// StatusCode is the HTTP status code to inject (defaults to 200).
	StatusCode int `json:"statusCode,omitempty"`

	// Body is the response body as arbitrary JSON.
	Body runtime.RawExtension `json:"body"`
}

// GlobalContextEntryValue provides static data for a named GlobalContextEntry.
type GlobalContextEntryValue struct {
	// Name is the name of the GlobalContextEntry resource.
	Name string `json:"name"`

	// Data is the static data to return for this global context entry (arbitrary JSON).
	Data runtime.RawExtension `json:"data"`
}

// RawExtensionToObject decodes a runtime.RawExtension into a plain Go value.
// Returns nil (without error) when raw.Raw is empty.
func RawExtensionToObject(raw runtime.RawExtension) (interface{}, error) {
	if len(raw.Raw) == 0 {
		return nil, nil
	}
	var v interface{}
	if err := json.Unmarshal(raw.Raw, &v); err != nil {
		return nil, err
	}
	return v, nil
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
