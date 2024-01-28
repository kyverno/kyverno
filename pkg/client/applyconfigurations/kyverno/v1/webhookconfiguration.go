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
	v1 "k8s.io/api/admissionregistration/v1"
)

// WebhookConfigurationApplyConfiguration represents an declarative configuration of the WebhookConfiguration type for use
// with apply.
type WebhookConfigurationApplyConfiguration struct {
	MatchConditions []v1.MatchCondition `json:"matchConditions,omitempty"`
}

// WebhookConfigurationApplyConfiguration constructs an declarative configuration of the WebhookConfiguration type for use with
// apply.
func WebhookConfiguration() *WebhookConfigurationApplyConfiguration {
	return &WebhookConfigurationApplyConfiguration{}
}

// WithMatchConditions adds the given value to the MatchConditions field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the MatchConditions field.
func (b *WebhookConfigurationApplyConfiguration) WithMatchConditions(values ...v1.MatchCondition) *WebhookConfigurationApplyConfiguration {
	for i := range values {
		b.MatchConditions = append(b.MatchConditions, values[i])
	}
	return b
}
