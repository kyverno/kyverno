package generate

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/validate"
	"github.com/nirmata/kyverno/pkg/engine/variables"
	"github.com/nirmata/kyverno/pkg/policyviolation"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func (c *Controller) processGR(gr *kyverno.GenerateRequest) error {
	var err error
	var resource *unstructured.Unstructured
	var genResources []kyverno.ResourceSpec
	// 1 - Check if the resource exists
	resource, err = getResource(c.client, gr.Spec.Resource)
	if err != nil {
		// Dont update status
		glog.V(4).Infof("resource does not exist or is yet to be created, requeuing: %v", err)
		return err
	}

	// 2 - Apply the generate policy on the resource
	genResources, err = c.applyGenerate(*resource, *gr)
	switch e := err.(type) {
	case *Violation:
		// Generate event
		// - resource -> rule failed and created PV
		// - policy -> failed to apply of resource and created PV
		c.pvGenerator.Add(generatePV(*gr, *resource, e))
	default:
		// Generate event
		// - resource -> rule failed
		// - policy -> failed tp apply on resource
		glog.V(4).Info(e)
	}
	// 3 - Report Events
	reportEvents(err, c.eventGen, *gr, *resource)

	// 4 - Update Status
	return updateStatus(c.statusControl, *gr, err, genResources)
}

func (c *Controller) applyGenerate(resource unstructured.Unstructured, gr kyverno.GenerateRequest) ([]kyverno.ResourceSpec, error) {
	// Get the list of rules to be applied
	// get policy
	policy, err := c.pLister.Get(gr.Spec.Policy)
	if err != nil {
		glog.V(4).Infof("policy %s not found: %v", gr.Spec.Policy, err)
		return nil, nil
	}
	// build context
	ctx := context.NewContext()
	resourceRaw, err := resource.MarshalJSON()
	if err != nil {
		glog.V(4).Infof("failed to marshal resource: %v", err)
		return nil, err
	}
	err = ctx.AddResource(resourceRaw)
	if err != nil {
		glog.Infof("Failed to load resource in context: %v", err)
		return nil, err
	}
	err = ctx.AddUserInfo(gr.Spec.Context.UserRequestInfo)
	if err != nil {
		glog.Infof("Failed to load userInfo in context: %v", err)
		return nil, err
	}
	err = ctx.AddSA(gr.Spec.Context.UserRequestInfo.AdmissionUserInfo.Username)
	if err != nil {
		glog.Infof("Failed to load serviceAccount in context: %v", err)
		return nil, err
	}

	policyContext := engine.PolicyContext{
		NewResource:   resource,
		Policy:        *policy,
		Context:       ctx,
		AdmissionInfo: gr.Spec.Context.UserRequestInfo,
	}

	// check if the policy still applies to the resource
	engineResponse := engine.Generate(policyContext)
	if len(engineResponse.PolicyResponse.Rules) == 0 {
		glog.V(4).Infof("policy %s, dont not apply to resource %v", gr.Spec.Policy, gr.Spec.Resource)
		return nil, fmt.Errorf("policy %s, dont not apply to resource %v", gr.Spec.Policy, gr.Spec.Resource)
	}

	if pv := buildPathNotPresentPV(engineResponse); pv != nil {
		c.pvGenerator.Add(pv...)
		// variable substitiution fails in ruleInfo (match,exclude,condition)
		// the overall policy should not apply to resource
		return nil, fmt.Errorf("referenced path not present in generate policy %s", policy.Name)
	}

	// Apply the generate rule on resource
	return applyGeneratePolicy(c.client, policyContext)
}

func updateStatus(statusControl StatusControlInterface, gr kyverno.GenerateRequest, err error, genResources []kyverno.ResourceSpec) error {
	if err != nil {
		return statusControl.Failed(gr, err.Error(), genResources)
	}

	// Generate request successfully processed
	return statusControl.Success(gr, genResources)
}

func applyGeneratePolicy(client *dclient.Client, policyContext engine.PolicyContext) ([]kyverno.ResourceSpec, error) {
	// List of generatedResources
	var genResources []kyverno.ResourceSpec
	// Get the response as the actions to be performed on the resource
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
		genResource, err := applyRule(client, rule, resource, ctx, processExisting)
		if err != nil {
			return nil, err
		}
		genResources = append(genResources, genResource)
	}

	return genResources, nil
}

