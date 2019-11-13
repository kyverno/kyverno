package client

import (
	kyvernov "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
)

//CreatePolicyViolation create a Policy Violation resource
func (c *Client) CreatePolicyViolation(pv kyvernov.ClusterPolicyViolation) error {
	_, err := c.CreateResource("PolicyViolation", ",", pv, false)
	return err
}
