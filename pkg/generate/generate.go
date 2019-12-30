package generate

import (
	"fmt"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (c *Controller) processGR(gr kyverno.GenerateRequest) error {
	// 1 - Check if the resource exists
	resource, err := getResource(c.client, gr.Spec.Resource)
	if err != nil {
		// Dont update status
		glog.V(4).Info("resource does not exist or is yet to be created, requeuing: %v", err)
		return err
	}
	// 2 - Apply the generate policy on the resource
	err = c.applyGenerate(*resource, gr)
	// 3 - Update Status
	return updateStatus(c.statusControl, gr, err)
}

func (c *Controller) applyGenerate(resource unstructured.Unstructured, gr kyverno.GenerateRequest) error {
	// Get the list of rules to be applied
	// get policy
	policy, err := c.pLister.Get(gr.Spec.Policy)
	if err != nil {
		glog.V(4).Infof("policy %s not found: %v", gr.Spec.Policy, err)
	}
	// build context
	ctx := context.NewContext()
	resourceRaw, err := resource.MarshalJSON()
	if err != nil {
		glog.V(4).Infof("failed to marshal resource: %v", err)
		return err
	}

	ctx.AddResource(resourceRaw)
	ctx.AddUserInfo(gr.Spec.Context.UserRequestInfo)

	policyContext := engine.PolicyContext{
		NewResource:   resource,
		Context:       ctx,
		AdmissionInfo: gr.Spec.Context.UserRequestInfo,
	}
	// check if the policy still applies to the resource
	engineResponse := engine.GenerateNew(policyContext)
	if len(engineResponse.PolicyResponse.Rules) == 0 {
		glog.V(4).Infof("policy %s, dont not apply to resource %v", gr.Spec.Policy, gr.Spec.Resource)
		return fmt.Errorf("policy %s, dont not apply to resource %v", gr.Spec.Policy, gr.Spec.Resource)
	}

	// Apply the generate rule on resource

	return nil
}

func updateStatus(statusControl StatusControlInterface, gr kyverno.GenerateRequest, err error) error {
	if err != nil {
		return statusControl.Failed(gr, err.Error())
	}

	// Generate request successfully processed
	return statusControl.Success(gr)
}

func applyGeneratePolicy(policy kyverno.ClusterPolicy) {
	// Get the response as the actions to be performed on the resource
	// - DATA (rule.Generation.Data)
	// - - substitute values
}