func applyRule(client *dclient.Client, rule kyverno.Rule, resource unstructured.Unstructured, ctx context.EvalInterface, processExisting bool) (kyverno.ResourceSpec, error) {
	var rdata map[string]interface{}
	var err error
	var mode ResourceMode
	var noGenResource kyverno.ResourceSpec

	if invalidPaths := variables.ValidateVariables(ctx, rule.Generation.ResourceSpec); len(invalidPaths) != 0 {
		return noGenResource, NewViolation(rule.Name, fmt.Errorf("path not present in generate resource spec: %s", invalidPaths))
	}

	// variable substitution
	// - name
	// - namespace
	// - clone.name
	// - clone.namespace
	gen := variableSubsitutionForAttributes(rule.Generation, ctx)
	// Resource to be generated
	newGenResource := kyverno.ResourceSpec{
		Kind:      gen.Kind,
		Namespace: gen.Namespace,
		Name:      gen.Name,
	}

	// DATA
	if gen.Data != nil {
		if rdata, mode, err = handleData(rule.Name, gen, client, resource, ctx); err != nil {
			glog.V(4).Info(err)
			switch e := err.(type) {
			case *ParseFailed, *NotFound, *ConfigNotFound:
				// handled errors
				return noGenResource, e
			case *Violation:
				// create policy violation
				return noGenResource, e
			default:
				// errors that cant be handled
				return noGenResource, e
			}
		}
		if rdata == nil {
			// existing resource contains the configuration
			return newGenResource, nil
		}
	}
	// CLONE
	if gen.Clone != (kyverno.CloneFrom{}) {
		if rdata, mode, err = handleClone(rule.Name, gen, client, resource, ctx); err != nil {
			glog.V(4).Info(err)
			switch e := err.(type) {
			case *NotFound:
				// handled errors
				return noGenResource, e
			default:
				// errors that cant be handled
				return noGenResource, e
			}
		}
		if rdata == nil {
			// resource already exists
			return newGenResource, nil
		}
	}
	if processExisting {
		// handle existing resources
		// policy was generated after the resource
		// we do not create new resource
		return noGenResource, err
	}

	// build the resource template
	newResource := &unstructured.Unstructured{}
	newResource.SetUnstructuredContent(rdata)
	newResource.SetName(gen.Name)
	newResource.SetNamespace(gen.Namespace)

	// manage labels
	// - app.kubernetes.io/managed-by: kyverno
	// - kyverno.io/generated-by: kind/namespace/name (trigger resource)
	manageLabels(newResource, resource)

	if mode == Create {
		// Reset resource version
		newResource.SetResourceVersion("")
		// Create the resource
		glog.V(4).Infof("Creating new resource %s/%s/%s", gen.Kind, gen.Namespace, gen.Name)
		_, err = client.CreateResource(gen.Kind, gen.Namespace, newResource, false)
		if err != nil {
			// Failed to create resource
			return noGenResource, err
		}
		glog.V(4).Infof("Created new resource %s/%s/%s", gen.Kind, gen.Namespace, gen.Name)

	} else if mode == Update {
		glog.V(4).Infof("Updating existing resource %s/%s/%s", gen.Kind, gen.Namespace, gen.Name)
		// Update the resource
		_, err := client.UpdateResource(gen.Kind, gen.Namespace, newResource, false)
		if err != nil {
			// Failed to update resource
			return noGenResource, err
		}
		glog.V(4).Infof("Updated existing resource %s/%s/%s", gen.Kind, gen.Namespace, gen.Name)
	}

	return newGenResource, nil
}

func variableSubsitutionForAttributes(gen kyverno.Generation, ctx context.EvalInterface) kyverno.Generation {
	// Name
	name := gen.Name
	namespace := gen.Namespace
	newNameVar := variables.SubstituteVariables(ctx, name)

	if newName, ok := newNameVar.(string); ok {
		gen.Name = newName
	}

	newNamespaceVar := variables.SubstituteVariables(ctx, namespace)
	if newNamespace, ok := newNamespaceVar.(string); ok {
		gen.Namespace = newNamespace
	}

	if gen.Clone != (kyverno.CloneFrom{}) {
		// Clone
		cloneName := gen.Clone.Name
		cloneNamespace := gen.Clone.Namespace

		newcloneNameVar := variables.SubstituteVariables(ctx, cloneName)
		if newcloneName, ok := newcloneNameVar.(string); ok {
			gen.Clone.Name = newcloneName
		}
		newcloneNamespaceVar := variables.SubstituteVariables(ctx, cloneNamespace)
		if newcloneNamespace, ok := newcloneNamespaceVar.(string); ok {
			gen.Clone.Namespace = newcloneNamespace
		}
	}
	return gen
}

// ResourceMode defines the mode for generated resource
type ResourceMode string

const (
	//Skip : failed to process rule, will not update the resource
	Skip ResourceMode = "SKIP"
	//Create : create a new resource
	Create = "CREATE"
	//Update : update/override the new resource
	Update = "UPDATE"
)

func copyInterface(original interface{}) (interface{}, error) {
	tempData, err := json.Marshal(original)
	if err != nil {
		return nil, err
	}
	fmt.Println(string(tempData))
	var temp interface{}
	err = json.Unmarshal(tempData, &temp)
	if err != nil {
		return nil, err
	}
	return temp, nil
}

