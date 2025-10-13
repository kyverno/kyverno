/*
Copyright 2022 The Kubernetes authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package v2alpha1

import (
	"time"

	gojmespath "github.com/kyverno/go-jmespath"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=gctxentry,categories=kyverno,scope="Cluster"
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="REFRESH INTERVAL",type="string",JSONPath=".spec.apiCall.refreshInterval"
// +kubebuilder:printcolumn:name="LAST REFRESH",type="date",JSONPath=".status.lastRefreshTime"
// +kubebuilder:storageversion

// GlobalContextEntry declares resources to be cached.
type GlobalContextEntry struct {
	metav1.TypeMeta   `json:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec declares policy exception behaviors.
	Spec GlobalContextEntrySpec `json:"spec"`

	// Status contains globalcontextentry runtime data.
	// +optional
	Status GlobalContextEntryStatus `json:"status,omitempty"`
}

// GetStatus returns the globalcontextentry status
func (p *GlobalContextEntry) GetStatus() *GlobalContextEntryStatus {
	return &p.Status
}

// Validate implements programmatic validation
func (c *GlobalContextEntry) Validate() (errs field.ErrorList) {
	errs = append(errs, c.Spec.Validate(field.NewPath("spec"), c.Name)...)
	return errs
}

// IsNamespaced indicates if the policy is namespace scoped
func (c *GlobalContextEntry) IsNamespaced() bool {
	return false
}

// GlobalContextEntrySpec stores policy exception spec
// +kubebuilder:oneOf:={required:{kubernetesResource}}
// +kubebuilder:oneOf:={required:{apiCall}}
type GlobalContextEntrySpec struct {
	// KubernetesResource stores infos about kubernetes resource that should be cached
	// +kubebuilder:validation:Optional
	KubernetesResource *KubernetesResource `json:"kubernetesResource,omitempty"`

	// APICall stores infos about API call that should be cached
	// +kubebuilder:validation:Optional
	APICall *ExternalAPICall `json:"apiCall,omitempty"`

	// Projections stores the data to be cached.
	// This determines what data from the source will be cached.
	// +kubebuilder:validation:Optional
	Projections []GlobalContextEntryProjection `json:"projections,omitempty"`
}

func (c *GlobalContextEntrySpec) IsAPICall() bool {
	return c.APICall != nil
}

func (c *GlobalContextEntrySpec) IsResource() bool {
	return c.KubernetesResource != nil
}

// Validate implements programmatic validation
func (c *GlobalContextEntrySpec) Validate(path *field.Path, name string) (errs field.ErrorList) {
	if c.IsResource() && c.IsAPICall() {
		errs = append(errs, field.Forbidden(path, "An entry cannot have both kubernetesResource and apiCall"))
	}
	if !c.IsResource() && !c.IsAPICall() {
		errs = append(errs, field.Required(path, "An entry must define either kubernetesResource or apiCall"))
	}
	if c.IsResource() {
		errs = append(errs, c.KubernetesResource.Validate(path.Child("kubernetesResource"))...)
	}
	if c.IsAPICall() {
		errs = append(errs, c.APICall.Validate(path.Child("apiCall"))...)
	}
	for i, projection := range c.Projections {
		errs = append(errs, projection.Validate(path.Child("projections").Index(i), name)...)
	}
	return errs
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GlobalContextEntryList is a list of Cached Context Entries
type GlobalContextEntryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []GlobalContextEntry `json:"items"`
}

// KubernetesResource stores infos about kubernetes resource that should be cached
type KubernetesResource struct {
	// Group defines the group of the resource.
	// +kubebuilder:validation:Optional
	Group string `json:"group,omitempty"`

	// Version defines the version of the resource.
	// +kubebuilder:validation:Optional
	Version string `json:"version,omitempty"`

	// Resource defines the type of the resource.
	// +kubebuilder:validation:Required
	Resource string `json:"resource"`

	// Namespace defines the namespace of the resource. Leave empty for cluster scoped resources.
	// If left empty for namespaced resources, all resources from all namespaces will be cached.
	// +kubebuilder:validation:Optional
	Namespace string `json:"namespace,omitempty"`
}

// Validate implements programmatic validation
func (k *KubernetesResource) Validate(path *field.Path) (errs field.ErrorList) {
	isCoreGroup := k.Group == "" && k.Version == "v1"
	if k.Group == "" && !isCoreGroup {
		errs = append(errs, field.Required(path.Child("group"), "A Resource entry requires a group"))
	}
	if k.Version == "" {
		errs = append(errs, field.Required(path.Child("version"), "A Resource entry requires a version"))
	}
	if k.Resource == "" {
		errs = append(errs, field.Required(path.Child("resource"), "A Resource entry requires a resource"))
	}
	return errs
}

// ExternalAPICall stores infos about API call that should be cached
type ExternalAPICall struct {
	// URLPath defines the URL to be used for the HTTP GET or POST request.
	// +kubebuilder:validation:Required
	URLPath string `json:"urlPath"`

	// Method defines the method for the HTTP request. Defaults to GET.
	// Valid values are GET and POST.
	// +kubebuilder:validation:Enum=GET;POST
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="GET"
	Method string `json:"method,omitempty"`

	// RefreshInterval defines the interval at which to poll the API endpoint.
	// The duration format is a number and a unit. Examples: 30s, 1m, 1h30m.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:="10m"
	RefreshInterval metav1.Duration `json:"refreshInterval,omitempty"`

	// RetryLimit sets the max number of times to retry the API request in case of failure. Defaults to 0 (no retries).
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:validation:Optional
	// +kubebuilder:default:=0
	RetryLimit int `json:"retryLimit,omitempty"`

	// Data specifies the POST data sent to the server.
	// +kubebuilder:validation:Optional
	Data []RequestData `json:"data,omitempty"`

	// Service defines the service data to query.
	// +kubebuilder:validation:Optional
	Service *kyvernov1.ServiceCall `json:"service,omitempty"`
}

// RequestData contains the HTTP POST data
type RequestData struct {
	// Key is a unique identifier for the data value
	// +kubebuilder:validation:Required
	Key string `json:"key"`

	// Value is the data value
	// +kubebuilder:validation:Required
	Value apiextensionsv1.JSON `json:"value"`
}

// Validate implements programmatic validation
func (e *ExternalAPICall) Validate(path *field.Path) (errs field.ErrorList) {
	if e.URLPath == "" && e.Service == nil {
		errs = append(errs, field.Required(path.Child("urlPath"), "either urlPath or service is required"))
	}
	if e.URLPath != "" && e.Service != nil {
		errs = append(errs, field.Forbidden(path, "cannot specify both urlPath and service"))
	}
	if e.Method != "" && e.Method != "GET" && e.Method != "POST" {
		errs = append(errs, field.Invalid(path.Child("method"), e.Method, "method must be GET or POST"))
	}
	if e.RefreshInterval.Duration < time.Second {
		errs = append(errs, field.Invalid(path.Child("refreshInterval"), e.RefreshInterval.Duration, "refreshInterval must be at least 1 second"))
	}
	return errs
}

// GlobalContextEntryProjection stores the data to be cached.
type GlobalContextEntryProjection struct {
	// Name is a unique identifier for the projection.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// JMESPath is a JMES expression that is used to transform the source JSON data.
	// +kubebuilder:validation:Required
	JMESPath string `json:"jmesPath"`
}

// Validate implements programmatic validation
func (p *GlobalContextEntryProjection) Validate(path *field.Path, gctxName string) (errs field.ErrorList) {
	if p.Name == "" {
		errs = append(errs, field.Required(path.Child("name"), "A projection entry requires a name"))
	}
	if p.Name == gctxName {
		errs = append(errs, field.Required(path.Child("name"), "A projection entry requires a name different from the global context entry name"))
	}
	if p.JMESPath == "" {
		errs = append(errs, field.Required(path.Child("jmesPath"), "A projection entry requires a JMESPath"))
	} else {
		if _, err := gojmespath.Compile(p.JMESPath); err != nil {
			errs = append(errs, field.Invalid(path.Child("jmesPath"), p.JMESPath, err.Error()))
		}
	}
	return errs
}
