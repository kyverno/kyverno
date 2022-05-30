/*
Copyright 2022.

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

package v1beta1

import (
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UpdateRequestStatus defines the observed state of UpdateRequest
type UpdateRequestStatus struct {
	// Handler represents the instance ID that handles the UR
	Handler string `json:"handler,omitempty" yaml:"handler,omitempty"`

	// State represents state of the update request.
	State UpdateRequestState `json:"state" yaml:"state"`

	// Specifies request status message.
	// +optional
	Message string `json:"message,omitempty" yaml:"message,omitempty"`

	// This will track the resources that are updated by the generate Policy.
	// Will be used during clean up resources.
	GeneratedResources []v1.ResourceSpec `json:"generatedResources,omitempty" yaml:"generatedResources,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Policy",type="string",JSONPath=".spec.policy"
// +kubebuilder:printcolumn:name="RuleType",type="string",JSONPath=".spec.requestType"
// +kubebuilder:printcolumn:name="ResourceKind",type="string",JSONPath=".spec.resource.kind"
// +kubebuilder:printcolumn:name="ResourceName",type="string",JSONPath=".spec.resource.name"
// +kubebuilder:printcolumn:name="ResourceNamespace",type="string",JSONPath=".spec.resource.namespace"
// +kubebuilder:printcolumn:name="status",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:shortName=ur

// UpdateRequestStatus is a request to process mutate and generate rules in background.
type UpdateRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the information to identify the update request.
	Spec UpdateRequestSpec `json:"spec,omitempty"`

	// Status contains statistics related to update request.
	// +optional
	Status UpdateRequestStatus `json:"status,omitempty"`
}

type RequestType string

const (
	Mutate   RequestType = "mutate"
	Generate RequestType = "generate"
)

// UpdateRequestSpec stores the request specification.
type UpdateRequestSpec struct {
	// Type represents request type for background processing
	// +kubebuilder:validation:Enum=mutate;generate
	Type RequestType `json:"requestType,omitempty" yaml:"requestType,omitempty"`

	// Specifies the name of the policy.
	Policy string `json:"policy" yaml:"policy"`

	// ResourceSpec is the information to identify the update request.
	Resource v1.ResourceSpec `json:"resource" yaml:"resource"`

	// Context ...
	Context UpdateRequestSpecContext `json:"context" yaml:"context"`
}

// UpdateRequestSpecContext stores the context to be shared.
type UpdateRequestSpecContext struct {
	// +optional
	UserRequestInfo RequestInfo `json:"userInfo,omitempty" yaml:"userInfo,omitempty"`
	// +optional
	AdmissionRequestInfo AdmissionRequestInfoObject `json:"admissionRequestInfo,omitempty" yaml:"admissionRequestInfo,omitempty"`
}

// RequestInfo contains permission info carried in an admission request.
type RequestInfo struct {
	// Roles is a list of possible role send the request.
	// +nullable
	// +optional
	Roles []string `json:"roles" yaml:"roles"`

	// ClusterRoles is a list of possible clusterRoles send the request.
	// +nullable
	// +optional
	ClusterRoles []string `json:"clusterRoles" yaml:"clusterRoles"`

	// UserInfo is the userInfo carried in the admission request.
	// +optional
	AdmissionUserInfo authenticationv1.UserInfo `json:"userInfo" yaml:"userInfo"`
}

// AdmissionRequestInfoObject stores the admission request and operation details
type AdmissionRequestInfoObject struct {
	// +optional
	AdmissionRequest *admissionv1.AdmissionRequest `json:"admissionRequest,omitempty" yaml:"admissionRequest,omitempty"`
	// +optional
	Operation admissionv1.Operation `json:"operation,omitempty" yaml:"operation,omitempty"`
}

// UpdateRequestState defines the state of request.
type UpdateRequestState string

const (
	// Pending - the Request is yet to be processed or resource has not been created.
	Pending UpdateRequestState = "Pending"

	// Failed - the Update Request Controller failed to process the rules.
	Failed UpdateRequestState = "Failed"

	// Completed - the Update Request Controller created resources defined in the policy.
	Completed UpdateRequestState = "Completed"

	// Skip - the Update Request Controller skips to generate the resource.
	Skip UpdateRequestState = "Skip"
)

//+kubebuilder:object:root=true

// UpdateRequestList contains a list of UpdateRequest
type UpdateRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UpdateRequest `json:"items"`
}

func (s *UpdateRequestSpec) GetRequestType() RequestType {
	return s.Type
}
