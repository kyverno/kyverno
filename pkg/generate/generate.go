package generate

import (
	"fmt"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/validate"
	"github.com/nirmata/kyverno/pkg/engine/variables"
	"github.com/nirmata/kyverno/pkg/policyviolation"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
	switch e := err.(type) {
	case *Violation:
		c.pvGenerator.Add(generatePV(gr, *resource, e))
	default:
		glog.V(4).Info(e)
	}
	// create events on policy and resource

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
		Policy:        *policy,
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
	return applyGeneratePolicy(c.client, policyContext, gr.Status.State)
}

func updateStatus(statusControl StatusControlInterface, gr kyverno.GenerateRequest, err error) error {
	if err != nil {
		return statusControl.Failed(gr, err.Error())
	}

	// Generate request successfully processed
	return statusControl.Success(gr)
}

func applyGeneratePolicy(client *dclient.Client, policyContext engine.PolicyContext, state kyverno.GenerateRequestState) error {
	// Get the response as the actions to be performed on the resource
	// - DATA (rule.Generation.Data)
	// - - substitute values
	policy := policyContext.Policy
	resource := policyContext.NewResource
	ctx := policyContext.Context

	// To manage existing resources, we compare the creation time for the default resiruce to be generated and policy creation time
	processExisting := func() bool {
		rcreationTime := resource.GetCreationTimestamp()
		pcreationTime := policy.GetCreationTimestamp()
		return rcreationTime.Before(&pcreationTime)
	}()

	for _, rule := range policy.Spec.Rules {
		if !rule.HasGenerate() {
			continue
		}
		if err := applyRule(client, rule, resource, ctx, state, processExisting); err != nil {
			return err
		}
	}

	return nil
}

func applyRule(client *dclient.Client, rule kyverno.Rule, resource unstructured.Unstructured, ctx context.EvalInterface, state kyverno.GenerateRequestState, processExisting bool) error {
	var rdata map[string]interface{}
	var err error
	// DATA
	if rule.Generation.Data != nil {
		if rdata, err = handleData(rule.Name, rule.Generation, client, resource, ctx, state); err != nil {
			switch e := err.(type) {
			case *ParseFailed, *NotFound, *ConfigNotFound:
				// handled errors
			case *Violation:
				// create policy violation
				return e
			default:
				// errors that cant be handled
				return e
			}
		}
		if rdata == nil {
			// existing resource contains the configuration
			return nil
		}
	}
	// CLONE
	if rule.Generation.Clone != (kyverno.CloneFrom{}) {
		if rdata, err = handleClone(rule.Generation, client, resource, ctx, state); err != nil {
			switch e := err.(type) {
			case *NotFound:
				// handled errors
				return e
			default:
				// errors that cant be handled
				return e
			}
		}
		if rdata == nil {
			// resource already exists
		}
	}
	if processExisting {
		// handle existing resources
		// policy was generated after the resource
		// we do not create new resource
		return err
	}
	// Create the generate resource
	newResource := &unstructured.Unstructured{}
	newResource.SetUnstructuredContent(rdata)
	newResource.SetName(rule.Generation.Name)
	newResource.SetNamespace(rule.Generation.Namespace)
	// Reset resource version
	newResource.SetResourceVersion("")
	_, err = client.CreateResource(rule.Generation.Kind, rule.Generation.Namespace, rule.Generation.Name, false)
	if err != nil {
		return err
	}
	// New Resource created succesfully
	return nil
}

func handleData(ruleName string, generateRule kyverno.Generation, client *dclient.Client, resource unstructured.Unstructured, ctx context.EvalInterface, state kyverno.GenerateRequestState) (map[string]interface{}, error) {
	newData := variables.SubstituteVariables(ctx, generateRule.Data)

	// check if resource exists
	obj, err := client.GetResource(generateRule.Kind, generateRule.Namespace, generateRule.Name)
	if errors.IsNotFound(err) {
		// Resource does not exist
		if state == kyverno.Pending {
			// Processing the request first time
			rdata, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&newData)
			if err != nil {
				return nil, NewParseFailed(newData, err)
			}
			return rdata, nil
		}
		// State : Failed,Completed
		// request has been processed before, so dont create the resource
		// report Violation to notify the error
		return nil, NewViolation(ruleName, NewNotFound(generateRule.Kind, generateRule.Namespace, generateRule.Name))
	}
	if err != nil {
		//something wrong while fetching resource
		return nil, err
	}
	// Resource exists; verfiy the content of the resource
	ok, err := checkResource(ctx, newData, obj)
	if err != nil {
		//something wrong with configuration
		return nil, err
	}
	if !ok {
		return nil, NewConfigNotFound(newData, generateRule.Kind, generateRule.Namespace, generateRule.Name)
	}
	// Existing resource does contain the required
	return nil, nil
}

func handleClone(generateRule kyverno.Generation, client *dclient.Client, resource unstructured.Unstructured, ctx context.EvalInterface, state kyverno.GenerateRequestState) (map[string]interface{}, error) {
	// check if resource exists
	_, err := client.GetResource(generateRule.Kind, generateRule.Namespace, generateRule.Name)
	if err == nil {
		// resource exists
		return nil, nil
	}
	if !errors.IsNotFound(err) {
		//something wrong while fetching resource
		return nil, err
	}

	// get reference clone resource
	obj, err := client.GetResource(generateRule.Kind, generateRule.Clone.Namespace, generateRule.Clone.Name)
	if errors.IsNotFound(err) {
		return nil, NewNotFound(generateRule.Kind, generateRule.Clone.Namespace, generateRule.Clone.Name)
	}
	if err != nil {
		//something wrong while fetching resource
		return nil, err
	}
	return obj.UnstructuredContent(), nil
}

func checkResource(ctx context.EvalInterface, newResourceSpec interface{}, resource *unstructured.Unstructured) (bool, error) {
	// check if the resource spec if a subset of the resource
	path, err := validate.ValidateResourceWithPattern(ctx, resource.Object, newResourceSpec)
	if err != nil {
		glog.V(4).Infof("config not a subset of resource. failed at path %s: %v", path, err)
		return false, err
	}
	return true, nil
}

func generatePV(gr kyverno.GenerateRequest, resource unstructured.Unstructured, err *Violation) policyviolation.Info {

	info := policyviolation.Info{
		Blocked:    false,
		PolicyName: gr.Spec.Policy,
		Resource:   resource,
		Rules: []kyverno.ViolatedRule{kyverno.ViolatedRule{
			Name:    err.rule,
			Type:    "Generation",
			Message: err.Error(),
		}},
	}
	return info
}
