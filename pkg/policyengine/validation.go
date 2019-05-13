package policyengine

import types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"

func (p *policyEngine) ProcessValidation(policy types.Policy, rawResource []byte) {}
