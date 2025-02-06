/*
Copyright The Kubernetes Authors.

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

// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1alpha1

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PolicyStatusApplyConfiguration represents an declarative configuration of the PolicyStatus type for use
// with apply.
type PolicyStatusApplyConfiguration struct {
	Ready      *bool          `json:"ready,omitempty"`
	Conditions []v1.Condition `json:"conditions,omitempty"`
}

// PolicyStatusApplyConfiguration constructs an declarative configuration of the PolicyStatus type for use with
// apply.
func PolicyStatus() *PolicyStatusApplyConfiguration {
	return &PolicyStatusApplyConfiguration{}
}

// WithReady sets the Ready field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Ready field is set to the value of the last call.
func (b *PolicyStatusApplyConfiguration) WithReady(value bool) *PolicyStatusApplyConfiguration {
	b.Ready = &value
	return b
}

// WithConditions adds the given value to the Conditions field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Conditions field.
func (b *PolicyStatusApplyConfiguration) WithConditions(values ...v1.Condition) *PolicyStatusApplyConfiguration {
	for i := range values {
		b.Conditions = append(b.Conditions, values[i])
	}
	return b
}
