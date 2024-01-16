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

// KeylessAttestorApplyConfiguration represents an declarative configuration of the KeylessAttestor type for use
// with apply.
type KeylessAttestorApplyConfiguration struct {
	Rekor                *RekorApplyConfiguration `json:"rekor,omitempty"`
	CTLog                *CTLogApplyConfiguration `json:"ctlog,omitempty"`
	Issuer               *string                  `json:"issuer,omitempty"`
	Subject              *string                  `json:"subject,omitempty"`
	Roots                *string                  `json:"roots,omitempty"`
	AdditionalExtensions map[string]string        `json:"additionalExtensions,omitempty"`
}

// KeylessAttestorApplyConfiguration constructs an declarative configuration of the KeylessAttestor type for use with
// apply.
func KeylessAttestor() *KeylessAttestorApplyConfiguration {
	return &KeylessAttestorApplyConfiguration{}
}

// WithRekor sets the Rekor field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Rekor field is set to the value of the last call.
func (b *KeylessAttestorApplyConfiguration) WithRekor(value *RekorApplyConfiguration) *KeylessAttestorApplyConfiguration {
	b.Rekor = value
	return b
}

// WithCTLog sets the CTLog field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the CTLog field is set to the value of the last call.
func (b *KeylessAttestorApplyConfiguration) WithCTLog(value *CTLogApplyConfiguration) *KeylessAttestorApplyConfiguration {
	b.CTLog = value
	return b
}

// WithIssuer sets the Issuer field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Issuer field is set to the value of the last call.
func (b *KeylessAttestorApplyConfiguration) WithIssuer(value string) *KeylessAttestorApplyConfiguration {
	b.Issuer = &value
	return b
}

// WithSubject sets the Subject field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Subject field is set to the value of the last call.
func (b *KeylessAttestorApplyConfiguration) WithSubject(value string) *KeylessAttestorApplyConfiguration {
	b.Subject = &value
	return b
}

// WithRoots sets the Roots field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Roots field is set to the value of the last call.
func (b *KeylessAttestorApplyConfiguration) WithRoots(value string) *KeylessAttestorApplyConfiguration {
	b.Roots = &value
	return b
}

// WithAdditionalExtensions puts the entries into the AdditionalExtensions field in the declarative configuration
// and returns the receiver, so that objects can be build by chaining "With" function invocations.
// If called multiple times, the entries provided by each call will be put on the AdditionalExtensions field,
// overwriting an existing map entries in AdditionalExtensions field with the same key.
func (b *KeylessAttestorApplyConfiguration) WithAdditionalExtensions(entries map[string]string) *KeylessAttestorApplyConfiguration {
	if b.AdditionalExtensions == nil && len(entries) > 0 {
		b.AdditionalExtensions = make(map[string]string, len(entries))
	}
	for k, v := range entries {
		b.AdditionalExtensions[k] = v
	}
	return b
}
