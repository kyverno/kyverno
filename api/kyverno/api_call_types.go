package kyverno

import (
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

type APICall struct {
	// URLPath is the URL path to be used in the HTTP GET or POST request to the
	// Kubernetes API server (e.g. "/api/v1/namespaces" or  "/apis/apps/v1/deployments").
	// The format required is the same format used by the `kubectl get --raw` command.
	// See https://kyverno.io/docs/writing-policies/external-data-sources/#variables-from-kubernetes-api-server-calls
	// for details.
	// It's mutually exclusive with the Service field.
	// +kubebuilder:validation:Optional
	URLPath string `json:"urlPath"`

	// Method is the HTTP request type (GET or POST). Defaults to GET.
	// +kubebuilder:default=GET
	Method Method `json:"method,omitempty"`

	// The data object specifies the POST data sent to the server.
	// Only applicable when the method field is set to POST.
	// +kubebuilder:validation:Optional
	Data []RequestData `json:"data,omitempty"`

	// Service is an API call to a JSON web service.
	// This is used for non-Kubernetes API server calls.
	// It's mutually exclusive with the URLPath field.
	// +kubebuilder:validation:Optional
	Service *ServiceCall `json:"service,omitempty"`
}

type ServiceCall struct {
	// URL is the JSON web service URL. A typical form is
	// `https://{service}.{namespace}:{port}/{path}`.
	URL string `json:"url"`

	// Headers is a list of optional HTTP headers to be included in the request.
	Headers []HTTPHeader `json:"headers,omitempty"`

	// CABundle is a PEM encoded CA bundle which will be used to validate
	// the server certificate.
	// +kubebuilder:validation:Optional
	CABundle string `json:"caBundle"`
}

// Method is a HTTP request type.
// +kubebuilder:validation:Enum=GET;POST
type Method string

// RequestData contains the HTTP POST data
type RequestData struct {
	// Key is a unique identifier for the data value
	Key string `json:"key"`

	// Value is the data value
	Value *apiextv1.JSON `json:"value"`
}

type HTTPHeader struct {
	// Key is the header key
	Key string `json:"key"`
	// Value is the header value
	Value string `json:"value"`
}
