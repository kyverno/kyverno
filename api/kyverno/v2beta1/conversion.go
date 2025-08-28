package v2beta1

import (
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
)

// ConvertFromV2Alpha1 converts a v2alpha1.GlobalContextEntry to v2beta1.GlobalContextEntry
func (dst *GlobalContextEntry) ConvertFromV2Alpha1(src *kyvernov2alpha1.GlobalContextEntry) {
	// Copy metadata
	dst.TypeMeta = src.TypeMeta
	dst.TypeMeta.APIVersion = "kyverno.io/v2beta1"
	dst.ObjectMeta = src.ObjectMeta

	// Convert spec
	dst.Spec = GlobalContextEntrySpec{
		KubernetesResource: convertKubernetesResourceFromV2Alpha1(src.Spec.KubernetesResource),
		APICall:            convertExternalAPICallFromV2Alpha1(src.Spec.APICall),
		Projections:        convertProjectionsFromV2Alpha1(src.Spec.Projections),
	}

	// Convert status
	dst.Status = GlobalContextEntryStatus{
		Ready:           src.Status.Ready,
		Conditions:      src.Status.Conditions,
		LastRefreshTime: src.Status.LastRefreshTime,
	}
}

// ConvertToV2Alpha1 converts a v2beta1.GlobalContextEntry to v2alpha1.GlobalContextEntry
func (src *GlobalContextEntry) ConvertToV2Alpha1(dst *kyvernov2alpha1.GlobalContextEntry) {
	// Copy metadata
	dst.TypeMeta = src.TypeMeta
	dst.TypeMeta.APIVersion = "kyverno.io/v2alpha1"
	dst.ObjectMeta = src.ObjectMeta

	// Convert spec
	dst.Spec = kyvernov2alpha1.GlobalContextEntrySpec{
		KubernetesResource: convertKubernetesResourceToV2Alpha1(src.Spec.KubernetesResource),
		APICall:            convertExternalAPICallToV2Alpha1(src.Spec.APICall),
		Projections:        convertProjectionsToV2Alpha1(src.Spec.Projections),
	}

	// Convert status
	dst.Status = kyvernov2alpha1.GlobalContextEntryStatus{
		Ready:           src.Status.Ready,
		Conditions:      src.Status.Conditions,
		LastRefreshTime: src.Status.LastRefreshTime,
	}
}

func convertKubernetesResourceFromV2Alpha1(src *kyvernov2alpha1.KubernetesResource) *KubernetesResource {
	if src == nil {
		return nil
	}
	return &KubernetesResource{
		Group:     src.Group,
		Version:   src.Version,
		Resource:  src.Resource,
		Namespace: src.Namespace,
	}
}

func convertKubernetesResourceToV2Alpha1(src *KubernetesResource) *kyvernov2alpha1.KubernetesResource {
	if src == nil {
		return nil
	}
	return &kyvernov2alpha1.KubernetesResource{
		Group:     src.Group,
		Version:   src.Version,
		Resource:  src.Resource,
		Namespace: src.Namespace,
	}
}

func convertExternalAPICallFromV2Alpha1(src *kyvernov2alpha1.ExternalAPICall) *ExternalAPICall {
	if src == nil {
		return nil
	}
	return &ExternalAPICall{
		APICall:         src.APICall,
		RefreshInterval: src.RefreshInterval,
		RetryLimit:      src.RetryLimit,
	}
}

func convertExternalAPICallToV2Alpha1(src *ExternalAPICall) *kyvernov2alpha1.ExternalAPICall {
	if src == nil {
		return nil
	}
	return &kyvernov2alpha1.ExternalAPICall{
		APICall:         src.APICall,
		RefreshInterval: src.RefreshInterval,
		RetryLimit:      src.RetryLimit,
	}
}

func convertProjectionsFromV2Alpha1(src []kyvernov2alpha1.GlobalContextEntryProjection) []GlobalContextEntryProjection {
	if src == nil {
		return nil
	}
	var result []GlobalContextEntryProjection
	for _, p := range src {
		result = append(result, GlobalContextEntryProjection{
			Name:     p.Name,
			JMESPath: p.JMESPath,
		})
	}
	return result
}

func convertProjectionsToV2Alpha1(src []GlobalContextEntryProjection) []kyvernov2alpha1.GlobalContextEntryProjection {
	if src == nil {
		return nil
	}
	var result []kyvernov2alpha1.GlobalContextEntryProjection
	for _, p := range src {
		result = append(result, kyvernov2alpha1.GlobalContextEntryProjection{
			Name:     p.Name,
			JMESPath: p.JMESPath,
		})
	}
	return result
}
