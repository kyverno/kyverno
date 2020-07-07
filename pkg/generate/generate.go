package generate

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
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
	logger := c.log.WithValues("name", gr.Name, "policy", gr.Spec.Policy, "kind", gr.Spec.Resource.Kind, "namespace", gr.Spec.Resource.Namespace, "name", gr.Spec.Resource.Name)
	var err error
	var resource *unstructured.Unstructured
	var genResources []kyverno.ResourceSpec
	// 1 - Check if the resource exists
	resource, err = getResource(c.client, gr.Spec.Resource)
	if err != nil {
		// Dont update status
		logger.Error(err, "resource does not exist or is yet to be created, requeueing")
		return err
	}
	// 2 - Apply the generate policy on the resource
	genResources, err = c.applyGenerate(*resource, *gr)
	// 3 - Report Events
	reportEvents(logger, err, c.eventGen, *gr, *resource)
	// 4 - Update Status
	return updateStatus(c.statusControl, *gr, err, genResources)
}

func (c *Controller) applyGenerate(resource unstructured.Unstructured, gr kyverno.GenerateRequest) ([]kyverno.ResourceSpec, error) {
	logger := c.log.WithValues("name", gr.Name, "policy", gr.Spec.Policy, "kind", gr.Spec.Resource.Kind, "namespace", gr.Spec.Resource.Namespace, "name", gr.Spec.Resource.Name)
	// Get the list of rules to be applied
	// get policy
	tempPolicyVariable, err := c.pLister.Get(gr.Spec.Policy)
	var policy *v2beta1.Clusterpolicy
	if tempPolicyVariable.APIVersion == "v1" {
		backupPolicy = tempPolicyVariable
		delete(backupPolicy.Spec.Rules.Generation,"data")
		delete(backupPolicy.Spec.Rules.Generation,"clone")
		policy = backupPolicy
		for _,e := range tempPolicyVariable.Spec.Rules {
			var data []interface{}
			var clone []v2beta1.CloneFrom
            if e.Generation.Data != nil {
				data = append(data,e.Generation.Data)
				policy.Spec.Rules.Generation.Data = data
			}else {
 				clone = append(clone,e.Generation.Clone)
				policy.Spec.Rules.Generation.Clone = clone
			}
		}
	}else{
		policy = tempPolicyVariable
	}

	if err != nil {
		logger.Error(err, "policy not found")
		return nil, nil
	}
	// build context
	ctx := context.NewContext()
	resourceRaw, err := resource.MarshalJSON()
	if err != nil {
		logger.Error(err, "failed to marshal resource")
		return nil, err
	}
	err = ctx.AddResource(resourceRaw)
	if err != nil {
		logger.Error(err, "failed to load resource in context")
		return nil, err
	}
	err = ctx.AddUserInfo(gr.Spec.Context.UserRequestInfo)
	if err != nil {
		logger.Error(err, "failed to load SA in context")
		return nil, err
	}
	err = ctx.AddSA(gr.Spec.Context.UserRequestInfo.AdmissionUserInfo.Username)
	if err != nil {
		logger.Error(err, "failed to load UserInfo in context")
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
		logger.V(4).Info("policy does not apply to resource")
		return nil, fmt.Errorf("policy %s, dont not apply to resource %v", gr.Spec.Policy, gr.Spec.Resource)
	}

	// Apply the generate rule on resource
	return c.applyGeneratePolicy(logger, policyContext, gr)
}

func updateStatus(statusControl StatusControlInterface, gr kyverno.GenerateRequest, err error, genResources []kyverno.ResourceSpec) error {
	if err != nil {
		return statusControl.Failed(gr, err.Error(), genResources)
	}

	// Generate request successfully processed
	return statusControl.Success(gr, genResources)
}

