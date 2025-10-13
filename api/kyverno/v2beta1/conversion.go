/*
Copyright 2022 The Kubernetes authors.

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

package v2beta1

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
)

// Convert_v2alpha1_GlobalContextEntry_To_v2beta1_GlobalContextEntry converts a v2alpha1 GlobalContextEntry to v2beta1
func Convert_v2alpha1_GlobalContextEntry_To_v2beta1_GlobalContextEntry(in *kyvernov2alpha1.GlobalContextEntry, out *GlobalContextEntry, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	return Convert_v2alpha1_GlobalContextEntrySpec_To_v2beta1_GlobalContextEntrySpec(&in.Spec, &out.Spec, s)
}

// Convert_v2beta1_GlobalContextEntry_To_v2alpha1_GlobalContextEntry converts a v2beta1 GlobalContextEntry to v2alpha1
func Convert_v2beta1_GlobalContextEntry_To_v2alpha1_GlobalContextEntry(in *GlobalContextEntry, out *kyvernov2alpha1.GlobalContextEntry, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	return Convert_v2beta1_GlobalContextEntrySpec_To_v2alpha1_GlobalContextEntrySpec(&in.Spec, &out.Spec, s)
}

// Convert_v2alpha1_GlobalContextEntrySpec_To_v2beta1_GlobalContextEntrySpec converts a v2alpha1 GlobalContextEntrySpec to v2beta1
func Convert_v2alpha1_GlobalContextEntrySpec_To_v2beta1_GlobalContextEntrySpec(in *kyvernov2alpha1.GlobalContextEntrySpec, out *GlobalContextEntrySpec, s conversion.Scope) error {
	if in.KubernetesResource != nil {
		out.KubernetesResource = &KubernetesResource{
			Group:     in.KubernetesResource.Group,
			Version:   in.KubernetesResource.Version,
			Resource:  in.KubernetesResource.Resource,
			Namespace: in.KubernetesResource.Namespace,
		}
	}

	if in.APICall != nil {
		out.APICall = &ExternalAPICall{
			APICall: kyvernov1.APICall{
				URLPath: in.APICall.URLPath,
				Method:  kyvernov1.Method(in.APICall.Method),
				Service: in.APICall.Service,
			},
			RefreshInterval: &in.APICall.RefreshInterval,
			RetryLimit:      in.APICall.RetryLimit,
		}

		// Convert Data fields from v2alpha1 RequestData to v1 RequestData (used in v2beta1)
		if len(in.APICall.Data) > 0 {
			out.APICall.Data = make([]kyvernov1.RequestData, len(in.APICall.Data))
			for i, data := range in.APICall.Data {
				out.APICall.Data[i] = kyvernov1.RequestData{
					Key:   data.Key,
					Value: &data.Value,
				}
			}
		}
	}

	// Convert projections
	if len(in.Projections) > 0 {
		out.Projections = make([]GlobalContextEntryProjection, len(in.Projections))
		for i, proj := range in.Projections {
			out.Projections[i] = GlobalContextEntryProjection{
				Name:     proj.Name,
				JMESPath: proj.JMESPath,
			}
		}
	}

	return nil
}

// Convert_v2beta1_GlobalContextEntrySpec_To_v2alpha1_GlobalContextEntrySpec converts a v2beta1 GlobalContextEntrySpec to v2alpha1
func Convert_v2beta1_GlobalContextEntrySpec_To_v2alpha1_GlobalContextEntrySpec(in *GlobalContextEntrySpec, out *kyvernov2alpha1.GlobalContextEntrySpec, s conversion.Scope) error {
	if in.KubernetesResource != nil {
		out.KubernetesResource = &kyvernov2alpha1.KubernetesResource{
			Group:     in.KubernetesResource.Group,
			Version:   in.KubernetesResource.Version,
			Resource:  in.KubernetesResource.Resource,
			Namespace: in.KubernetesResource.Namespace,
		}
	}

	if in.APICall != nil {
		out.APICall = &kyvernov2alpha1.ExternalAPICall{
			URLPath:    in.APICall.URLPath,
			Method:     string(in.APICall.Method),
			RetryLimit: in.APICall.RetryLimit,
			Service:    in.APICall.Service,
		}

		// Convert RefreshInterval (handle pointer difference)
		if in.APICall.RefreshInterval != nil {
			out.APICall.RefreshInterval = *in.APICall.RefreshInterval
		}

		// Convert Data fields from v1 RequestData (used in v2beta1) to v2alpha1 RequestData
		if len(in.APICall.Data) > 0 {
			out.APICall.Data = make([]kyvernov2alpha1.RequestData, len(in.APICall.Data))
			for i, data := range in.APICall.Data {
				out.APICall.Data[i] = kyvernov2alpha1.RequestData{
					Key: data.Key,
				}
				if data.Value != nil {
					out.APICall.Data[i].Value = *data.Value
				}
			}
		}
	}

	// Convert projections
	if len(in.Projections) > 0 {
		out.Projections = make([]kyvernov2alpha1.GlobalContextEntryProjection, len(in.Projections))
		for i, proj := range in.Projections {
			out.Projections[i] = kyvernov2alpha1.GlobalContextEntryProjection{
				Name:     proj.Name,
				JMESPath: proj.JMESPath,
			}
		}
	}

	return nil
}

// Convert_v2alpha1_GlobalContextEntryStatus_To_v2beta1_GlobalContextEntryStatus converts a v2alpha1 GlobalContextEntryStatus to v2beta1
func Convert_v2alpha1_GlobalContextEntryStatus_To_v2beta1_GlobalContextEntryStatus(in *kyvernov2alpha1.GlobalContextEntryStatus, out *GlobalContextEntryStatus, s conversion.Scope) error {
	out.Conditions = make([]metav1.Condition, len(in.Conditions))
	copy(out.Conditions, in.Conditions)

	out.LastRefreshTime = in.LastRefreshTime

	return nil
}

// Convert_v2beta1_GlobalContextEntryStatus_To_v2alpha1_GlobalContextEntryStatus converts a v2beta1 GlobalContextEntryStatus to v2alpha1
func Convert_v2beta1_GlobalContextEntryStatus_To_v2alpha1_GlobalContextEntryStatus(in *GlobalContextEntryStatus, out *kyvernov2alpha1.GlobalContextEntryStatus, s conversion.Scope) error {
	out.Conditions = make([]metav1.Condition, len(in.Conditions))
	copy(out.Conditions, in.Conditions)

	out.LastRefreshTime = in.LastRefreshTime

	return nil
}
