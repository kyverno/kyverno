package internal

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

const (
	PolicyConfigMediaType = "application/vnd.cncf.kyverno.config.v1+json"
	PolicyLayerMediaType  = "application/vnd.cncf.kyverno.policy.layer.v1+yaml"
	AnnotationKind        = "io.kyverno.image.kind"
	AnnotationName        = "io.kyverno.image.name"
	AnnotationApiVersion  = "io.kyverno.image.apiVersion"
)

func apiVersion(policy kyvernov1.PolicyInterface) string {
	switch p := policy.(type) {
	case *kyvernov1.Policy:
		return p.APIVersion
	case *kyvernov1.ClusterPolicy:
		return p.APIVersion
	default:
		return ""
	}
}

func Annotations(policy kyvernov1.PolicyInterface) map[string]string {
	if policy == nil {
		return nil
	}
	kind := "ClusterPolicy"
	if policy.IsNamespaced() {
		kind = "Policy"
	}
	return map[string]string{
		AnnotationKind:       kind,
		AnnotationName:       policy.GetName(),
		AnnotationApiVersion: apiVersion(policy),
	}
}
