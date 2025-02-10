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

// ValidatingPolicySpecApplyConfiguration represents an declarative configuration of the ValidatingPolicySpec type for use
// with apply.
type ValidatingPolicySpecApplyConfiguration struct {
	v1.ValidatingAdmissionPolicySpec `json:",inline"`
	ValidationAction                 []v1.ValidationAction                   `json:"validationActions,omitempty"`
	WebhookConfiguration             *WebhookConfigurationApplyConfiguration `json:"webhookConfiguration,omitempty"`
}

// ValidatingPolicySpecApplyConfiguration constructs an declarative configuration of the ValidatingPolicySpec type for use with
// apply.
func ValidatingPolicySpec() *ValidatingPolicySpecApplyConfiguration {
	return &ValidatingPolicySpecApplyConfiguration{}
}

// WithParamKind sets the ParamKind field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ParamKind field is set to the value of the last call.
func (b *ValidatingPolicySpecApplyConfiguration) WithParamKind(value v1.ParamKind) *ValidatingPolicySpecApplyConfiguration {
	b.ParamKind = &value
	return b
}

// WithMatchConstraints sets the MatchConstraints field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the MatchConstraints field is set to the value of the last call.
func (b *ValidatingPolicySpecApplyConfiguration) WithMatchConstraints(value v1.MatchResources) *ValidatingPolicySpecApplyConfiguration {
	b.MatchConstraints = &value
	return b
}

// WithValidations adds the given value to the Validations field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Validations field.
func (b *ValidatingPolicySpecApplyConfiguration) WithValidations(values ...v1.Validation) *ValidatingPolicySpecApplyConfiguration {
	for i := range values {
		b.Validations = append(b.Validations, values[i])
	}
	return b
}

// WithFailurePolicy sets the FailurePolicy field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the FailurePolicy field is set to the value of the last call.
func (b *ValidatingPolicySpecApplyConfiguration) WithFailurePolicy(value v1.FailurePolicyType) *ValidatingPolicySpecApplyConfiguration {
	b.FailurePolicy = &value
	return b
}

// WithAuditAnnotations adds the given value to the AuditAnnotations field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the AuditAnnotations field.
func (b *ValidatingPolicySpecApplyConfiguration) WithAuditAnnotations(values ...v1.AuditAnnotation) *ValidatingPolicySpecApplyConfiguration {
	for i := range values {
		b.AuditAnnotations = append(b.AuditAnnotations, values[i])
	}
	return b
}

// WithMatchConditions adds the given value to the MatchConditions field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the MatchConditions field.
func (b *ValidatingPolicySpecApplyConfiguration) WithMatchConditions(values ...v1.MatchCondition) *ValidatingPolicySpecApplyConfiguration {
	for i := range values {
		b.MatchConditions = append(b.MatchConditions, values[i])
	}
	return b
}

// WithVariables adds the given value to the Variables field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Variables field.
func (b *ValidatingPolicySpecApplyConfiguration) WithVariables(values ...v1.Variable) *ValidatingPolicySpecApplyConfiguration {
	for i := range values {
		b.Variables = append(b.Variables, values[i])
	}
	return b
}

// WithValidationAction adds the given value to the ValidationAction field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the ValidationAction field.
func (b *ValidatingPolicySpecApplyConfiguration) WithValidationAction(values ...v1.ValidationAction) *ValidatingPolicySpecApplyConfiguration {
	for i := range values {
		b.ValidationAction = append(b.ValidationAction, values[i])
	}
	return b
}

// WithWebhookConfiguration sets the WebhookConfiguration field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the WebhookConfiguration field is set to the value of the last call.
func (b *ValidatingPolicySpecApplyConfiguration) WithWebhookConfiguration(value *WebhookConfigurationApplyConfiguration) *ValidatingPolicySpecApplyConfiguration {
	b.WebhookConfiguration = value
	return b
}
