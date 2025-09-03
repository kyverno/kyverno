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
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
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

// Validate implements programmatic validation
func (c *GlobalContextEntry) Validate() (errs field.ErrorList) {
	errs = append(errs, c.Spec.Validate(field.NewPath("spec"), c.Name)...)
	return errs
}

// GlobalContextEntrySpec stores policy exception spec
// +kubebuilder:oneOf:={required:{kubernetesResource}}
// +kubebuilder:oneOf:={required:{apiCall}}
type GlobalContextEntrySpec = kyvernov2beta1.GlobalContextEntrySpec



// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GlobalContextEntryList is a list of Cached Context Entries
type GlobalContextEntryList = kyvernov2beta1.GlobalContextEntryList

// KubernetesResource stores infos about kubernetes resource that should be cached
type KubernetesResource = kyvernov2beta1.KubernetesResource



type ExternalAPICall = kyvernov2beta1.ExternalAPICall



type GlobalContextEntryProjection = kyvernov2beta1.GlobalContextEntryProjection


