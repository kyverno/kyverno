package generate

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/validate"
	"github.com/nirmata/kyverno/pkg/engine/variables"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

	// Apply the generate rule on resource
	return c.applyGeneratePolicy(policyContext, gr)
}

func updateStatus(statusControl StatusControlInterface, gr kyverno.GenerateRequest, err error, genResources []kyverno.ResourceSpec) error {
	if err != nil {
		return statusControl.Failed(gr, err.Error(), genResources)
	}

	// Generate request successfully processed
	return statusControl.Success(gr, genResources)
}

func (c *Controller) applyGeneratePolicy(policyContext engine.PolicyContext, gr kyverno.GenerateRequest) ([]kyverno.ResourceSpec, error) {
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

	ruleNameToProcessingTime := make(map[string]time.Duration)
	for _, rule := range policy.Spec.Rules {
		if !rule.HasGenerate() {
			continue
		}

		startTime := time.Now()
		genResource, err := applyRule(c.client, rule, resource, ctx, processExisting)
		if err != nil {
			return nil, err
		}

		ruleNameToProcessingTime[rule.Name] = time.Since(startTime)
		genResources = append(genResources, genResource)
	}

	if gr.Status.State == "" {
		c.policyStatus.Listener <- generateSyncStats{
			policyName:               policy.Name,
			ruleNameToProcessingTime: ruleNameToProcessingTime,
		}
	}

	return genResources, nil
}

type generateSyncStats struct {
	policyName               string
	ruleNameToProcessingTime map[string]time.Duration
}

func (vc generateSyncStats) PolicyName() string {
	return vc.policyName
}

func (vc generateSyncStats) UpdateStatus(status kyverno.PolicyStatus) kyverno.PolicyStatus {

	for i := range status.Rules {
		if executionTime, exist := vc.ruleNameToProcessingTime[status.Rules[i].Name]; exist {
			status.ResourcesGeneratedCount += 1
			status.Rules[i].ResourcesGeneratedCount += 1
			averageOver := int64(status.Rules[i].AppliedCount + status.Rules[i].FailedCount)
			status.Rules[i].ExecutionTime = updateGenerateExecutionTime(
				executionTime,
				status.Rules[i].ExecutionTime,
				averageOver,
			).String()
		}
	}

	return status
}

func updateGenerateExecutionTime(newTime time.Duration, oldAverageTimeString string, averageOver int64) time.Duration {
	if averageOver == 0 {
		return newTime
	}
	oldAverageExecutionTime, _ := time.ParseDuration(oldAverageTimeString)
	numerator := (oldAverageExecutionTime.Nanoseconds() * averageOver) + newTime.Nanoseconds()
	denominator := averageOver
	newAverageTimeInNanoSeconds := numerator / denominator
	return time.Duration(newAverageTimeInNanoSeconds) * time.Nanosecond
}