func (c *Controller) applyGeneratePolicy(log logr.Logger, policyContext engine.PolicyContext, gr kyverno.GenerateRequest) ([]kyverno.ResourceSpec, error) {
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
		genResource, err := applyRule(log, c.client, rule, resource, ctx, processExisting)
		if err != nil {
			return nil, err
		}

		ruleNameToProcessingTime[rule.Name] = time.Since(startTime)
		for _,value := range genResource {
			genResources = append(genResources, value)
		}

	}

	if gr.Status.State == "" {
		c.policyStatusListener.Send(generateSyncStats{
			policyName:               policy.Name,
			ruleNameToProcessingTime: ruleNameToProcessingTime,
		})
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

func applyRule(log logr.Logger, client *dclient.Client, rule kyverno.Rule, resource unstructured.Unstructured, ctx context.EvalInterface, processExisting bool) ([]kyverno.ResourceSpec, error) {

	var newGenResources []kyverno.ResourceSpec
	var err error
	var mode ResourceMode
	var noGenResource kyverno.ResourceSpec
	// convert to unstructured Resource
	genUnst, err := getUnstrRule(rule.Generation.DeepCopy())
	if err != nil {
		newGenResources = append(newGenResources,noGenResource)
		return newGenResources,err
	}

	// Variable substitutions
	// format : {{<variable_name}}
	// - if there is variables that are not defined the context -> results in error and rule is not applied
	// - valid variables are replaced with the values
	object, err := variables.SubstituteVars(log, ctx, genUnst.Object)
	if err != nil {
		newGenResources = append(newGenResources,noGenResource)
		return newGenResources,err
	}
	genUnst.Object, _ = object.(map[string]interface{})

	genKind, _, err := unstructured.NestedString(genUnst.Object, "kind")
	if err != nil {
		newGenResources = append(newGenResources,noGenResource)
		return newGenResources,err
	}
	genName, _, err := unstructured.NestedString(genUnst.Object, "name")
	if err != nil {
		newGenResources = append(newGenResources,noGenResource)
		return newGenResources,err
	}
	genNamespace, _, err := unstructured.NestedString(genUnst.Object, "namespace")
	if err != nil {
		newGenResources = append(newGenResources,noGenResource)
		return newGenResources,err
	}


	genData, _, err := unstructured.NestedSlice(genUnst.Object, "data")
	if err != nil {
		newGenResources = append(newGenResources,noGenResource)
		return newGenResources,err
	}
	genCopy, _, err := unstructured.NestedSlice(genUnst.Object, "clone")
	if err != nil {
		newGenResources = append(newGenResources,noGenResource)
		return newGenResources,err
	}
	var rdata []generateResponse
	if genData != nil {
		rdata = manageData(log, genKind, genNamespace, genName, genData, client, resource)
	} else {
		rdata = manageClone(log, genKind, genNamespace, genName, genCopy, client, resource)
	}
		for _,singleData := range rdata {
			// Resource to be generated
			newGenResource := kyverno.ResourceSpec{
				Kind:      genKind,
				Namespace: genNamespace,
				Name:      genName,
			}
			if singleData.data == nil {
				// existing resource contains the configuration
				newGenResources = append(newGenResources,newGenResource)
				continue
			}
			if processExisting {
				// handle existing resources
				// policy was generated after the resource
				// we do not create new resource
				newGenResources = append(newGenResources,newGenResource)
				continue;

			}
			// build the resource template
			newResource := &unstructured.Unstructured{}
			newResource.SetUnstructuredContent(rdata)
			newResource.SetName(genName)
			newResource.SetNamespace(genNamespace)
			if newResource.GetKind() == "" {
				newResource.SetKind(genKind)
			}

			// manage labels
			// - app.kubernetes.io/managed-by: kyverno
			// - kyverno.io/generated-by: kind/namespace/name (trigger resource)
			manageLabels(newResource, resource)
			logger := log.WithValues("genKind", genKind, "genNamespace", genNamespace, "genName", genName)
			if singleData.action == Create {
				// Reset resource version
				newResource.SetResourceVersion("")
				// Create the resource
				logger.V(4).Info("creating new resource")
				_, err = client.CreateResource(genKind, genNamespace, newResource, false)
				if err != nil {
					// Failed to create resource
					newGenResources = append(newGenResources,noGenResource)
					continue;
				}
				logger.V(4).Info("created new resource")

			} else if mode == Update {
				if rule.Generation.Synchronize {
					logger.V(4).Info("updating existing resource")
					// Update the resource
					_, err := client.UpdateResource(genKind, genNamespace, newResource, false)
					if err != nil {
						// Failed to update resource
						newGenResources = append(newGenResources,noGenResource)
						continue;
					}
					logger.V(4).Info("updated new resource")

				} else {
					logger.V(4).Info("Synchronize resource is disabled")
				}
			}
		}

	return newGenResources, nil
}


func manageData(log logr.Logger, kind, namespace, name string, data []interface{}, client *dclient.Client, resource unstructured.Unstructured) ([]generateResponse) {
	// check if resource to be generated exists
	var respone []generateResponse;
	for _,dataResorce := range data {
		resource := dataResorce.(map[string]interface{})
		obj, err := client.GetResource(kind, namespace, name)
		if apierrors.IsNotFound(err) {
			log.Error(err, "resource does not exist, will try to create", "genKind", kind, "genNamespace", namespace, "genName", name)
			respone = append(respone,generateResponse{
				data : resource,
				action : Create,
				err : nil,
			})
		}
		if err != nil {
			//something wrong while fetching resource
			// client-errors
			respone = append(respone,generateResponse{
				data : nil,
				action : Skip,
				err : err,
			})
		}
		// Resource exists; verfiy the content of the resource
		err = checkResource(log, dataResorce, obj)
		if err == nil {
			// Existing resource does contain the mentioned configuration in spec, skip processing the resource as it is already in expected state
			respone = append(respone,generateResponse{
				data : nil,
				action : Skip,
				err : nil,
			})
		}
		log.Info("to be generated resoruce already exists, but is missing the specifeid configurations, will try to update", "genKind", kind, "genNamespace", namespace, "genName", name)
		respone = append(respone,generateResponse{
			data : resource,
			action : Update,
			err : nil,
		})
	}
    return respone
}

type generateResponse struct {
	data               map[string]interface{}
	action ResourceMode
	err error
}

func manageClone(log logr.Logger, kind, namespace, name string, clone []interface{}, client *dclient.Client, resource unstructured.Unstructured) ([]generateResponse) {
	var respone []generateResponse{};
	for _,cloneResorce := range clone {
		newRNs, _, err := unstructured. (cloneResorce, "namespace")
		if err != nil {
			respone = append(respone,generateResponse{
				data : nil,
				action : Skip,
				err : err,
			})
		}
		newRName, _, err := unstructured.NestedString(cloneResorce, "name")
		if err != nil {
			respone = append(respone,generateResponse{
				data : nil,
				action : Skip,
				err : err,
			})
		}
		// Short-circuit if the resource to be generated and the clone is the same
		if newRNs == namespace && newRName == name {
			// attempting to clone it self, this will fail -> short-ciruit it
			respone = append(respone,generateResponse{
				data : nil,
				action : Skip,
				err : nil,
			})
		}

		// check if the resource as reference in clone exists?
		obj, err := client.GetResource(kind, newRNs, newRName)
		if err != nil {
			respone = append(respone,generateResponse{
				data : nil,
				action : Skip,
				err : fmt.Errorf("reference clone resource %s/%s/%s not found. %v", kind, newRNs, newRName, err),
			})
		}

		// check if resource to be generated exists
		newResource, err := client.GetResource(kind, namespace, name)
		if err == nil {
			obj.SetUID(newResource.GetUID())
			obj.SetSelfLink(newResource.GetSelfLink())
			obj.SetCreationTimestamp(newResource.GetCreationTimestamp())
			obj.SetManagedFields(newResource.GetManagedFields())
			obj.SetResourceVersion(newResource.GetResourceVersion())
			if reflect.DeepEqual(obj, newResource) {
				respone = append(respone,generateResponse{
					data : nil,
					action : Skip,
					err : nil,
				})
			}
			respone = append(respone,generateResponse{
				data :  obj.UnstructuredContent(),
				action : Update,
				err : nil,
			})
		}

		//TODO: check this
		if !apierrors.IsNotFound(err) {
			log.Error(err, "reference/clone resource is not found", "genKind", kind, "genNamespace", namespace, "genName", name)
			//something wrong while fetching resource
			respone = append(respone,generateResponse{
				data : nil,
				action : Skip,
				err : err,
			})
		}

		// create the resource based on the reference clone
		respone = append(respone,generateResponse{
			data : obj.UnstructuredContent(),
			action : Create,
			err : nil,
		})
	}
	return respone
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

func checkResource(log logr.Logger, newResourceSpec interface{}, resource *unstructured.Unstructured) error {
	// check if the resource spec if a subset of the resource
	if path, err := validate.ValidateResourceWithPattern(log, resource.Object, newResourceSpec); err != nil {
		log.Error(err, "Failed to match the resource ", "path", path)
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
