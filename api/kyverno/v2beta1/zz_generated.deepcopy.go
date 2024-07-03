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

package v2beta1

import (
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

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
	return
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
func (in *CleanupPolicy) DeepCopyInto(out *CleanupPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CleanupPolicy.
func (in *CleanupPolicy) DeepCopy() *CleanupPolicy {
	if in == nil {
		return nil
	}
	out := new(CleanupPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CleanupPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CleanupPolicyList) DeepCopyInto(out *CleanupPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]CleanupPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CleanupPolicyList.
func (in *CleanupPolicyList) DeepCopy() *CleanupPolicyList {
	if in == nil {
		return nil
	}
	out := new(CleanupPolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CleanupPolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CleanupPolicySpec) DeepCopyInto(out *CleanupPolicySpec) {
	*out = *in
	if in.Context != nil {
		in, out := &in.Context, &out.Context
		*out = make([]v1.ContextEntry, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.MatchResources.DeepCopyInto(&out.MatchResources)
	if in.ExcludeResources != nil {
		in, out := &in.ExcludeResources, &out.ExcludeResources
		*out = new(MatchResources)
		(*in).DeepCopyInto(*out)
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = new(AnyAllConditions)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CleanupPolicySpec.
func (in *CleanupPolicySpec) DeepCopy() *CleanupPolicySpec {
	if in == nil {
		return nil
	}
	out := new(CleanupPolicySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CleanupPolicyStatus) DeepCopyInto(out *CleanupPolicyStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.LastExecutionTime.DeepCopyInto(&out.LastExecutionTime)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CleanupPolicyStatus.
func (in *CleanupPolicyStatus) DeepCopy() *CleanupPolicyStatus {
	if in == nil {
		return nil
	}
	out := new(CleanupPolicyStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterCleanupPolicy) DeepCopyInto(out *ClusterCleanupPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterCleanupPolicy.
func (in *ClusterCleanupPolicy) DeepCopy() *ClusterCleanupPolicy {
	if in == nil {
		return nil
	}
	out := new(ClusterCleanupPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterCleanupPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterCleanupPolicyList) DeepCopyInto(out *ClusterCleanupPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterCleanupPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterCleanupPolicyList.
func (in *ClusterCleanupPolicyList) DeepCopy() *ClusterCleanupPolicyList {
	if in == nil {
		return nil
	}
	out := new(ClusterCleanupPolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterCleanupPolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterPolicy) DeepCopyInto(out *ClusterPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
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
	return
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
	return
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
func (in *Deny) DeepCopyInto(out *Deny) {
	*out = *in
	if in.RawAnyAllConditions != nil {
		in, out := &in.RawAnyAllConditions, &out.RawAnyAllConditions
		*out = new(AnyAllConditions)
		(*in).DeepCopyInto(*out)
	}
	return
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
func (in *Exception) DeepCopyInto(out *Exception) {
	*out = *in
	if in.RuleNames != nil {
		in, out := &in.RuleNames, &out.RuleNames
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Exception.
func (in *Exception) DeepCopy() *Exception {
	if in == nil {
		return nil
	}
	out := new(Exception)
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
	if in.SkipImageReferences != nil {
		in, out := &in.SkipImageReferences, &out.SkipImageReferences
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Attestors != nil {
		in, out := &in.Attestors, &out.Attestors
		*out = make([]v1.AttestorSet, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Attestations != nil {
		in, out := &in.Attestations, &out.Attestations
		*out = make([]v1.Attestation, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ImageRegistryCredentials != nil {
		in, out := &in.ImageRegistryCredentials, &out.ImageRegistryCredentials
		*out = new(v1.ImageRegistryCredentials)
		(*in).DeepCopyInto(*out)
	}
	return
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
func (in *MatchResources) DeepCopyInto(out *MatchResources) {
	*out = *in
	if in.Any != nil {
		in, out := &in.Any, &out.Any
		*out = make(v1.ResourceFilters, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.All != nil {
		in, out := &in.All, &out.All
		*out = make(v1.ResourceFilters, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
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
func (in *Policy) DeepCopyInto(out *Policy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
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
func (in *PolicyException) DeepCopyInto(out *PolicyException) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PolicyException.
func (in *PolicyException) DeepCopy() *PolicyException {
	if in == nil {
		return nil
	}
	out := new(PolicyException)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PolicyException) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PolicyExceptionList) DeepCopyInto(out *PolicyExceptionList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PolicyException, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PolicyExceptionList.
func (in *PolicyExceptionList) DeepCopy() *PolicyExceptionList {
	if in == nil {
		return nil
	}
	out := new(PolicyExceptionList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PolicyExceptionList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PolicyExceptionSpec) DeepCopyInto(out *PolicyExceptionSpec) {
	*out = *in
	if in.Background != nil {
		in, out := &in.Background, &out.Background
		*out = new(bool)
		**out = **in
	}
	in.Match.DeepCopyInto(&out.Match)
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = new(AnyAllConditions)
		(*in).DeepCopyInto(*out)
	}
	if in.Exceptions != nil {
		in, out := &in.Exceptions, &out.Exceptions
		*out = make([]Exception, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.PodSecurity != nil {
		in, out := &in.PodSecurity, &out.PodSecurity
		*out = make([]v1.PodSecurityStandard, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PolicyExceptionSpec.
func (in *PolicyExceptionSpec) DeepCopy() *PolicyExceptionSpec {
	if in == nil {
		return nil
	}
	out := new(PolicyExceptionSpec)
	in.DeepCopyInto(out)
	return out
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
	return
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
	if in.Operations != nil {
		in, out := &in.Operations, &out.Operations
		*out = make([]v1.AdmissionOperation, len(*in))
		copy(*out, *in)
	}
	return
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
	return
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
		return
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
func (in *Rule) DeepCopyInto(out *Rule) {
	*out = *in
	if in.Context != nil {
		in, out := &in.Context, &out.Context
		*out = make([]v1.ContextEntry, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.MatchResources.DeepCopyInto(&out.MatchResources)
	in.ExcludeResources.DeepCopyInto(&out.ExcludeResources)
	if in.ImageExtractors != nil {
		in, out := &in.ImageExtractors, &out.ImageExtractors
		*out = make(v1.ImageExtractorConfigs, len(*in))
		for key, val := range *in {
			var outVal []v1.ImageExtractorConfig
			if val == nil {
				(*out)[key] = nil
			} else {
				in, out := &val, &outVal
				*out = make([]v1.ImageExtractorConfig, len(*in))
				copy(*out, *in)
			}
			(*out)[key] = outVal
		}
	}
	if in.RawAnyAllConditions != nil {
		in, out := &in.RawAnyAllConditions, &out.RawAnyAllConditions
		*out = new(AnyAllConditions)
		(*in).DeepCopyInto(*out)
	}
	if in.CELPreconditions != nil {
		in, out := &in.CELPreconditions, &out.CELPreconditions
		*out = make([]admissionregistrationv1.MatchCondition, len(*in))
		copy(*out, *in)
	}
	in.Mutation.DeepCopyInto(&out.Mutation)
	in.Validation.DeepCopyInto(&out.Validation)
	in.Generation.DeepCopyInto(&out.Generation)
	if in.VerifyImages != nil {
		in, out := &in.VerifyImages, &out.VerifyImages
		*out = make([]ImageVerification, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
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
	if in.ApplyRules != nil {
		in, out := &in.ApplyRules, &out.ApplyRules
		*out = new(v1.ApplyRulesType)
		**out = **in
	}
	if in.FailurePolicy != nil {
		in, out := &in.FailurePolicy, &out.FailurePolicy
		*out = new(v1.FailurePolicyType)
		**out = **in
	}
	if in.ValidationFailureActionOverrides != nil {
		in, out := &in.ValidationFailureActionOverrides, &out.ValidationFailureActionOverrides
		*out = make([]v1.ValidationFailureActionOverride, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Admission != nil {
		in, out := &in.Admission, &out.Admission
		*out = new(bool)
		**out = **in
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
	if in.GenerateExistingOnPolicyUpdate != nil {
		in, out := &in.GenerateExistingOnPolicyUpdate, &out.GenerateExistingOnPolicyUpdate
		*out = new(bool)
		**out = **in
	}
	if in.WebhookConfiguration != nil {
		in, out := &in.WebhookConfiguration, &out.WebhookConfiguration
		*out = new(v1.WebhookConfiguration)
		(*in).DeepCopyInto(*out)
	}
	return
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
func (in *Validation) DeepCopyInto(out *Validation) {
	*out = *in
	if in.ValidationFailureAction != nil {
		in, out := &in.ValidationFailureAction, &out.ValidationFailureAction
		*out = new(v1.ValidationFailureAction)
		**out = **in
	}
	if in.ValidationFailureActionOverrides != nil {
		in, out := &in.ValidationFailureActionOverrides, &out.ValidationFailureActionOverrides
		*out = make([]v1.ValidationFailureActionOverride, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Manifests != nil {
		in, out := &in.Manifests, &out.Manifests
		*out = new(v1.Manifests)
		(*in).DeepCopyInto(*out)
	}
	if in.ForEachValidation != nil {
		in, out := &in.ForEachValidation, &out.ForEachValidation
		*out = make([]v1.ForEachValidation, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
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
	if in.PodSecurity != nil {
		in, out := &in.PodSecurity, &out.PodSecurity
		*out = new(v1.PodSecurity)
		(*in).DeepCopyInto(*out)
	}
	if in.CEL != nil {
		in, out := &in.CEL, &out.CEL
		*out = new(v1.CEL)
		(*in).DeepCopyInto(*out)
	}
	return
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
