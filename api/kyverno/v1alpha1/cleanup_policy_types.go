/*
Copyright 2020 The Kubernetes authors.

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

package v1alpha1

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Schedule",type=string,JSONPath=".spec.schedule"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// CleanupPolicy defines a rule for resource cleanup.
type CleanupPolicy struct {
	metav1.TypeMeta   `json:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec declares policy behaviors.
	Spec CleanupPolicySpec `json:"spec"`

	// Status contains policy runtime data.
	// +optional
	Status CleanupPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CleanupPolicyList is a list of ClusterPolicy instances.
type CleanupPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []CleanupPolicy `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Schedule",type=string,JSONPath=".spec.schedule"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ClusterCleanupPolicy defines rule for resource cleanup.
type ClusterCleanupPolicy struct {
	metav1.TypeMeta   `json:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec declares policy behaviors.
	Spec CleanupPolicySpec `json:"spec"`

	// Status contains policy runtime data.
	// +optional
	Status CleanupPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterCleanupPolicyList is a list of ClusterCleanupPolicy instances.
type ClusterCleanupPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ClusterCleanupPolicy `json:"items"`
}

// CleanupPolicySpec stores specifications for selecting resources that the user needs to delete
// and schedule when the matching resources needs deleted.
type CleanupPolicySpec struct {
	// MatchResources defines when cleanuppolicy should be applied. The match
	// criteria can include resource information (e.g. kind, name, namespace, labels)
	// and admission review request information like the user name or role.
	// At least one kind is required.
	MatchResources kyvernov1.MatchResources `json:"match,omitempty"`

	// ExcludeResources defines when cleanuppolicy should not be applied. The exclude
	// criteria can include resource information (e.g. kind, name, namespace, labels)
	// and admission review request information like the name or role.
	// +optional
	ExcludeResources kyvernov1.MatchResources `json:"exclude,omitempty"`

	// The schedule in Cron format
	Schedule string `json:"schedule"`

	// Conditions defines conditions used to select resources which user needs to delete
	// +optional
	Conditions *kyvernov1.AnyAllConditions `json:"conditions,omitempty"`
}

// CleanupPolicyStatus stores the status of the policy.
type CleanupPolicyStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}
