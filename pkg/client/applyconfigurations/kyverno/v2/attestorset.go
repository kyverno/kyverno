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

// AttestorSetApplyConfiguration represents an declarative configuration of the AttestorSet type for use
// with apply.
type AttestorSetApplyConfiguration struct {
	Count   *int                         `json:"count,omitempty"`
	Entries []AttestorApplyConfiguration `json:"entries,omitempty"`
}

// AttestorSetApplyConfiguration constructs an declarative configuration of the AttestorSet type for use with
// apply.
func AttestorSet() *AttestorSetApplyConfiguration {
	return &AttestorSetApplyConfiguration{}
}

// WithCount sets the Count field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Count field is set to the value of the last call.
func (b *AttestorSetApplyConfiguration) WithCount(value int) *AttestorSetApplyConfiguration {
	b.Count = &value
	return b
}

// WithEntries adds the given value to the Entries field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, values provided by each call will be appended to the Entries field.
func (b *AttestorSetApplyConfiguration) WithEntries(values ...*AttestorApplyConfiguration) *AttestorSetApplyConfiguration {
	for i := range values {
		if values[i] == nil {
			panic("nil value passed to WithEntries")
		}
		b.Entries = append(b.Entries, *values[i])
	}
	return b
}
