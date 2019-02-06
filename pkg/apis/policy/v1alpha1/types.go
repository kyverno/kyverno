/*
Copyright 2017 The Kubernetes Authors.
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Policy is a specification for a Policy resource
type Policy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PolicySpec   `json:"spec"`
	Status PolicyStatus `json:"status"`
}

// PolicySpec is the spec for a Policy resource
type PolicySpec struct {
	Rules []Rule `json:"rules"`
}

type Rule struct {
	Kind     string                     `json:"kind"`
	Name     *string                    `json:"name"`
	Selector *metav1.LabelSelector      `json:"selector"`
}

// PolicyStatus is the status for a Policy resource
type PolicyStatus struct {
	Logs []string `json:"log"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// StaticEgressIPList is a list of StaticEgressIP resources
type PolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Policy `json:"items"`
}
