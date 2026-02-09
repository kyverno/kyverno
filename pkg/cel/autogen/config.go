package autogen

import (
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
)

type Config struct {
	Target          policiesv1beta1.Target
	ReplacementsRef string
}
