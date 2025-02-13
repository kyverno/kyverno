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

// IdentityApplyConfiguration represents an declarative configuration of the Identity type for use
// with apply.
type IdentityApplyConfiguration struct {
	Issuer        *string `json:"issuer,omitempty"`
	Subject       *string `json:"subject,omitempty"`
	IssuerRegExp  *string `json:"issuerRegExp,omitempty"`
	SubjectRegExp *string `json:"subjectRegExp,omitempty"`
}

// IdentityApplyConfiguration constructs an declarative configuration of the Identity type for use with
// apply.
func Identity() *IdentityApplyConfiguration {
	return &IdentityApplyConfiguration{}
}

// WithIssuer sets the Issuer field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Issuer field is set to the value of the last call.
func (b *IdentityApplyConfiguration) WithIssuer(value string) *IdentityApplyConfiguration {
	b.Issuer = &value
	return b
}

// WithSubject sets the Subject field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Subject field is set to the value of the last call.
func (b *IdentityApplyConfiguration) WithSubject(value string) *IdentityApplyConfiguration {
	b.Subject = &value
	return b
}

// WithIssuerRegExp sets the IssuerRegExp field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the IssuerRegExp field is set to the value of the last call.
func (b *IdentityApplyConfiguration) WithIssuerRegExp(value string) *IdentityApplyConfiguration {
	b.IssuerRegExp = &value
	return b
}

// WithSubjectRegExp sets the SubjectRegExp field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the SubjectRegExp field is set to the value of the last call.
func (b *IdentityApplyConfiguration) WithSubjectRegExp(value string) *IdentityApplyConfiguration {
	b.SubjectRegExp = &value
	return b
}
