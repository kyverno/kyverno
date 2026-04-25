package v1alpha1

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

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
	// Each entry maps a URL (and optional HTTP method) to a fixed response.
	// This is used to mock CEL http.Get/http.Post and v1 policy context.apiCall entries
	// so tests can run without a real server.
	APICallResponses []APICallResponseEntry `json:"apiCallResponses,omitempty"`

	// GlobalContextEntries provides static data for GlobalContextEntry references
	// during offline testing (v1 globalReference and CEL globalContext.Get).
	// Optional fieldPath and projections shape the JSON passed to policies.
	GlobalContextEntries []GlobalContextEntryValue `json:"globalContextEntries,omitempty"`
}

// APICallResponseEntry maps a URL (and optional HTTP method) to a static response body.
type APICallResponseEntry struct {
	// URL is the request URL to match (for example https://example.com/config).
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

	// Body is the response body as arbitrary JSON (Kubernetes object shape: string keys, JSON values).
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	Body runtime.RawExtension `json:"body"`
}

// GlobalContextProjection names a fragment of the mock global context root, extracted with JMESPath.
// The resulting map is exposed to policies as top-level keys (e.g. name "items" → policy sees .items).
type GlobalContextProjection struct {
	// Name is the key policies see on the mocked global context object.
	Name string `json:"name"`

	// Path is a JMESPath expression evaluated against the root after FieldPath (if any) is applied.
	Path string `json:"path"`
}

// GlobalContextEntryValue provides static data for a named GlobalContextEntry.
type GlobalContextEntryValue struct {
	// Name is the name of the GlobalContextEntry resource.
	Name string `json:"name"`

	// Data is the static JSON root for this mock entry (arbitrary object).
	// +kubebuilder:validation:Type=object
	// +kubebuilder:pruning:PreserveUnknownFields
	Data runtime.RawExtension `json:"data"`

	// FieldPath is an optional JMESPath applied to decoded Data to produce the root before projections.
	FieldPath string `json:"fieldPath,omitempty"`

	// Projections optionally build a map of named values from that root.
	// When non-empty, the value passed to policies is only these keys (each Path is evaluated on the root).
	Projections []GlobalContextProjection `json:"projections,omitempty"`
}

// ValidateAPICallResponses checks mock HTTP entries for a valid URL, HTTP status, and JSON body.
func ValidateAPICallResponses(entries []APICallResponseEntry) error {
	for i := range entries {
		if err := validateAPICallResponseEntry(i, entries[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateAPICallResponseEntry(i int, e APICallResponseEntry) error {
	u := strings.TrimSpace(e.URL)
	if u == "" {
		return fmt.Errorf("apiCallResponses[%d]: url is required", i)
	}
	parsedURL, err := url.Parse(u)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return fmt.Errorf("apiCallResponses[%d]: url must be a valid http/https URL", i)
	}
	m := strings.ToUpper(strings.TrimSpace(e.Method))
	if m != "" && m != "GET" && m != "POST" {
		return fmt.Errorf("apiCallResponses[%d]: method must be GET or POST", i)
	}

	raw := e.Response.Body.Raw
	data := strings.TrimSpace(string(raw))
	if data == "" || data == "null" {
		return fmt.Errorf("apiCallResponses[%d] url %q: response.body is required", i, e.URL)
	}
	if !json.Valid(raw) {
		return fmt.Errorf("apiCallResponses[%d] url %q: response.body must be valid JSON", i, e.URL)
	}
	sc := e.Response.StatusCode
	if sc != 0 && (sc < 100 || sc > 599) {
		return fmt.Errorf("apiCallResponses[%d] url %q: statusCode %d must be between 100 and 599", i, e.URL, sc)
	}
	return nil
}

// ValidateGlobalContextEntries checks mock global context entries and projection definitions.
// Data is required for every entry (matches the CLI Test CRD); must be a JSON object.
func ValidateGlobalContextEntries(entries []GlobalContextEntryValue) error {
	for _, e := range entries {
		if strings.TrimSpace(e.Name) == "" {
			return fmt.Errorf("globalContextEntries: name is required")
		}
		for _, p := range e.Projections {
			if strings.TrimSpace(p.Name) == "" {
				return fmt.Errorf("globalContextEntries entry %q: projection name is required", e.Name)
			}
			if strings.TrimSpace(p.Path) == "" {
				return fmt.Errorf("globalContextEntries entry %q projection %q: path is required", e.Name, p.Name)
			}
		}
		data := strings.TrimSpace(string(e.Data.Raw))
		if data == "" || data == "null" {
			return fmt.Errorf("globalContextEntries entry %q: data is required", e.Name)
		}
		obj, err := RawExtensionToObject(e.Data)
		if err != nil {
			return fmt.Errorf("globalContextEntries entry %q: data must be valid JSON: %w", e.Name, err)
		}
		if _, ok := obj.(map[string]interface{}); !ok {
			return fmt.Errorf("globalContextEntries entry %q: data must be a JSON object", e.Name)
		}
	}
	return nil
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
