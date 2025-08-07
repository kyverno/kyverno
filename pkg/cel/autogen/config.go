package autogen

import (
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
)

type Config struct {
	Target          policiesv1alpha1.Target
	ReplacementsRef string
}
