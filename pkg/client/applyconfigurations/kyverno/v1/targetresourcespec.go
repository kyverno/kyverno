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

package v1

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
)

// TargetResourceSpecApplyConfiguration represents an declarative configuration of the TargetResourceSpec type for use
// with apply.
type TargetResourceSpecApplyConfiguration struct {
	*ResourceSpecApplyConfiguration `json:"ResourceSpec,omitempty"`
	Context                         []ContextEntryApplyConfiguration `json:"context,omitempty"`
	RawAnyAllConditions             *kyvernov1.ConditionsWrapper     `json:"preconditions,omitempty"`
}

// TargetResourceSpecApplyConfiguration constructs an declarative configuration of the TargetResourceSpec type for use with
// apply.
func TargetResourceSpec() *TargetResourceSpecApplyConfiguration {
	return &TargetResourceSpecApplyConfiguration{}
}

// WithAPIVersion sets the APIVersion field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the APIVersion field is set to the value of the last call.
func (b *TargetResourceSpecApplyConfiguration) WithAPIVersion(value string) *TargetResourceSpecApplyConfiguration {
	b.ensureResourceSpecApplyConfigurationExists()
	b.APIVersion = &value
	return b
}

// WithKind sets the Kind field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Kind field is set to the value of the last call.
func (b *TargetResourceSpecApplyConfiguration) WithKind(value string) *TargetResourceSpecApplyConfiguration {
	b.ensureResourceSpecApplyConfigurationExists()
	b.Kind = &value
	return b
}

// WithNamespace sets the Namespace field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Namespace field is set to the value of the last call.
func (b *TargetResourceSpecApplyConfiguration) WithNamespace(value string) *TargetResourceSpecApplyConfiguration {
	b.ensureResourceSpecApplyConfigurationExists()
	b.Namespace = &value
	return b
}

// WithName sets the Name field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Name field is set to the value of the last call.
func (b *TargetResourceSpecApplyConfiguration) WithName(value string) *TargetResourceSpecApplyConfiguration {
	b.ensureResourceSpecApplyConfigurationExists()
	b.Name = &value
	return b
}

// WithSelector sets the Selector field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Selector field is set to the value of the last call.
func (b *TargetResourceSpecApplyConfiguration) WithSelector(value metav1.LabelSelector) *TargetResourceSpecApplyConfiguration {
	b.ensureResourceSpecApplyConfigurationExists()
	b.Selector = &value
	return b
}

// WithUID sets the UID field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the UID field is set to the value of the last call.
func (b *TargetResourceSpecApplyConfiguration) WithUID(value types.UID) *TargetResourceSpecApplyConfiguration {
	b.ensureResourceSpecApplyConfigurationExists()
	b.UID = &value
	return b
}

func (b *TargetResourceSpecApplyConfiguration) ensureResourceSpecApplyConfigurationExists() {
	if b.ResourceSpecApplyConfiguration == nil {
		b.ResourceSpecApplyConfiguration = &ResourceSpecApplyConfiguration{}
	}
}

// WithContext adds the given value to the Context field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Context field.
func (b *TargetResourceSpecApplyConfiguration) WithContext(values ...*ContextEntryApplyConfiguration) *TargetResourceSpecApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithContext")
		}
		b.Context = append(b.Context, *values[i])
	}
	return b
}

// WithRawAnyAllConditions sets the RawAnyAllConditions field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the RawAnyAllConditions field is set to the value of the last call.
func (b *TargetResourceSpecApplyConfiguration) WithRawAnyAllConditions(value kyvernov1.ConditionsWrapper) *TargetResourceSpecApplyConfiguration {
	b.RawAnyAllConditions = &value
	return b
}
