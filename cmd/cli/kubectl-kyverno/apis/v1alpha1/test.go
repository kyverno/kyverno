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

	// APICallResponses mocks HTTP for offline tests (CEL http.* and v1 context.apiCall).
	APICallResponses []APICallResponseEntry `json:"apiCallResponses,omitempty"`

	// GlobalContextEntries mocks GlobalContextEntry data for offline tests (v1 and CEL).
	GlobalContextEntries []GlobalContextEntryValue `json:"globalContextEntries,omitempty"`
}

// APICallResponseEntry maps a URL or URL path (optional method) to a static response body.
type APICallResponseEntry struct {
	// URL is an absolute https or http URL for matching (CEL http.* and v1 context.apiCall service.url).
	URL string `json:"url,omitempty"`

	// URLPath is an absolute path (e.g. /api/v1/namespaces) for matching v1 context.apiCall urlPath.
	URLPath string `json:"urlPath,omitempty"`

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

// GlobalContextProjection pairs a policy-visible name with a JMESPath over the mock root.
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

	// Projections are optional named JMESPath extracts (see GlobalContextProjection).
	Projections []GlobalContextProjection `json:"projections,omitempty"`
}

// ValidateAPICallResponses validates mock HTTP entries before a test run.
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
	up := strings.TrimSpace(e.URLPath)
	if u == "" && up == "" {
		return fmt.Errorf("apiCallResponses[%d]: either url or urlPath is required", i)
	}
	if u != "" && up != "" {
		return fmt.Errorf("apiCallResponses[%d]: url and urlPath are mutually exclusive", i)
	}
	if u != "" {
		parsedURL, err := url.Parse(u)
		if err != nil {
			return fmt.Errorf("apiCallResponses[%d]: url %q is not a valid URL: %w", i, u, err)
		}
		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			return fmt.Errorf("apiCallResponses[%d]: url %q must be an absolute https/http URL", i, u)
		}
		if parsedURL.Hostname() == "" {
			return fmt.Errorf("apiCallResponses[%d]: url %q has scheme but no host", i, u)
		}
	}
	if up != "" && !strings.HasPrefix(up, "/") {
		return fmt.Errorf("apiCallResponses[%d]: urlPath %q must be an absolute path starting with '/'", i, up)
	}
	m := strings.ToUpper(strings.TrimSpace(e.Method))
	if m != "" && m != "GET" && m != "POST" {
		return fmt.Errorf("apiCallResponses[%d]: method must be GET or POST", i)
	}

	label := u
	if label == "" {
		label = up
	}
	raw := e.Response.Body.Raw
	data := strings.TrimSpace(string(raw))
	if data == "" || data == "null" {
		return fmt.Errorf("apiCallResponses[%d] %q: response.body is required", i, label)
	}
	obj, err := RawExtensionToObject(e.Response.Body)
	if err != nil {
		return fmt.Errorf("apiCallResponses[%d] %q: response.body must be valid JSON: %w", i, label, err)
	}
	if _, ok := obj.(map[string]interface{}); !ok {
		return fmt.Errorf("apiCallResponses[%d] %q: response.body must be a JSON object", i, label)
	}
	sc := e.Response.StatusCode
	if sc != 0 && (sc < 100 || sc > 599) {
		return fmt.Errorf("apiCallResponses[%d] %q: statusCode %d must be between 100 and 599", i, label, sc)
	}
	return nil
}

// ResolvedURL returns the effective URL from either the url or urlPath field.
func (e APICallResponseEntry) ResolvedURL() string {
	if u := strings.TrimSpace(e.URL); u != "" {
		return u
	}
	return strings.TrimSpace(e.URLPath)
}

// ValidateGlobalContextEntries validates mock global context entries.
func ValidateGlobalContextEntries(entries []GlobalContextEntryValue) error {
	seen := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		if strings.TrimSpace(e.Name) == "" {
			return fmt.Errorf("globalContextEntries: name is required")
		}
		name := strings.TrimSpace(e.Name)
		if _, dup := seen[name]; dup {
			return fmt.Errorf("globalContextEntries: duplicate name %q", name)
		}
		seen[name] = struct{}{}
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