func applyRule(client *dclient.Client, rule kyverno.Rule, resource unstructured.Unstructured, ctx context.EvalInterface, processExisting bool) (kyverno.ResourceSpec, error) {
	var rdata map[string]interface{}
	var err error
	var mode ResourceMode
	var noGenResource kyverno.ResourceSpec
	// convert to unstructured Resource
	genUnst, err := getUnstrRule(rule.Generation.DeepCopy())
	if err != nil {
		return noGenResource, err
	}

	// Variable substitutions
	// format : {{<variable_name}}
	// - if there is variables that are not defined the context -> results in error and rule is not applied
	// - valid variables are replaced with the values
	if _, err := variables.SubstituteVars(ctx, genUnst.Object); err != nil {
		return noGenResource, err
	}
	genKind, _, err := unstructured.NestedString(genUnst.Object, "kind")
	if err != nil {
		return noGenResource, err
	}
	genName, _, err := unstructured.NestedString(genUnst.Object, "name")
	if err != nil {
		return noGenResource, err
	}
	genNamespace, _, err := unstructured.NestedString(genUnst.Object, "namespace")
	if err != nil {
		return noGenResource, err
	}

	// Resource to be generated
	newGenResource := kyverno.ResourceSpec{
		Kind:      genKind,
		Namespace: genNamespace,
		Name:      genName,
	}
	genData, _, err := unstructured.NestedMap(genUnst.Object, "data")
	if err != nil {
		return noGenResource, err
	}
	genCopy, _, err := unstructured.NestedMap(genUnst.Object, "clone")
	if err != nil {
		return noGenResource, err
	}

	if genData != nil {
		rdata, mode, err = manageData(genKind, genNamespace, genName, genData, client, resource)
	} else {
		rdata, mode, err = manageClone(genKind, genNamespace, genName, genCopy, client, resource)
	}
	if err != nil {
		return noGenResource, err
	}

	if rdata == nil {
		// existing resource contains the configuration
		return newGenResource, nil
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
	newResource.SetName(genName)
	newResource.SetNamespace(genNamespace)

	// manage labels
	// - app.kubernetes.io/managed-by: kyverno
	// - kyverno.io/generated-by: kind/namespace/name (trigger resource)
	manageLabels(newResource, resource)

	if mode == Create {
		// Reset resource version
		newResource.SetResourceVersion("")
		// Create the resource
		glog.V(4).Infof("Creating new resource %s/%s/%s", genKind, genNamespace, genName)
		_, err = client.CreateResource(genKind, genNamespace, newResource, false)
		if err != nil {
			// Failed to create resource
			return noGenResource, err
		}
		glog.V(4).Infof("Created new resource %s/%s/%s", genKind, genNamespace, genName)

	} else if mode == Update {
		glog.V(4).Infof("Updating existing resource %s/%s/%s", genKind, genNamespace, genName)
		// Update the resource
		_, err := client.UpdateResource(genKind, genNamespace, newResource, false)
		if err != nil {
			// Failed to update resource
			return noGenResource, err
		}
		glog.V(4).Infof("Updated existing resource %s/%s/%s", genKind, genNamespace, genName)
	}

	return newGenResource, nil
}

func manageData(kind, namespace, name string, data map[string]interface{}, client *dclient.Client, resource unstructured.Unstructured) (map[string]interface{}, ResourceMode, error) {
	// check if resource to be generated exists
	obj, err := client.GetResource(kind, namespace, name)
	if apierrors.IsNotFound(err) {
		glog.V(4).Infof("Resource %s/%s/%s does not exists, will try to create", kind, namespace, name)
		return data, Create, nil
	}
	if err != nil {
		//something wrong while fetching resource
		// client-errors
		return nil, Skip, err
	}
	// Resource exists; verfiy the content of the resource
	err = checkResource(data, obj)
	if err == nil {
		// Existing resource does contain the mentioned configuration in spec, skip processing the resource as it is already in expected state
		return nil, Skip, nil
	}

	glog.V(4).Infof("Resource %s/%s/%s exists but missing required configuration, will try to update", kind, namespace, name)
	return data, Update, nil

}

func manageClone(kind, namespace, name string, clone map[string]interface{}, client *dclient.Client, resource unstructured.Unstructured) (map[string]interface{}, ResourceMode, error) {
	// check if resource to be generated exists
	_, err := client.GetResource(kind, namespace, name)
	if err == nil {
		// resource does exists, not need to process further as it is already in expected state
		return nil, Skip, nil
	}
	//TODO: check this
	if !apierrors.IsNotFound(err) {
		//something wrong while fetching resource
		return nil, Skip, err
	}

	newRNs, _, err := unstructured.NestedString(clone, "namespace")
	if err != nil {
		return nil, Skip, err
	}
	newRName, _, err := unstructured.NestedString(clone, "name")
	if err != nil {
		return nil, Skip, err
	}
	// Short-circuit if the resource to be generated and the clone is the same
	if newRNs == namespace && newRName == name {
		// attempting to clone it self, this will fail -> short-ciruit it
		return nil, Skip, nil
	}

	glog.V(4).Infof("check if resource %s/%s/%s exists", kind, newRNs, newRName)
	// check if the resource as reference in clone exists?
	obj, err := client.GetResource(kind, newRNs, newRName)
	if err != nil {
		return nil, Skip, fmt.Errorf("reference clone resource %s/%s/%s not found. %v", kind, newRNs, newRName, err)
	}
	// create the resource based on the reference clone
	return obj.UnstructuredContent(), Create, nil

}

// ResourceMode defines the mode for generated resource
type ResourceMode string

const (
	//Skip : failed to process rule, will not update the resource
	Skip ResourceMode = "SKIP"
	//Create : create a new resource
	Create = "CREATE"
	//Update : update/overwrite the new resource
	Update = "UPDATE"
)

func checkResource(newResourceSpec interface{}, resource *unstructured.Unstructured) error {
	// check if the resource spec if a subset of the resource
	if path, err := validate.ValidateResourceWithPattern(resource.Object, newResourceSpec); err != nil {
		glog.V(4).Infof("Failed to match the resource at path %s: err %v", path, err)
		return err
	}
	return nil
}

func getUnstrRule(rule *kyverno.Generation) (*unstructured.Unstructured, error) {
	ruleData, err := json.Marshal(rule)
	if err != nil {
		return nil, err
	}
	return ConvertToUnstructured(ruleData)
}

//ConvertToUnstructured converts the resource to unstructured format
func ConvertToUnstructured(data []byte) (*unstructured.Unstructured, error) {
	resource := &unstructured.Unstructured{}
	err := resource.UnmarshalJSON(data)
	if err != nil {
		return nil, err
	}
	return resource, nil
}
