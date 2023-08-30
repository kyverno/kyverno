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

// RekorApplyConfiguration represents an declarative configuration of the Rekor type for use
// with apply.
type RekorApplyConfiguration struct {
	URL         *string `json:"url,omitempty"`
	RekorPubKey *string `json:"pubkey,omitempty"`
	IgnoreTlog  *bool   `json:"ignoreTlog,omitempty"`
}

// RekorApplyConfiguration constructs an declarative configuration of the Rekor type for use with
// apply.
func Rekor() *RekorApplyConfiguration {
	return &RekorApplyConfiguration{}
}

// WithURL sets the URL field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the URL field is set to the value of the last call.
func (b *RekorApplyConfiguration) WithURL(value string) *RekorApplyConfiguration {
	b.URL = &value
	return b
}

// WithRekorPubKey sets the RekorPubKey field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the RekorPubKey field is set to the value of the last call.
func (b *RekorApplyConfiguration) WithRekorPubKey(value string) *RekorApplyConfiguration {
	b.RekorPubKey = &value
	return b
}

// WithIgnoreTlog sets the IgnoreTlog field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the IgnoreTlog field is set to the value of the last call.
func (b *RekorApplyConfiguration) WithIgnoreTlog(value bool) *RekorApplyConfiguration {
	b.IgnoreTlog = &value
	return b
}
