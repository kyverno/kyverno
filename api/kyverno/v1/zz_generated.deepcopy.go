//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
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

// Code generated by controller-gen. DO NOT EDIT.

package v1

import (
	"github.com/kyverno/kyverno/pkg/utils/kube"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APICall) DeepCopyInto(out *APICall) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APICall.
func (in *APICall) DeepCopy() *APICall {
	if in == nil {
		return nil
	}
	out := new(APICall)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AdmissionRequestInfoObject) DeepCopyInto(out *AdmissionRequestInfoObject) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AdmissionRequestInfoObject.
func (in *AdmissionRequestInfoObject) DeepCopy() *AdmissionRequestInfoObject {
	if in == nil {
		return nil
	}
	out := new(AdmissionRequestInfoObject)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AnyAllConditions) DeepCopyInto(out *AnyAllConditions) {
	*out = *in
	if in.AnyConditions != nil {
		in, out := &in.AnyConditions, &out.AnyConditions
		*out = make([]Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.AllConditions != nil {
		in, out := &in.AllConditions, &out.AllConditions
		*out = make([]Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AnyAllConditions.
func (in *AnyAllConditions) DeepCopy() *AnyAllConditions {
	if in == nil {
		return nil
	}
	out := new(AnyAllConditions)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Attestation) DeepCopyInto(out *Attestation) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]*AnyAllConditions, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(AnyAllConditions)
				(*in).DeepCopyInto(*out)
			}
		}
	}
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
	if in.StaticKey != nil {
		in, out := &in.StaticKey, &out.StaticKey
		*out = new(StaticKeyAttestor)
		**out = **in
	}
	if in.Keyless != nil {
		in, out := &in.Keyless, &out.Keyless
		*out = new(KeylessAttestor)
		(*in).DeepCopyInto(*out)
	}
	if in.Attestor != nil {
		in, out := &in.Attestor, &out.Attestor
		*out = new(apiextensionsv1.JSON)
		(*in).DeepCopyInto(*out)
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
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
func (in *AttestorSet) DeepCopyInto(out *AttestorSet) {
	*out = *in
	if in.Count != nil {
		in, out := &in.Count, &out.Count
		*out = new(int)
		**out = **in
	}
	if in.Entries != nil {
		in, out := &in.Entries, &out.Entries
		*out = make([]*Attestor, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(Attestor)
				(*in).DeepCopyInto(*out)
			}
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AttestorSet.
func (in *AttestorSet) DeepCopy() *AttestorSet {
	if in == nil {
		return nil
	}
	out := new(AttestorSet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AutogenStatus) DeepCopyInto(out *AutogenStatus) {
	*out = *in
	if in.Requested != nil {
		in, out := &in.Requested, &out.Requested
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Supported != nil {
		in, out := &in.Supported, &out.Supported
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Activated != nil {
		in, out := &in.Activated, &out.Activated
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
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
func (in *CTLog) DeepCopyInto(out *CTLog) {
	*out = *in
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
func (in *CloneFrom) DeepCopyInto(out *CloneFrom) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CloneFrom.
func (in *CloneFrom) DeepCopy() *CloneFrom {
	if in == nil {
		return nil
	}
	out := new(CloneFrom)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterPolicy) DeepCopyInto(out *ClusterPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterPolicy.
func (in *ClusterPolicy) DeepCopy() *ClusterPolicy {
	if in == nil {
		return nil
	}
	out := new(ClusterPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterPolicyList) DeepCopyInto(out *ClusterPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterPolicyList.
func (in *ClusterPolicyList) DeepCopy() *ClusterPolicyList {
	if in == nil {
		return nil
	}
	out := new(ClusterPolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterPolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Condition) DeepCopyInto(out *Condition) {
	*out = *in
	if in.RawKey != nil {
		in, out := &in.RawKey, &out.RawKey
		*out = new(apiextensionsv1.JSON)
		(*in).DeepCopyInto(*out)
	}
	if in.RawValue != nil {
		in, out := &in.RawValue, &out.RawValue
		*out = new(apiextensionsv1.JSON)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Condition.
func (in *Condition) DeepCopy() *Condition {
	if in == nil {
		return nil
	}
	out := new(Condition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ConfigMapReference) DeepCopyInto(out *ConfigMapReference) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConfigMapReference.
func (in *ConfigMapReference) DeepCopy() *ConfigMapReference {
	if in == nil {
		return nil
	}
	out := new(ConfigMapReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ContextEntry) DeepCopyInto(out *ContextEntry) {
	*out = *in
	if in.ConfigMap != nil {
		in, out := &in.ConfigMap, &out.ConfigMap
		*out = new(ConfigMapReference)
		**out = **in
	}
	if in.APICall != nil {
		in, out := &in.APICall, &out.APICall
		*out = new(APICall)
		**out = **in
	}
	if in.ImageRegistry != nil {
		in, out := &in.ImageRegistry, &out.ImageRegistry
		*out = new(ImageRegistry)
		**out = **in
	}
	if in.Variable != nil {
		in, out := &in.Variable, &out.Variable
		*out = new(Variable)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ContextEntry.
func (in *ContextEntry) DeepCopy() *ContextEntry {
	if in == nil {
		return nil
	}
	out := new(ContextEntry)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Deny) DeepCopyInto(out *Deny) {
	*out = *in
	if in.RawAnyAllConditions != nil {
		in, out := &in.RawAnyAllConditions, &out.RawAnyAllConditions
		*out = new(apiextensionsv1.JSON)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Deny.
func (in *Deny) DeepCopy() *Deny {
	if in == nil {
		return nil
	}
	out := new(Deny)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ForEachMutation) DeepCopyInto(out *ForEachMutation) {
	*out = *in
	if in.Context != nil {
		in, out := &in.Context, &out.Context
		*out = make([]ContextEntry, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.AnyAllConditions != nil {
		in, out := &in.AnyAllConditions, &out.AnyAllConditions
		*out = new(AnyAllConditions)
		(*in).DeepCopyInto(*out)
	}
	if in.RawPatchStrategicMerge != nil {
		in, out := &in.RawPatchStrategicMerge, &out.RawPatchStrategicMerge
		*out = new(apiextensionsv1.JSON)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ForEachMutation.
func (in *ForEachMutation) DeepCopy() *ForEachMutation {
	if in == nil {
		return nil
	}
	out := new(ForEachMutation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ForEachValidation) DeepCopyInto(out *ForEachValidation) {
	*out = *in
	if in.ElementScope != nil {
		in, out := &in.ElementScope, &out.ElementScope
		*out = new(bool)
		**out = **in
	}
	if in.Context != nil {
		in, out := &in.Context, &out.Context
		*out = make([]ContextEntry, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.AnyAllConditions != nil {
		in, out := &in.AnyAllConditions, &out.AnyAllConditions
		*out = new(AnyAllConditions)
		(*in).DeepCopyInto(*out)
	}
	if in.RawPattern != nil {
		in, out := &in.RawPattern, &out.RawPattern
		*out = new(apiextensionsv1.JSON)
		(*in).DeepCopyInto(*out)
	}
	if in.RawAnyPattern != nil {
		in, out := &in.RawAnyPattern, &out.RawAnyPattern
		*out = new(apiextensionsv1.JSON)
		(*in).DeepCopyInto(*out)
	}
	if in.Deny != nil {
		in, out := &in.Deny, &out.Deny
		*out = new(Deny)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ForEachValidation.
func (in *ForEachValidation) DeepCopy() *ForEachValidation {
	if in == nil {
		return nil
	}
	out := new(ForEachValidation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GenerateRequest) DeepCopyInto(out *GenerateRequest) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GenerateRequest.
func (in *GenerateRequest) DeepCopy() *GenerateRequest {
	if in == nil {
		return nil
	}
	out := new(GenerateRequest)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *GenerateRequest) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GenerateRequestContext) DeepCopyInto(out *GenerateRequestContext) {
	*out = *in
	in.UserRequestInfo.DeepCopyInto(&out.UserRequestInfo)
	out.AdmissionRequestInfo = in.AdmissionRequestInfo
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GenerateRequestContext.
func (in *GenerateRequestContext) DeepCopy() *GenerateRequestContext {
	if in == nil {
		return nil
	}
	out := new(GenerateRequestContext)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GenerateRequestList) DeepCopyInto(out *GenerateRequestList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]GenerateRequest, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GenerateRequestList.
func (in *GenerateRequestList) DeepCopy() *GenerateRequestList {
	if in == nil {
		return nil
	}
	out := new(GenerateRequestList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *GenerateRequestList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GenerateRequestSpec) DeepCopyInto(out *GenerateRequestSpec) {
	*out = *in
	out.Resource = in.Resource
	in.Context.DeepCopyInto(&out.Context)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GenerateRequestSpec.
func (in *GenerateRequestSpec) DeepCopy() *GenerateRequestSpec {
	if in == nil {
		return nil
	}
	out := new(GenerateRequestSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *GenerateRequestStatus) DeepCopyInto(out *GenerateRequestStatus) {
	*out = *in
	if in.GeneratedResources != nil {
		in, out := &in.GeneratedResources, &out.GeneratedResources
		*out = make([]ResourceSpec, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new GenerateRequestStatus.
func (in *GenerateRequestStatus) DeepCopy() *GenerateRequestStatus {
	if in == nil {
		return nil
	}
	out := new(GenerateRequestStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Generation) DeepCopyInto(out *Generation) {
	*out = *in
	out.ResourceSpec = in.ResourceSpec
	if in.RawData != nil {
		in, out := &in.RawData, &out.RawData
		*out = new(apiextensionsv1.JSON)
		(*in).DeepCopyInto(*out)
	}
	out.Clone = in.Clone
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Generation.
func (in *Generation) DeepCopy() *Generation {
	if in == nil {
		return nil
	}
	out := new(Generation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ImageRegistry) DeepCopyInto(out *ImageRegistry) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ImageRegistry.
func (in *ImageRegistry) DeepCopy() *ImageRegistry {
	if in == nil {
		return nil
	}
	out := new(ImageRegistry)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ImageVerification) DeepCopyInto(out *ImageVerification) {
	*out = *in
	if in.ImageReferences != nil {
		in, out := &in.ImageReferences, &out.ImageReferences
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.AdditionalExtensions != nil {
		in, out := &in.AdditionalExtensions, &out.AdditionalExtensions
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Attestors != nil {
		in, out := &in.Attestors, &out.Attestors
		*out = make([]*AttestorSet, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(AttestorSet)
				(*in).DeepCopyInto(*out)
			}
		}
	}
	if in.Attestations != nil {
		in, out := &in.Attestations, &out.Attestations
		*out = make([]*Attestation, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(Attestation)
				(*in).DeepCopyInto(*out)
			}
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.MutateDigest != nil {
		in, out := &in.MutateDigest, &out.MutateDigest
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ImageVerification.
func (in *ImageVerification) DeepCopy() *ImageVerification {
	if in == nil {
		return nil
	}
	out := new(ImageVerification)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *KeylessAttestor) DeepCopyInto(out *KeylessAttestor) {
	*out = *in
	if in.Rekor != nil {
		in, out := &in.Rekor, &out.Rekor
		*out = new(CTLog)
		**out = **in
	}
	if in.AdditionalExtensions != nil {
		in, out := &in.AdditionalExtensions, &out.AdditionalExtensions
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new KeylessAttestor.
func (in *KeylessAttestor) DeepCopy() *KeylessAttestor {
	if in == nil {
		return nil
	}
	out := new(KeylessAttestor)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MatchResources) DeepCopyInto(out *MatchResources) {
	*out = *in
	if in.Any != nil {
		in, out := &in.Any, &out.Any
		*out = make(ResourceFilters, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.All != nil {
		in, out := &in.All, &out.All
		*out = make(ResourceFilters, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.UserInfo.DeepCopyInto(&out.UserInfo)
	in.ResourceDescription.DeepCopyInto(&out.ResourceDescription)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MatchResources.
func (in *MatchResources) DeepCopy() *MatchResources {
	if in == nil {
		return nil
	}
	out := new(MatchResources)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Mutation) DeepCopyInto(out *Mutation) {
	*out = *in
	if in.RawPatchStrategicMerge != nil {
		in, out := &in.RawPatchStrategicMerge, &out.RawPatchStrategicMerge
		*out = new(apiextensionsv1.JSON)
		(*in).DeepCopyInto(*out)
	}
	if in.ForEachMutation != nil {
		in, out := &in.ForEachMutation, &out.ForEachMutation
		*out = make([]*ForEachMutation, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(ForEachMutation)
				(*in).DeepCopyInto(*out)
			}
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Mutation.
func (in *Mutation) DeepCopy() *Mutation {
	if in == nil {
		return nil
	}
	out := new(Mutation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Policy) DeepCopyInto(out *Policy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Policy.
func (in *Policy) DeepCopy() *Policy {
	if in == nil {
		return nil
	}
	out := new(Policy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Policy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PolicyList) DeepCopyInto(out *PolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Policy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PolicyList.
func (in *PolicyList) DeepCopy() *PolicyList {
	if in == nil {
		return nil
	}
	out := new(PolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PolicyStatus) DeepCopyInto(out *PolicyStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.Autogen.DeepCopyInto(&out.Autogen)
	if in.Rules != nil {
		in, out := &in.Rules, &out.Rules
		*out = make([]Rule, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
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
func (in *RequestInfo) DeepCopyInto(out *RequestInfo) {
	*out = *in
	if in.Roles != nil {
		in, out := &in.Roles, &out.Roles
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.ClusterRoles != nil {
		in, out := &in.ClusterRoles, &out.ClusterRoles
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	in.AdmissionUserInfo.DeepCopyInto(&out.AdmissionUserInfo)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RequestInfo.
func (in *RequestInfo) DeepCopy() *RequestInfo {
	if in == nil {
		return nil
	}
	out := new(RequestInfo)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceDescription) DeepCopyInto(out *ResourceDescription) {
	*out = *in
	if in.Kinds != nil {
		in, out := &in.Kinds, &out.Kinds
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Names != nil {
		in, out := &in.Names, &out.Names
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Namespaces != nil {
		in, out := &in.Namespaces, &out.Namespaces
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Selector != nil {
		in, out := &in.Selector, &out.Selector
		*out = new(metav1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.NamespaceSelector != nil {
		in, out := &in.NamespaceSelector, &out.NamespaceSelector
		*out = new(metav1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceDescription.
func (in *ResourceDescription) DeepCopy() *ResourceDescription {
	if in == nil {
		return nil
	}
	out := new(ResourceDescription)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceFilter) DeepCopyInto(out *ResourceFilter) {
	*out = *in
	in.UserInfo.DeepCopyInto(&out.UserInfo)
	in.ResourceDescription.DeepCopyInto(&out.ResourceDescription)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceFilter.
func (in *ResourceFilter) DeepCopy() *ResourceFilter {
	if in == nil {
		return nil
	}
	out := new(ResourceFilter)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in ResourceFilters) DeepCopyInto(out *ResourceFilters) {
	{
		in := &in
		*out = make(ResourceFilters, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceFilters.
func (in ResourceFilters) DeepCopy() ResourceFilters {
	if in == nil {
		return nil
	}
	out := new(ResourceFilters)
	in.DeepCopyInto(out)
	return *out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceSpec) DeepCopyInto(out *ResourceSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceSpec.
func (in *ResourceSpec) DeepCopy() *ResourceSpec {
	if in == nil {
		return nil
	}
	out := new(ResourceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Rule) DeepCopyInto(out *Rule) {
	*out = *in
	if in.Context != nil {
		in, out := &in.Context, &out.Context
		*out = make([]ContextEntry, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.MatchResources.DeepCopyInto(&out.MatchResources)
	in.ExcludeResources.DeepCopyInto(&out.ExcludeResources)
	if in.ImageExtractors != nil {
		in, out := &in.ImageExtractors, &out.ImageExtractors
		*out = make(kube.ImageExtractorConfigs, len(*in))
		for key, val := range *in {
			var outVal []kube.ImageExtractorConfig
			if val == nil {
				(*out)[key] = nil
			} else {
				in, out := &val, &outVal
				*out = make([]kube.ImageExtractorConfig, len(*in))
				copy(*out, *in)
			}
			(*out)[key] = outVal
		}
	}
	if in.RawAnyAllConditions != nil {
		in, out := &in.RawAnyAllConditions, &out.RawAnyAllConditions
		*out = new(apiextensionsv1.JSON)
		(*in).DeepCopyInto(*out)
	}
	in.Mutation.DeepCopyInto(&out.Mutation)
	in.Validation.DeepCopyInto(&out.Validation)
	in.Generation.DeepCopyInto(&out.Generation)
	if in.VerifyImages != nil {
		in, out := &in.VerifyImages, &out.VerifyImages
		*out = make([]*ImageVerification, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(ImageVerification)
				(*in).DeepCopyInto(*out)
			}
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Rule.
func (in *Rule) DeepCopy() *Rule {
	if in == nil {
		return nil
	}
	out := new(Rule)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Spec) DeepCopyInto(out *Spec) {
	*out = *in
	if in.Rules != nil {
		in, out := &in.Rules, &out.Rules
		*out = make([]Rule, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.FailurePolicy != nil {
		in, out := &in.FailurePolicy, &out.FailurePolicy
		*out = new(FailurePolicyType)
		**out = **in
	}
	if in.ValidationFailureActionOverrides != nil {
		in, out := &in.ValidationFailureActionOverrides, &out.ValidationFailureActionOverrides
		*out = make([]ValidationFailureActionOverride, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Background != nil {
		in, out := &in.Background, &out.Background
		*out = new(bool)
		**out = **in
	}
	if in.SchemaValidation != nil {
		in, out := &in.SchemaValidation, &out.SchemaValidation
		*out = new(bool)
		**out = **in
	}
	if in.WebhookTimeoutSeconds != nil {
		in, out := &in.WebhookTimeoutSeconds, &out.WebhookTimeoutSeconds
		*out = new(int32)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Spec.
func (in *Spec) DeepCopy() *Spec {
	if in == nil {
		return nil
	}
	out := new(Spec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *StaticKeyAttestor) DeepCopyInto(out *StaticKeyAttestor) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new StaticKeyAttestor.
func (in *StaticKeyAttestor) DeepCopy() *StaticKeyAttestor {
	if in == nil {
		return nil
	}
	out := new(StaticKeyAttestor)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *UserInfo) DeepCopyInto(out *UserInfo) {
	*out = *in
	if in.Roles != nil {
		in, out := &in.Roles, &out.Roles
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.ClusterRoles != nil {
		in, out := &in.ClusterRoles, &out.ClusterRoles
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Subjects != nil {
		in, out := &in.Subjects, &out.Subjects
		*out = make([]rbacv1.Subject, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new UserInfo.
func (in *UserInfo) DeepCopy() *UserInfo {
	if in == nil {
		return nil
	}
	out := new(UserInfo)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Validation) DeepCopyInto(out *Validation) {
	*out = *in
	if in.ForEachValidation != nil {
		in, out := &in.ForEachValidation, &out.ForEachValidation
		*out = make([]*ForEachValidation, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(ForEachValidation)
				(*in).DeepCopyInto(*out)
			}
		}
	}
	if in.RawPattern != nil {
		in, out := &in.RawPattern, &out.RawPattern
		*out = new(apiextensionsv1.JSON)
		(*in).DeepCopyInto(*out)
	}
	if in.RawAnyPattern != nil {
		in, out := &in.RawAnyPattern, &out.RawAnyPattern
		*out = new(apiextensionsv1.JSON)
		(*in).DeepCopyInto(*out)
	}
	if in.Deny != nil {
		in, out := &in.Deny, &out.Deny
		*out = new(Deny)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Validation.
func (in *Validation) DeepCopy() *Validation {
	if in == nil {
		return nil
	}
	out := new(Validation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ValidationFailureActionOverride) DeepCopyInto(out *ValidationFailureActionOverride) {
	*out = *in
	if in.Namespaces != nil {
		in, out := &in.Namespaces, &out.Namespaces
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ValidationFailureActionOverride.
func (in *ValidationFailureActionOverride) DeepCopy() *ValidationFailureActionOverride {
	if in == nil {
		return nil
	}
	out := new(ValidationFailureActionOverride)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Variable) DeepCopyInto(out *Variable) {
	*out = *in
	if in.Value != nil {
		in, out := &in.Value, &out.Value
		*out = new(apiextensionsv1.JSON)
		(*in).DeepCopyInto(*out)
	}
	if in.Default != nil {
		in, out := &in.Default, &out.Default
		*out = new(apiextensionsv1.JSON)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Variable.
func (in *Variable) DeepCopy() *Variable {
	if in == nil {
		return nil
	}
	out := new(Variable)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ViolatedRule) DeepCopyInto(out *ViolatedRule) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ViolatedRule.
func (in *ViolatedRule) DeepCopy() *ViolatedRule {
	if in == nil {
		return nil
	}
	out := new(ViolatedRule)
	in.DeepCopyInto(out)
	return out
}
