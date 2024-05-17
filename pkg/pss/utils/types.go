package utils

import (
	"k8s.io/pod-security-admission/policy"
)

type RestrictedField struct {
	Path          string
	AllowedValues []interface{}
}

type PSSCheckResult struct {
	ID               string
	CheckResult      policy.CheckResult
	RestrictedFields []RestrictedField
	Images           []string
}
