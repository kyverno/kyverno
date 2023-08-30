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

// CertificateAttestorApplyConfiguration represents an declarative configuration of the CertificateAttestor type for use
// with apply.
type CertificateAttestorApplyConfiguration struct {
	Certificate      *string                  `json:"cert,omitempty"`
	CertificateChain *string                  `json:"certChain,omitempty"`
	Rekor            *RekorApplyConfiguration `json:"rekor,omitempty"`
	CTLog            *CTLogApplyConfiguration `json:"ctlog,omitempty"`
}

// CertificateAttestorApplyConfiguration constructs an declarative configuration of the CertificateAttestor type for use with
// apply.
func CertificateAttestor() *CertificateAttestorApplyConfiguration {
	return &CertificateAttestorApplyConfiguration{}
}

// WithCertificate sets the Certificate field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Certificate field is set to the value of the last call.
func (b *CertificateAttestorApplyConfiguration) WithCertificate(value string) *CertificateAttestorApplyConfiguration {
	b.Certificate = &value
	return b
}

// WithCertificateChain sets the CertificateChain field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the CertificateChain field is set to the value of the last call.
func (b *CertificateAttestorApplyConfiguration) WithCertificateChain(value string) *CertificateAttestorApplyConfiguration {
	b.CertificateChain = &value
	return b
}

// WithRekor sets the Rekor field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Rekor field is set to the value of the last call.
func (b *CertificateAttestorApplyConfiguration) WithRekor(value *RekorApplyConfiguration) *CertificateAttestorApplyConfiguration {
	b.Rekor = value
	return b
}

// WithCTLog sets the CTLog field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the CTLog field is set to the value of the last call.
func (b *CertificateAttestorApplyConfiguration) WithCTLog(value *CTLogApplyConfiguration) *CertificateAttestorApplyConfiguration {
	b.CTLog = value
	return b
}
