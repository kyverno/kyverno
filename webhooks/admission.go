package webhooks

import (
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	"k8s.io/api/admission/v1beta1"
)

var supportedKinds = [...]string{
	"ConfigMap",
	"CronJob",
	"DaemonSet",
	"Deployment",
	"Endpoint",
	"HorizontalPodAutoscaler",
	"Ingress",
	"Job",
	"LimitRange",
	"Namespace",
	"NetworkPolicy",
	"PersistentVolumeClaim",
	"PodDisruptionBudget",
	"PodTemplate",
	"ResourceQuota",
	"Secret",
	"Service",
	"StatefulSet",
}

func kindIsSupported(kind string) bool {
	for _, k := range supportedKinds {
		if k == kind {
			return true
		}
	}
	return false
}

func AdmissionIsRequired(request *v1beta1.AdmissionRequest) bool {
	// Here you can make additional hardcoded checks
	return kindIsSupported(request.Kind.Kind)
}

func IsRuleApplicableToRequest(rule types.PolicyRule, request *v1beta1.AdmissionRequest) bool {
	return IsRuleResourceFitsRequest(rule.Resource, request)
}

func IsRuleResourceFitsRequest(resource types.PolicyResource, request *v1beta1.AdmissionRequest) bool {
	if resource.Kind != request.Kind.Kind {
		return false
	}
	// TODO: resource.Name must be equal to request.Object.Raw -> /metadata/name

	return true
}
