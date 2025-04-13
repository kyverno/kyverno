package report

import (
	"github.com/kyverno/kyverno/api/kyverno"
)

const (
	SourceKyverno                   = kyverno.ValueKyvernoApp
	SourceValidatingAdmissionPolicy = "ValidatingAdmissionPolicy"
	SourceValidatingPolicy          = "KyvernoValidatingPolicy"
	SourceImageValidatingPolicy     = "KyvernoImageValidatingPolicy"
)
