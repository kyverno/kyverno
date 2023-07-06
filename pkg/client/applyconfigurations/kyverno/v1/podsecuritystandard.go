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

// PodSecurityStandardApplyConfiguration represents an declarative configuration of the PodSecurityStandard type for use
// with apply.
type PodSecurityStandardApplyConfiguration struct {
	ControlName *string  `json:"controlName,omitempty"`
	Images      []string `json:"images,omitempty"`
}

// PodSecurityStandardApplyConfiguration constructs an declarative configuration of the PodSecurityStandard type for use with
// apply.
func PodSecurityStandard() *PodSecurityStandardApplyConfiguration {
	return &PodSecurityStandardApplyConfiguration{}
}

// WithControlName sets the ControlName field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the ControlName field is set to the value of the last call.
func (b *PodSecurityStandardApplyConfiguration) WithControlName(value string) *PodSecurityStandardApplyConfiguration {
	b.ControlName = &value
	return b
}

// WithImages adds the given value to the Images field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Images field.
func (b *PodSecurityStandardApplyConfiguration) WithImages(values ...string) *PodSecurityStandardApplyConfiguration {
	for i := range values {
		b.Images = append(b.Images, values[i])
	}
	return b
}
