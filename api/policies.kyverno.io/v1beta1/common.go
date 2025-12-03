package v1beta1

import "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"

type (
	EvaluationMode          = v1alpha1.EvaluationMode
	EvaluationConfiguration = v1alpha1.EvaluationConfiguration
	AdmissionConfiguration  = v1alpha1.AdmissionConfiguration
	BackgroundConfiguration = v1alpha1.BackgroundConfiguration
)

const (
	EvaluationModeKubernetes EvaluationMode = "Kubernetes"
	EvaluationModeJSON       EvaluationMode = "JSON"
)
