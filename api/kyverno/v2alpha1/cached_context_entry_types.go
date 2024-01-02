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
// +kubebuilder:resource:shortName=cacheentry,categories=kyverno,scope="Cluster"

// CachedContextEntry declares resources to be cached.
type CachedContextEntry struct {
	metav1.TypeMeta   `json:",inline,omitempty" yaml:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Spec declares policy exception behaviors.
	Spec CachedContextEntrySpec `json:"spec" yaml:"spec"`
}

// Validate implements programmatic validation
func (c *CachedContextEntry) Validate() (errs field.ErrorList) {
	errs = append(errs, c.Spec.Validate(field.NewPath("spec"))...)
	return errs
}

// IsNamespaced indicates if the policy is namespace scoped
func (c *CachedContextEntry) IsNamespaced() bool {
	return false
}

// CachedContextEntrySpec stores policy exception spec
type CachedContextEntrySpec struct {
	kyvernov1.ResourceCache `json:",inline,omitempty" yaml:",inline,omitempty"`
}

func (c *CachedContextEntrySpec) IsAPICall() bool {
	return c.APICall != nil
}

func (c *CachedContextEntrySpec) IsResource() bool {
	return c.Resource != nil
}

// Validate implements programmatic validation
func (c *CachedContextEntrySpec) Validate(path *field.Path) (errs field.ErrorList) {
	if c.IsResource() && c.IsAPICall() {
		errs = append(errs, field.Forbidden(path.Child("resource"), "An External API Call entry requires a url"))
	}
	if c.IsResource() {
		errs = append(errs, c.Resource.Validate(path.Child("resource"))...)
	}
	if c.IsAPICall() {
		errs = append(errs, c.APICall.Validate(path.Child("apiCall"))...)
	}
	return errs
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CachedContextEntryList is a list of Cached Context Entries
type CachedContextEntryList struct {
	metav1.TypeMeta `json:",inline" yaml:",inline"`
	metav1.ListMeta `json:"metadata" yaml:"metadata"`
	Items           []CachedContextEntry `json:"items" yaml:"items"`
}
