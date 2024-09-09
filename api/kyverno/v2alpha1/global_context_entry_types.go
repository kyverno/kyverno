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

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=gctxentry,categories=kyverno,scope="Cluster"
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type == "Ready")].status`
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="REFRESH INTERVAL",type="string",JSONPath=".spec.apiCall.refreshInterval"
// +kubebuilder:printcolumn:name="LAST REFRESH",type="date",JSONPath=".status.lastRefreshTime"

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

// Validate implements programmatic validation
func (c *GlobalContextEntry) Validate() (errs field.ErrorList) {
	errs = append(errs, c.Spec.Validate(field.NewPath("spec"))...)
	return errs
}

// GlobalContextEntrySpec stores policy exception spec
// +kubebuilder:oneOf:={required:{kubernetesResource}}
// +kubebuilder:oneOf:={required:{apiCall}}
type GlobalContextEntrySpec struct {
	// Stores a list of Kubernetes resources which will be cached.
	// Mutually exclusive with APICall.
	// +kubebuilder:validation:Optional
	KubernetesResource *KubernetesResource `json:"kubernetesResource,omitempty"`

	// Stores results from an API call which will be cached.
	// Mutually exclusive with KubernetesResource.
	// This can be used to make calls to external (non-Kubernetes API server) services.
	// It can also be used to make calls to the Kubernetes API server in such cases:
	// 1. A POST is needed to create a resource.
	// 2. Finer-grained control is needed. Example: To restrict the number of resources cached.
	// +kubebuilder:validation:Optional
	APICall *ExternalAPICall `json:"apiCall,omitempty"`
}

func (c *GlobalContextEntrySpec) IsAPICall() bool {
	return c.APICall != nil
}

func (c *GlobalContextEntrySpec) IsResource() bool {
	return c.KubernetesResource != nil
}

// Validate implements programmatic validation
func (c *GlobalContextEntrySpec) Validate(path *field.Path) (errs field.ErrorList) {
	if c.IsResource() && c.IsAPICall() {
		errs = append(errs, field.Forbidden(path.Child("kubernetesResource"), "A global context entry should either have KubernetesResource or APICall"))
	}
	if !c.IsResource() && !c.IsAPICall() {
		errs = append(errs, field.Forbidden(path.Child("kubernetesResource"), "A global context entry should either have KubernetesResource or APICall"))
	}
	if c.IsResource() {
		errs = append(errs, c.KubernetesResource.Validate(path.Child("resource"))...)
	}
	if c.IsAPICall() {
		errs = append(errs, c.APICall.Validate(path.Child("apiCall"))...)
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
	// +kubebuilder:validation:Required
	Version string `json:"version"`
	// Resource defines the type of the resource.
	// Requires the pluralized form of the resource kind in lowercase. (Ex., "deployments")
	// +kubebuilder:validation:Required
	Resource string `json:"resource"`
	// Namespace defines the namespace of the resource. Leave empty for cluster scoped resources.
	// If left empty for namespaced resources, all resources from all namespaces will be cached.
	// +kubebuilder:validation:Optional
	// +optional
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

type ExternalAPICall struct {
	kyvernov1.APICall `json:",inline,omitempty"`
	// RefreshInterval defines the interval in duration at which to poll the APICall.
	// The duration is a sequence of decimal numbers, each with optional fraction and a unit suffix,
	// such as "300ms", "1.5h" or "2h45m". Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
	// +kubebuilder:validation:Format=duration
	// +kubebuilder:default=`10m`
	RefreshInterval *metav1.Duration `json:"refreshInterval,omitempty"`
	// RetryLimit defines the number of times the APICall should be retried in case of failure.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=3
	// +kubebuilder:validation:Optional
	// +optional
	RetryLimit int `json:"retryLimit,omitempty"`
}

// Validate implements programmatic validation
func (e *ExternalAPICall) Validate(path *field.Path) (errs field.ErrorList) {
	if e.RefreshInterval.Duration == 0*time.Second {
		errs = append(errs, field.Required(path.Child("refreshIntervalSeconds"), "A Resource entry requires a refresh interval greater than 0 seconds"))
	}
	if (e.Service == nil && e.URLPath == "") || (e.Service != nil && e.URLPath != "") {
		errs = append(errs, field.Forbidden(path.Child("service"), "An External API call should either have Service or URLPath"))
	}
	if e.Data != nil && e.Method != "POST" {
		errs = append(errs, field.Forbidden(path.Child("method"), "An External API call with data should have method as POST"))
	}
	return errs
}
