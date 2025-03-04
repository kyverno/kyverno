//go:build !ignore_autogenerated
// +build !ignore_autogenerated

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

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha1

import (
	v1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AdmissionConfiguration) DeepCopyInto(out *AdmissionConfiguration) {
	*out = *in
	if in.Enabled != nil {
		in, out := &in.Enabled, &out.Enabled
		*out = new(bool)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AdmissionConfiguration.
func (in *AdmissionConfiguration) DeepCopy() *AdmissionConfiguration {
	if in == nil {
		return nil
	}
	out := new(AdmissionConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Attestation) DeepCopyInto(out *Attestation) {
	*out = *in
	if in.InToto != nil {
		in, out := &in.InToto, &out.InToto
		*out = new(InToto)
		**out = **in
	}
	if in.Referrer != nil {
		in, out := &in.Referrer, &out.Referrer
		*out = new(Referrer)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Attestation.
func (in *Attestation) DeepCopy() *Attestation {
	if in == nil {
		return nil
	}
	out := new(Attestation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Attestor) DeepCopyInto(out *Attestor) {
	*out = *in
	if in.Cosign != nil {
		in, out := &in.Cosign, &out.Cosign
		*out = new(Cosign)
		(*in).DeepCopyInto(*out)
	}
	if in.Notary != nil {
		in, out := &in.Notary, &out.Notary
		*out = new(Notary)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Attestor.
func (in *Attestor) DeepCopy() *Attestor {
	if in == nil {
		return nil
	}
	out := new(Attestor)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AutogenRule) DeepCopyInto(out *AutogenRule) {
	*out = *in
	if in.MatchConstraints != nil {
		in, out := &in.MatchConstraints, &out.MatchConstraints
		*out = new(v1.MatchResources)
		(*in).DeepCopyInto(*out)
	}
	if in.MatchConditions != nil {
		in, out := &in.MatchConditions, &out.MatchConditions
		*out = make([]v1.MatchCondition, len(*in))
		copy(*out, *in)
	}
	if in.Validations != nil {
		in, out := &in.Validations, &out.Validations
		*out = make([]v1.Validation, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.AuditAnnotation != nil {
		in, out := &in.AuditAnnotation, &out.AuditAnnotation
		*out = make([]v1.AuditAnnotation, len(*in))
		copy(*out, *in)
	}
	if in.Variables != nil {
		in, out := &in.Variables, &out.Variables
		*out = make([]v1.Variable, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AutogenRule.
func (in *AutogenRule) DeepCopy() *AutogenRule {
	if in == nil {
		return nil
	}
	out := new(AutogenRule)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AutogenStatus) DeepCopyInto(out *AutogenStatus) {
	*out = *in
	if in.Rules != nil {
		in, out := &in.Rules, &out.Rules
		*out = make([]AutogenRule, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AutogenStatus.
func (in *AutogenStatus) DeepCopy() *AutogenStatus {
	if in == nil {
		return nil
	}
	out := new(AutogenStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BackgroundConfiguration) DeepCopyInto(out *BackgroundConfiguration) {
	*out = *in
	if in.Enabled != nil {
		in, out := &in.Enabled, &out.Enabled
		*out = new(bool)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BackgroundConfiguration.
func (in *BackgroundConfiguration) DeepCopy() *BackgroundConfiguration {
	if in == nil {
		return nil
	}
	out := new(BackgroundConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CELPolicyException) DeepCopyInto(out *CELPolicyException) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CELPolicyException.
func (in *CELPolicyException) DeepCopy() *CELPolicyException {
	if in == nil {
		return nil
	}
	out := new(CELPolicyException)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CELPolicyException) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CELPolicyExceptionList) DeepCopyInto(out *CELPolicyExceptionList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]CELPolicyException, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CELPolicyExceptionList.
func (in *CELPolicyExceptionList) DeepCopy() *CELPolicyExceptionList {
	if in == nil {
		return nil
	}
	out := new(CELPolicyExceptionList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CELPolicyExceptionList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CELPolicyExceptionSpec) DeepCopyInto(out *CELPolicyExceptionSpec) {
	*out = *in
	if in.PolicyRefs != nil {
		in, out := &in.PolicyRefs, &out.PolicyRefs
		*out = make([]PolicyRef, len(*in))
		copy(*out, *in)
	}
	if in.MatchConditions != nil {
		in, out := &in.MatchConditions, &out.MatchConditions
		*out = make([]v1.MatchCondition, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CELPolicyExceptionSpec.
func (in *CELPolicyExceptionSpec) DeepCopy() *CELPolicyExceptionSpec {
	if in == nil {
		return nil
	}
	out := new(CELPolicyExceptionSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CTLog) DeepCopyInto(out *CTLog) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CTLog.
func (in *CTLog) DeepCopy() *CTLog {
	if in == nil {
		return nil
	}
	out := new(CTLog)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Certificate) DeepCopyInto(out *Certificate) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Certificate.
func (in *Certificate) DeepCopy() *Certificate {
	if in == nil {
		return nil
	}
	out := new(Certificate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Cosign) DeepCopyInto(out *Cosign) {
	*out = *in
	if in.Key != nil {
		in, out := &in.Key, &out.Key
		*out = new(Key)
		(*in).DeepCopyInto(*out)
	}
	if in.Keyless != nil {
		in, out := &in.Keyless, &out.Keyless
		*out = new(Keyless)
		(*in).DeepCopyInto(*out)
	}
	if in.Certificate != nil {
		in, out := &in.Certificate, &out.Certificate
		*out = new(Certificate)
		**out = **in
	}
	if in.Source != nil {
		in, out := &in.Source, &out.Source
		*out = new(Source)
		(*in).DeepCopyInto(*out)
	}
	if in.CTLog != nil {
		in, out := &in.CTLog, &out.CTLog
		*out = new(CTLog)
		**out = **in
	}
	if in.TUF != nil {
		in, out := &in.TUF, &out.TUF
		*out = new(TUF)
		**out = **in
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Cosign.
func (in *Cosign) DeepCopy() *Cosign {
	if in == nil {
		return nil
	}
	out := new(Cosign)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Credentials) DeepCopyInto(out *Credentials) {
	*out = *in
	if in.Providers != nil {
		in, out := &in.Providers, &out.Providers
		*out = make([]CredentialsProvidersType, len(*in))
		copy(*out, *in)
	}
	if in.Secrets != nil {
		in, out := &in.Secrets, &out.Secrets
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Credentials.
func (in *Credentials) DeepCopy() *Credentials {
	if in == nil {
		return nil
	}
	out := new(Credentials)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *EvaluationConfiguration) DeepCopyInto(out *EvaluationConfiguration) {
	*out = *in
	if in.Admission != nil {
		in, out := &in.Admission, &out.Admission
		*out = new(AdmissionConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.Background != nil {
		in, out := &in.Background, &out.Background
		*out = new(BackgroundConfiguration)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new EvaluationConfiguration.
func (in *EvaluationConfiguration) DeepCopy() *EvaluationConfiguration {
	if in == nil {
		return nil
	}
	out := new(EvaluationConfiguration)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Identity) DeepCopyInto(out *Identity) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Identity.
func (in *Identity) DeepCopy() *Identity {
	if in == nil {
		return nil
	}
	out := new(Identity)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Image) DeepCopyInto(out *Image) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Image.
func (in *Image) DeepCopy() *Image {
	if in == nil {
		return nil
	}
	out := new(Image)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ImageRule) DeepCopyInto(out *ImageRule) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ImageRule.
func (in *ImageRule) DeepCopy() *ImageRule {
	if in == nil {
		return nil
	}
	out := new(ImageRule)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ImageVerificationPolicy) DeepCopyInto(out *ImageVerificationPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ImageVerificationPolicy.
func (in *ImageVerificationPolicy) DeepCopy() *ImageVerificationPolicy {
	if in == nil {
		return nil
	}
	out := new(ImageVerificationPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ImageVerificationPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ImageVerificationPolicyList) DeepCopyInto(out *ImageVerificationPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ImageVerificationPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ImageVerificationPolicyList.
func (in *ImageVerificationPolicyList) DeepCopy() *ImageVerificationPolicyList {
	if in == nil {
		return nil
	}
	out := new(ImageVerificationPolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ImageVerificationPolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ImageVerificationPolicySpec) DeepCopyInto(out *ImageVerificationPolicySpec) {
	*out = *in
	if in.MatchConstraints != nil {
		in, out := &in.MatchConstraints, &out.MatchConstraints
		*out = new(v1.MatchResources)
		(*in).DeepCopyInto(*out)
	}
	if in.FailurePolicy != nil {
		in, out := &in.FailurePolicy, &out.FailurePolicy
		*out = new(v1.FailurePolicyType)
		**out = **in
	}
	if in.ValidationAction != nil {
		in, out := &in.ValidationAction, &out.ValidationAction
		*out = make([]v1.ValidationAction, len(*in))
		copy(*out, *in)
	}
	if in.MatchConditions != nil {
		in, out := &in.MatchConditions, &out.MatchConditions
		*out = make([]v1.MatchCondition, len(*in))
		copy(*out, *in)
	}
	if in.Variables != nil {
		in, out := &in.Variables, &out.Variables
		*out = make([]v1.Variable, len(*in))
		copy(*out, *in)
	}
	if in.ImageRules != nil {
		in, out := &in.ImageRules, &out.ImageRules
		*out = make([]ImageRule, len(*in))
		copy(*out, *in)
	}
	if in.MutateDigest != nil {
		in, out := &in.MutateDigest, &out.MutateDigest
		*out = new(bool)
		**out = **in
	}
	if in.VerifyDigest != nil {
		in, out := &in.VerifyDigest, &out.VerifyDigest
		*out = new(bool)
		**out = **in
	}
	if in.Required != nil {
		in, out := &in.Required, &out.Required
		*out = new(bool)
		**out = **in
	}
	if in.Credentials != nil {
		in, out := &in.Credentials, &out.Credentials
		*out = new(Credentials)
		(*in).DeepCopyInto(*out)
	}
	if in.Images != nil {
		in, out := &in.Images, &out.Images
		*out = make([]Image, len(*in))
		copy(*out, *in)
	}
	if in.Attestors != nil {
		in, out := &in.Attestors, &out.Attestors
		*out = make([]Attestor, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Attestations != nil {
		in, out := &in.Attestations, &out.Attestations
		*out = make([]Attestation, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Verifications != nil {
		in, out := &in.Verifications, &out.Verifications
		*out = make([]v1.Validation, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.WebhookConfiguration != nil {
		in, out := &in.WebhookConfiguration, &out.WebhookConfiguration
		*out = new(WebhookConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.EvaluationConfiguration != nil {
		in, out := &in.EvaluationConfiguration, &out.EvaluationConfiguration
		*out = new(EvaluationConfiguration)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ImageVerificationPolicySpec.
func (in *ImageVerificationPolicySpec) DeepCopy() *ImageVerificationPolicySpec {
	if in == nil {
		return nil
	}
	out := new(ImageVerificationPolicySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *InToto) DeepCopyInto(out *InToto) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new InToto.
func (in *InToto) DeepCopy() *InToto {
	if in == nil {
		return nil
	}
	out := new(InToto)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Key) DeepCopyInto(out *Key) {
	*out = *in
	if in.SecretRef != nil {
		in, out := &in.SecretRef, &out.SecretRef
		*out = new(corev1.SecretReference)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Key.
func (in *Key) DeepCopy() *Key {
	if in == nil {
		return nil
	}
	out := new(Key)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Keyless) DeepCopyInto(out *Keyless) {
	*out = *in
	if in.Identities != nil {
		in, out := &in.Identities, &out.Identities
		*out = make([]Identity, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Keyless.
func (in *Keyless) DeepCopy() *Keyless {
	if in == nil {
		return nil
	}
	out := new(Keyless)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Notary) DeepCopyInto(out *Notary) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Notary.
func (in *Notary) DeepCopy() *Notary {
	if in == nil {
		return nil
	}
	out := new(Notary)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PolicyRef) DeepCopyInto(out *PolicyRef) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PolicyRef.
func (in *PolicyRef) DeepCopy() *PolicyRef {
	if in == nil {
		return nil
	}
	out := new(PolicyRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PolicyStatus) DeepCopyInto(out *PolicyStatus) {
	*out = *in
	if in.Ready != nil {
		in, out := &in.Ready, &out.Ready
		*out = new(bool)
		**out = **in
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.Autogen.DeepCopyInto(&out.Autogen)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PolicyStatus.
func (in *PolicyStatus) DeepCopy() *PolicyStatus {
	if in == nil {
		return nil
	}
	out := new(PolicyStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Referrer) DeepCopyInto(out *Referrer) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Referrer.
func (in *Referrer) DeepCopy() *Referrer {
	if in == nil {
		return nil
	}
	out := new(Referrer)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Source) DeepCopyInto(out *Source) {
	*out = *in
	if in.SignaturePullSecrets != nil {
		in, out := &in.SignaturePullSecrets, &out.SignaturePullSecrets
		*out = make([]corev1.LocalObjectReference, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Source.
func (in *Source) DeepCopy() *Source {
	if in == nil {
		return nil
	}
	out := new(Source)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TUF) DeepCopyInto(out *TUF) {
	*out = *in
	out.Root = in.Root
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TUF.
func (in *TUF) DeepCopy() *TUF {
	if in == nil {
		return nil
	}
	out := new(TUF)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TUFRoot) DeepCopyInto(out *TUFRoot) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TUFRoot.
func (in *TUFRoot) DeepCopy() *TUFRoot {
	if in == nil {
		return nil
	}
	out := new(TUFRoot)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ValidatingPolicy) DeepCopyInto(out *ValidatingPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ValidatingPolicy.
func (in *ValidatingPolicy) DeepCopy() *ValidatingPolicy {
	if in == nil {
		return nil
	}
	out := new(ValidatingPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ValidatingPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ValidatingPolicyList) DeepCopyInto(out *ValidatingPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ValidatingPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ValidatingPolicyList.
func (in *ValidatingPolicyList) DeepCopy() *ValidatingPolicyList {
	if in == nil {
		return nil
	}
	out := new(ValidatingPolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ValidatingPolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ValidatingPolicySpec) DeepCopyInto(out *ValidatingPolicySpec) {
	*out = *in
	if in.MatchConstraints != nil {
		in, out := &in.MatchConstraints, &out.MatchConstraints
		*out = new(v1.MatchResources)
		(*in).DeepCopyInto(*out)
	}
	if in.Validations != nil {
		in, out := &in.Validations, &out.Validations
		*out = make([]v1.Validation, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.FailurePolicy != nil {
		in, out := &in.FailurePolicy, &out.FailurePolicy
		*out = new(v1.FailurePolicyType)
		**out = **in
	}
	if in.AuditAnnotations != nil {
		in, out := &in.AuditAnnotations, &out.AuditAnnotations
		*out = make([]v1.AuditAnnotation, len(*in))
		copy(*out, *in)
	}
	if in.MatchConditions != nil {
		in, out := &in.MatchConditions, &out.MatchConditions
		*out = make([]v1.MatchCondition, len(*in))
		copy(*out, *in)
	}
	if in.Variables != nil {
		in, out := &in.Variables, &out.Variables
		*out = make([]v1.Variable, len(*in))
		copy(*out, *in)
	}
	if in.Generate != nil {
		in, out := &in.Generate, &out.Generate
		*out = new(bool)
		**out = **in
	}
	if in.ValidationAction != nil {
		in, out := &in.ValidationAction, &out.ValidationAction
		*out = make([]v1.ValidationAction, len(*in))
		copy(*out, *in)
	}
	if in.WebhookConfiguration != nil {
		in, out := &in.WebhookConfiguration, &out.WebhookConfiguration
		*out = new(WebhookConfiguration)
		(*in).DeepCopyInto(*out)
	}
	if in.EvaluationConfiguration != nil {
		in, out := &in.EvaluationConfiguration, &out.EvaluationConfiguration
		*out = new(EvaluationConfiguration)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ValidatingPolicySpec.
func (in *ValidatingPolicySpec) DeepCopy() *ValidatingPolicySpec {
	if in == nil {
		return nil
	}
	out := new(ValidatingPolicySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *WebhookConfiguration) DeepCopyInto(out *WebhookConfiguration) {
	*out = *in
	if in.TimeoutSeconds != nil {
		in, out := &in.TimeoutSeconds, &out.TimeoutSeconds
		*out = new(int32)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new WebhookConfiguration.
func (in *WebhookConfiguration) DeepCopy() *WebhookConfiguration {
	if in == nil {
		return nil
	}
	out := new(WebhookConfiguration)
	in.DeepCopyInto(out)
	return out
}
