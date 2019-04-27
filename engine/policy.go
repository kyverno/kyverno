package engine

import (
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
)

type policyInterface interface {
	Apply(policy types.Policy, kind string, resource string, resourceRaw []byte) []byte, 
}
