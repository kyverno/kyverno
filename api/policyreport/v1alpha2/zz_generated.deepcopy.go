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

package v1alpha2

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterPolicyReport) DeepCopyInto(out *ClusterPolicyReport) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Scope != nil {
		in, out := &in.Scope, &out.Scope
		*out = new(v1.ObjectReference)
		**out = **in
	}
	if in.ScopeSelector != nil {
		in, out := &in.ScopeSelector, &out.ScopeSelector
		*out = new(metav1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	out.Summary = in.Summary
	if in.Results != nil {
		in, out := &in.Results, &out.Results
		*out = make([]*PolicyReportResult, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(PolicyReportResult)
				(*in).DeepCopyInto(*out)
			}
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterPolicyReport.
func (in *ClusterPolicyReport) DeepCopy() *ClusterPolicyReport {
	if in == nil {
		return nil
	}
	out := new(ClusterPolicyReport)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterPolicyReport) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ClusterPolicyReportList) DeepCopyInto(out *ClusterPolicyReportList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ClusterPolicyReport, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ClusterPolicyReportList.
func (in *ClusterPolicyReportList) DeepCopy() *ClusterPolicyReportList {
	if in == nil {
		return nil
	}
	out := new(ClusterPolicyReportList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ClusterPolicyReportList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PolicyReport) DeepCopyInto(out *PolicyReport) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Scope != nil {
		in, out := &in.Scope, &out.Scope
		*out = new(v1.ObjectReference)
		**out = **in
	}
	if in.ScopeSelector != nil {
		in, out := &in.ScopeSelector, &out.ScopeSelector
		*out = new(metav1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	out.Summary = in.Summary
	if in.Results != nil {
		in, out := &in.Results, &out.Results
		*out = make([]*PolicyReportResult, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(PolicyReportResult)
				(*in).DeepCopyInto(*out)
			}
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PolicyReport.
func (in *PolicyReport) DeepCopy() *PolicyReport {
	if in == nil {
		return nil
	}
	out := new(PolicyReport)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PolicyReport) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PolicyReportList) DeepCopyInto(out *PolicyReportList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PolicyReport, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PolicyReportList.
func (in *PolicyReportList) DeepCopy() *PolicyReportList {
	if in == nil {
		return nil
	}
	out := new(PolicyReportList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PolicyReportList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PolicyReportResult) DeepCopyInto(out *PolicyReportResult) {
	*out = *in
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = make([]*v1.ObjectReference, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(v1.ObjectReference)
				**out = **in
			}
		}
	}
	if in.ResourceSelector != nil {
		in, out := &in.ResourceSelector, &out.ResourceSelector
		*out = new(metav1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.Properties != nil {
		in, out := &in.Properties, &out.Properties
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	out.Timestamp = in.Timestamp
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PolicyReportResult.
func (in *PolicyReportResult) DeepCopy() *PolicyReportResult {
	if in == nil {
		return nil
	}
	out := new(PolicyReportResult)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PolicyReportSummary) DeepCopyInto(out *PolicyReportSummary) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PolicyReportSummary.
func (in *PolicyReportSummary) DeepCopy() *PolicyReportSummary {
	if in == nil {
		return nil
	}
	out := new(PolicyReportSummary)
	in.DeepCopyInto(out)
	return out
}
