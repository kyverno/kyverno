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
	v1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// RuleApplyConfiguration represents an declarative configuration of the Rule type for use
// with apply.
type RuleApplyConfiguration struct {
	Name                   *string                               `json:"name,omitempty"`
	Context                []ContextEntryApplyConfiguration      `json:"context,omitempty"`
	MatchResources         *MatchResourcesApplyConfiguration     `json:"match,omitempty"`
	ExcludeResources       *MatchResourcesApplyConfiguration     `json:"exclude,omitempty"`
	ImageExtractors        *kyvernov1.ImageExtractorConfigs      `json:"imageExtractors,omitempty"`
	RawAnyAllConditions    *apiextensionsv1.JSON                 `json:"preconditions,omitempty"`
	CELPreconditions       []v1alpha1.MatchCondition             `json:"celPreconditions,omitempty"`
	Mutation               *MutationApplyConfiguration           `json:"mutate,omitempty"`
	Validation             *ValidationApplyConfiguration         `json:"validate,omitempty"`
	Generation             *GenerationApplyConfiguration         `json:"generate,omitempty"`
	VerifyImages           []ImageVerificationApplyConfiguration `json:"verifyImages,omitempty"`
	SkipBackgroundRequests *bool                                 `json:"skipBackgroundRequests,omitempty"`
}

// RuleApplyConfiguration constructs an declarative configuration of the Rule type for use with
// apply.
func Rule() *RuleApplyConfiguration {
	return &RuleApplyConfiguration{}
}

// WithName sets the Name field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Name field is set to the value of the last call.
func (b *RuleApplyConfiguration) WithName(value string) *RuleApplyConfiguration {
	b.Name = &value
	return b
}

// WithContext adds the given value to the Context field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Context field.
func (b *RuleApplyConfiguration) WithContext(values ...*ContextEntryApplyConfiguration) *RuleApplyConfiguration {
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
func (b *RuleApplyConfiguration) WithMatchResources(value *MatchResourcesApplyConfiguration) *RuleApplyConfiguration {
	b.MatchResources = value
	return b
}

// WithExcludeResources sets the ExcludeResources field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ExcludeResources field is set to the value of the last call.
func (b *RuleApplyConfiguration) WithExcludeResources(value *MatchResourcesApplyConfiguration) *RuleApplyConfiguration {
	b.ExcludeResources = value
	return b
}

// WithImageExtractors sets the ImageExtractors field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ImageExtractors field is set to the value of the last call.
func (b *RuleApplyConfiguration) WithImageExtractors(value kyvernov1.ImageExtractorConfigs) *RuleApplyConfiguration {
	b.ImageExtractors = &value
	return b
}

// WithRawAnyAllConditions sets the RawAnyAllConditions field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the RawAnyAllConditions field is set to the value of the last call.
func (b *RuleApplyConfiguration) WithRawAnyAllConditions(value apiextensionsv1.JSON) *RuleApplyConfiguration {
	b.RawAnyAllConditions = &value
	return b
}

// WithCELPreconditions adds the given value to the CELPreconditions field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the CELPreconditions field.
func (b *RuleApplyConfiguration) WithCELPreconditions(values ...v1alpha1.MatchCondition) *RuleApplyConfiguration {
	for i := range values {
		b.CELPreconditions = append(b.CELPreconditions, values[i])
	}
	return b
}

// WithMutation sets the Mutation field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Mutation field is set to the value of the last call.
func (b *RuleApplyConfiguration) WithMutation(value *MutationApplyConfiguration) *RuleApplyConfiguration {
	b.Mutation = value
	return b
}

// WithValidation sets the Validation field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Validation field is set to the value of the last call.
func (b *RuleApplyConfiguration) WithValidation(value *ValidationApplyConfiguration) *RuleApplyConfiguration {
	b.Validation = value
	return b
}

// WithGeneration sets the Generation field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Generation field is set to the value of the last call.
func (b *RuleApplyConfiguration) WithGeneration(value *GenerationApplyConfiguration) *RuleApplyConfiguration {
	b.Generation = value
	return b
}

// WithVerifyImages adds the given value to the VerifyImages field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the VerifyImages field.
func (b *RuleApplyConfiguration) WithVerifyImages(values ...*ImageVerificationApplyConfiguration) *RuleApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithVerifyImages")
		}
		b.VerifyImages = append(b.VerifyImages, *values[i])
	}
	return b
}

// WithSkipBackgroundRequests sets the SkipBackgroundRequests field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the SkipBackgroundRequests field is set to the value of the last call.
func (b *RuleApplyConfiguration) WithSkipBackgroundRequests(value bool) *RuleApplyConfiguration {
	b.SkipBackgroundRequests = &value
	return b
}
