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

package v2

import (
	v1 "github.com/kyverno/kyverno/pkg/client/applyconfigurations/kyverno/v1"
	v2beta1 "github.com/kyverno/kyverno/pkg/client/applyconfigurations/kyverno/v2beta1"
)

// CleanupPolicySpecApplyConfiguration represents an declarative configuration of the CleanupPolicySpec type for use
// with apply.
type CleanupPolicySpecApplyConfiguration struct {
	Context          []v1.ContextEntryApplyConfiguration       `json:"context,omitempty"`
	MatchResources   *v2beta1.MatchResourcesApplyConfiguration `json:"match,omitempty"`
	ExcludeResources *v2beta1.MatchResourcesApplyConfiguration `json:"exclude,omitempty"`
	Schedule         *string                                   `json:"schedule,omitempty"`
	Conditions       *AnyAllConditionsApplyConfiguration       `json:"conditions,omitempty"`
}

// CleanupPolicySpecApplyConfiguration constructs an declarative configuration of the CleanupPolicySpec type for use with
// apply.
func CleanupPolicySpec() *CleanupPolicySpecApplyConfiguration {
	return &CleanupPolicySpecApplyConfiguration{}
}

// WithContext adds the given value to the Context field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Context field.
func (b *CleanupPolicySpecApplyConfiguration) WithContext(values ...*v1.ContextEntryApplyConfiguration) *CleanupPolicySpecApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithContext")
		}
		b.Context = append(b.Context, *values[i])
	}
	return b
}

// WithMatchResources sets the MatchResources field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the MatchResources field is set to the value of the last call.
func (b *CleanupPolicySpecApplyConfiguration) WithMatchResources(value *v2beta1.MatchResourcesApplyConfiguration) *CleanupPolicySpecApplyConfiguration {
	b.MatchResources = value
	return b
}

// WithExcludeResources sets the ExcludeResources field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ExcludeResources field is set to the value of the last call.
func (b *CleanupPolicySpecApplyConfiguration) WithExcludeResources(value *v2beta1.MatchResourcesApplyConfiguration) *CleanupPolicySpecApplyConfiguration {
	b.ExcludeResources = value
	return b
}

// WithSchedule sets the Schedule field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Schedule field is set to the value of the last call.
func (b *CleanupPolicySpecApplyConfiguration) WithSchedule(value string) *CleanupPolicySpecApplyConfiguration {
	b.Schedule = &value
	return b
}

// WithConditions sets the Conditions field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Conditions field is set to the value of the last call.
func (b *CleanupPolicySpecApplyConfiguration) WithConditions(value *AnyAllConditionsApplyConfiguration) *CleanupPolicySpecApplyConfiguration {
	b.Conditions = value
	return b
}
