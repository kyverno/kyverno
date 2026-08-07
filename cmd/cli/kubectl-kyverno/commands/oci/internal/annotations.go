package internal

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	PolicyConfigMediaType = "application/vnd.cncf.kyverno.config.v1+json"
	PolicyLayerMediaType  = "application/vnd.cncf.kyverno.policy.layer.v1+yaml"
	AnnotationKind        = "io.kyverno.image.kind"
	AnnotationName        = "io.kyverno.image.name"
	AnnotationApiVersion  = "io.kyverno.image.apiVersion"

	defaultApiVersion = "kyverno.io/v1"
)

func Annotations(policy kyvernov1.PolicyInterface) map[string]string {
	if policy == nil {
		return nil
	}
	kind := "ClusterPolicy"
	if policy.IsNamespaced() {
		kind = "Policy"
	}
	apiVersion := defaultApiVersion
	if obj, ok := policy.(runtime.Object); ok {
		if gvk := obj.GetObjectKind().GroupVersionKind(); gvk.Version != "" {
			apiVersion = gvk.GroupVersion().String()
		}
	}
	return map[string]string{
		AnnotationKind:       kind,
		AnnotationName:       policy.GetName(),
		AnnotationApiVersion: apiVersion,
	}
}