// manage the creation/update of resource to be generated using the spec defined in the policy
func handleData(ruleName string, generateRule kyverno.Generation, client *dclient.Client, resource unstructured.Unstructured, ctx context.EvalInterface) (map[string]interface{}, ResourceMode, error) {
	//work on copy of the data
	// as the type of data stored in interface is not know,
	// we marshall the data and unmarshal it into a new resource to create a copy
	dataCopy, err := copyInterface(generateRule.Data)
	if err != nil {
		glog.V(4).Infof("failed to create a copy of the interface %v", generateRule.Data)
		return nil, Skip, err
	}
	// replace variables with the corresponding values
	newData := variables.SubstituteVariables(ctx, dataCopy)
	// if any variable defined in the data is not avaialbe in the context
	if invalidPaths := variables.ValidateVariables(ctx, newData); len(invalidPaths) != 0 {
		return nil, Skip, NewViolation(ruleName, fmt.Errorf("path not present in generate data: %s", invalidPaths))
	}

	// check if resource exists
	obj, err := client.GetResource(generateRule.Kind, generateRule.Namespace, generateRule.Name)
	if apierrors.IsNotFound(err) {
		// Resource does not exist
		// Processing the request first time
		rdata, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&newData)
		if err != nil {
			return nil, Skip, NewParseFailed(newData, err)
		}
		glog.V(4).Infof("Resource %s/%s/%s does not exists, will try to create", generateRule.Kind, generateRule.Namespace, generateRule.Name)
		return rdata, Create, nil
	}
	if err != nil {
		//something wrong while fetching resource
		return nil, Skip, err
	}
	// Resource exists; verfiy the content of the resource
	ok, err := checkResource(ctx, newData, obj)
	if err != nil {
		// error while evaluating if the existing resource contains the required information
		return nil, Skip, err
	}

	if !ok {
		// existing resource does not contain the configuration mentioned in spec, will try to update
		rdata, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&newData)
		if err != nil {
			return nil, Skip, NewParseFailed(newData, err)
		}

		glog.V(4).Infof("Resource %s/%s/%s exists but missing required configuration, will try to update", generateRule.Kind, generateRule.Namespace, generateRule.Name)
		return rdata, Update, nil
	}
	// Existing resource does contain the mentioned configuration in spec, skip processing the resource as it is already in expected state
	return nil, Skip, nil
}

// manage the creation/update based on the reference clone resource
func handleClone(ruleName string, generateRule kyverno.Generation, client *dclient.Client, resource unstructured.Unstructured, ctx context.EvalInterface) (map[string]interface{}, ResourceMode, error) {
	// if any variable defined in the data is not avaialbe in the context
	if invalidPaths := variables.ValidateVariables(ctx, generateRule.Clone); len(invalidPaths) != 0 {
		return nil, Skip, NewViolation(ruleName, fmt.Errorf("path not present in generate clone: %s", invalidPaths))
	}

	// check if resource to be generated exists
	_, err := client.GetResource(generateRule.Kind, generateRule.Namespace, generateRule.Name)
	if err == nil {
		// resource does exists, not need to process further as it is already in expected state
		return nil, Skip, nil
	}
	if !apierrors.IsNotFound(err) {
		//something wrong while fetching resource
		return nil, Skip, err
	}

	// get clone resource reference in the rule
	obj, err := client.GetResource(generateRule.Kind, generateRule.Clone.Namespace, generateRule.Clone.Name)
	if apierrors.IsNotFound(err) {
		// reference resource does not exist, cant generate the resources
		return nil, Skip, NewNotFound(generateRule.Kind, generateRule.Clone.Namespace, generateRule.Clone.Name)
	}
	if err != nil {
		//something wrong while fetching resource
		return nil, Skip, err
	}
	// create the resource based on the reference clone
	return obj.UnstructuredContent(), Create, nil
}

func checkResource(ctx context.EvalInterface, newResourceSpec interface{}, resource *unstructured.Unstructured) (bool, error) {
	// check if the resource spec if a subset of the resource
	path, err := validate.ValidateResourceWithPattern(ctx, resource.Object, newResourceSpec)
	if !reflect.DeepEqual(err, validate.ValidationError{}) {
		glog.V(4).Infof("config not a subset of resource. failed at path %s: %v", path, err)
		return false, errors.New(err.ErrorMsg)
	}
	return true, nil
}

func generatePV(gr kyverno.GenerateRequest, resource unstructured.Unstructured, err *Violation) policyviolation.Info {

	info := policyviolation.Info{
		PolicyName: gr.Spec.Policy,
		Resource:   resource,
		Rules: []kyverno.ViolatedRule{{
			Name:    err.rule,
			Type:    "Generation",
			Message: err.Error(),
		}},
	}
	return info
}
