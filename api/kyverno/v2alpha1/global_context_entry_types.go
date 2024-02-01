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
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:shortName=gctxentry,categories=kyverno,scope="Cluster"

// GlobalContextEntry declares resources to be cached.
type GlobalContextEntry struct {
	metav1.TypeMeta   `json:",inline,omitempty" yaml:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Spec declares policy exception behaviors.
	Spec GlobalContextEntrySpec `json:"spec" yaml:"spec"`

	// Status contains globalcontextentry runtime data.
	// +optional
	Status GlobalContextEntryStatus `json:"status,omitempty" yaml:"status,omitempty"`
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
	// K8sResource stores infos about kubernetes resource that should be cached
	// +kubebuilder:validation:Optional
	K8sResource *kyvernov1.K8sResource `json:"k8sResource,omitempty" yaml:"k8sResource,omitempty"`

	// APICall stores infos about API call that should be cached
	// +kubebuilder:validation:Optional
	APICall *kyvernov1.ExternalAPICall `json:"apiCall,omitempty" yaml:"apiCall,omitempty"`
}

func (c *GlobalContextEntrySpec) IsAPICall() bool {
	return c.APICall != nil
}

func (c *GlobalContextEntrySpec) IsResource() bool {
	return c.K8sResource != nil
}

// Validate implements programmatic validation
func (c *GlobalContextEntrySpec) Validate(path *field.Path) (errs field.ErrorList) {
	if c.IsResource() && c.IsAPICall() {
		errs = append(errs, field.Forbidden(path.Child("resource"), "An External API Call entry requires a url"))
	}
	if c.IsResource() {
		errs = append(errs, c.K8sResource.Validate(path.Child("resource"))...)
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
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata" yaml:"metadata"`
	Items           []GlobalContextEntry `json:"items" yaml:"items"`
}
