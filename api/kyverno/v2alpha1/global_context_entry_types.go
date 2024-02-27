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

// GlobalContextEntry declares resources to be cached.
type GlobalContextEntry struct {
	metav1.TypeMeta   `json:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec declares policy exception behaviors.
	Spec GlobalContextEntrySpec `json:"spec" yaml:"spec"`

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
	errs = append(errs, c.Spec.Validate(field.NewPath("spec"))...)
	return errs
}

// IsNamespaced indicates if the policy is namespace scoped
func (c *GlobalContextEntry) IsNamespaced() bool {
	return false
}

// GlobalContextEntrySpec stores policy exception spec
type GlobalContextEntrySpec struct {
	// KubernetesResource stores infos about kubernetes resource that should be cached
	// +kubebuilder:validation:Optional
	KubernetesResource *KubernetesResource `json:"kubernetesResource,omitempty"`

	// APICall stores infos about API call that should be cached
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
	// Group defines the group of the resource
	Group string `json:"group,omitempty"`
	// Version defines the version of the resource
	Version string `json:"version,omitempty"`
	// Resource defines the type of the resource
	Resource string `json:"resource,omitempty"`
	// Namespace defines the namespace of the resource. Leave empty for cluster scoped resources.
	// +kubebuilder:validation:Optional
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// Validate implements programmatic validation
func (k *KubernetesResource) Validate(path *field.Path) (errs field.ErrorList) {
	if k.Group == "" {
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
	kyvernov1.APICall `json:",inline,omitempty"`
	// RefreshInterval defines the interval in duration at which to poll the APICall
	// +kubebuilder:validation:Format=duration
	// +kubebuilder:default=`10m`
	RefreshInterval *metav1.Duration `json:"refreshInterval,omitempty"`
}

// Validate implements programmatic validation
func (e *ExternalAPICall) Validate(path *field.Path) (errs field.ErrorList) {
	if e.RefreshInterval.Duration == 0*time.Second {
		errs = append(errs, field.Required(path.Child("refreshIntervalSeconds"), "A Resource entry requires a refresh interval greater than 0 seconds"))
	}
	return errs
}
