package utils

import policytype "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"

type ViolationInfo struct {
	Policy string
	policytype.Violation
}
