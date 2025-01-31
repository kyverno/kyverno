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

package v2alpha1

import (
	v1 "k8s.io/api/admissionregistration/v1"
)

// CELPolicyExceptionSpecApplyConfiguration represents an declarative configuration of the CELPolicyExceptionSpec type for use
// with apply.
type CELPolicyExceptionSpecApplyConfiguration struct {
	PolicyRefs       []PolicyRefApplyConfiguration `json:"policyRefs,omitempty"`
	MatchConstraints *v1.MatchResources            `json:"matchConstraints,omitempty"`
	MatchConditions  []v1.MatchCondition           `json:"matchConditions,omitempty"`
}

// CELPolicyExceptionSpecApplyConfiguration constructs an declarative configuration of the CELPolicyExceptionSpec type for use with
// apply.
func CELPolicyExceptionSpec() *CELPolicyExceptionSpecApplyConfiguration {
	return &CELPolicyExceptionSpecApplyConfiguration{}
}

// WithPolicyRefs adds the given value to the PolicyRefs field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the PolicyRefs field.
func (b *CELPolicyExceptionSpecApplyConfiguration) WithPolicyRefs(values ...*PolicyRefApplyConfiguration) *CELPolicyExceptionSpecApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithPolicyRefs")
		}
		b.PolicyRefs = append(b.PolicyRefs, *values[i])
	}
	return b
}

// WithMatchConstraints sets the MatchConstraints field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the MatchConstraints field is set to the value of the last call.
func (b *CELPolicyExceptionSpecApplyConfiguration) WithMatchConstraints(value v1.MatchResources) *CELPolicyExceptionSpecApplyConfiguration {
	b.MatchConstraints = &value
	return b
}

// WithMatchConditions adds the given value to the MatchConditions field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the MatchConditions field.
func (b *CELPolicyExceptionSpecApplyConfiguration) WithMatchConditions(values ...v1.MatchCondition) *CELPolicyExceptionSpecApplyConfiguration {
	for i := range values {
		b.MatchConditions = append(b.MatchConditions, values[i])
	}
	return b
}
