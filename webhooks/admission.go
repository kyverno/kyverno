package webhooks

import "k8s.io/api/admission/v1beta1"

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
