package client

import (
	kyvernov1alpha1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
)

//CreatePolicyViolation create a Policy Violation resource
func (c *Client) CreatePolicyViolation(pv kyvernov1alpha1.PolicyViolation) error {
	_, err := c.CreateResource("PolicyViolation", ",", pv, false)
	return err
}
