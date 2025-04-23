package variables

import (
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
)

func GetAttestors(att []v1alpha1.Attestor) map[string]string {
	m := make(map[string]string)
	for _, v := range att {
		m[v.Name] = v.Name
	}
	return m
}

func GetAttestations(att []v1alpha1.Attestation) map[string]string {
	m := make(map[string]string)
	for _, v := range att {
		m[v.Name] = v.Name
	}
	return m
}
