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

// Code generated by controller-gen. DO NOT EDIT.

package v2beta1

import (
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

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
func (in *Deny) DeepCopyInto(out *Deny) {
	*out = *in
	if in.RawAnyAllConditions != nil {
		in, out := &in.RawAnyAllConditions, &out.RawAnyAllConditions
		*out = new(v1.AnyAllConditions)
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
		*out = new(v1.AnyAllConditions)
		(*in).DeepCopyInto(*out)
	}
	in.Mutation.DeepCopyInto(&out.Mutation)
	in.Validation.DeepCopyInto(&out.Validation)
	in.Generation.DeepCopyInto(&out.Generation)
	if in.VerifyImages != nil {
		in, out := &in.VerifyImages, &out.VerifyImages
		*out = make([]v1.ImageVerification, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
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
func (in *Validation) DeepCopyInto(out *Validation) {
	*out = *in
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
