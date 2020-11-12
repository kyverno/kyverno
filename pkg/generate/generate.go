package generate

import (
	contextdefault "context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/config"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/validate"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (c *Controller) processGR(gr *kyverno.GenerateRequest) error {
	logger := c.log.WithValues("name", gr.Name, "policy", gr.Spec.Policy, "kind", gr.Spec.Resource.Kind, "apiVersion", gr.Spec.Resource.APIVersion, "namespace", gr.Spec.Resource.Namespace, "name", gr.Spec.Resource.Name)
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
	events := failedEvents(err, *gr, *resource)
	c.eventGen.Add(events...)

	// 4 - Update Status
	return updateStatus(c.statusControl, *gr, err, genResources)
}

func (c *Controller) applyGenerate(resource unstructured.Unstructured, gr kyverno.GenerateRequest) ([]kyverno.ResourceSpec, error) {
	logger := c.log.WithValues("name", gr.Name, "policy", gr.Spec.Policy, "kind", gr.Spec.Resource.Kind, "apiVersion", gr.Spec.Resource.APIVersion, "namespace", gr.Spec.Resource.Namespace, "name", gr.Spec.Resource.Name)
	// Get the list of rules to be applied
	// get policy
	// build context
	ctx := context.NewContext()

	policyObj, err := c.pLister.Get(gr.Spec.Policy)
	if err != nil {
		if apierrors.IsNotFound(err) {
			for _, e := range gr.Status.GeneratedResources {
				resp, err := c.client.GetResource(e.APIVersion, e.Kind, e.Namespace, e.Name)
				if err != nil {
					logger.Error(err, "failed to find generated resource", "name", e.Name)
					continue
				}

				labels := resp.GetLabels()
				if labels["policy.kyverno.io/synchronize"] == "enable" {
					if err := c.client.DeleteResource(resp.GetAPIVersion(), resp.GetKind(), resp.GetNamespace(), resp.GetName(), false); err != nil {
						logger.Error(err, "Generated resource is not deleted", "Resource", e.Name)
					}
				}
			}

			return nil, nil
		}

		logger.Error(err, "error in fetching policy")
		return nil, err
	}

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
		NewResource:      resource,
		Policy:           *policyObj,
		Context:          ctx,
		AdmissionInfo:    gr.Spec.Context.UserRequestInfo,
		ExcludeGroupRole: c.Config.GetExcludeGroupRole(),
		ResourceCache:    c.resCache,
		JSONContext:      ctx,
	}

	// check if the policy still applies to the resource
	engineResponse := engine.Generate(policyContext)
	if len(engineResponse.PolicyResponse.Rules) == 0 {
		logger.V(4).Info("policy does not apply to resource")
		return nil, fmt.Errorf("policy %s, dont not apply to resource %v", gr.Spec.Policy, gr.Spec.Resource)
	}

	// Removing GR if rule is failed. Used when the generate condition failed but gr exist
	for _, r := range engineResponse.PolicyResponse.Rules {
		if !r.Success {
			grList, err := c.kyvernoClient.KyvernoV1().GenerateRequests(config.KubePolicyNamespace).List(contextdefault.TODO(), metav1.ListOptions{})
			if err != nil {
				logger.Error(err, "failed to list generate requests")
				continue
			}

			for _, v := range grList.Items {
				if engineResponse.PolicyResponse.Policy == v.Spec.Policy && engineResponse.PolicyResponse.Resource.Name == v.Spec.Resource.Name && engineResponse.PolicyResponse.Resource.Kind == v.Spec.Resource.Kind && engineResponse.PolicyResponse.Resource.Namespace == v.Spec.Resource.Namespace {
					err := c.kyvernoClient.KyvernoV1().GenerateRequests(config.KubePolicyNamespace).Delete(contextdefault.TODO(), v.GetName(), metav1.DeleteOptions{})
					if err != nil {
						logger.Error(err, " failed to delete generate request")
					}
				}
			}
		}
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

	resCache := policyContext.ResourceCache
	jsonContext := policyContext.JSONContext
	// To manage existing resources, we compare the creation time for the default resource to be generated and policy creation time

	ruleNameToProcessingTime := make(map[string]time.Duration)
	for _, rule := range policy.Spec.Rules {
		if !rule.HasGenerate() {
			continue
		}

		startTime := time.Now()
		processExisting := false

		if len(rule.MatchResources.Kinds) > 0 {
			if len(rule.MatchResources.Annotations) == 0 && rule.MatchResources.Selector == nil {
				processExisting = func() bool {
					rcreationTime := resource.GetCreationTimestamp()
					pcreationTime := policy.GetCreationTimestamp()
					return rcreationTime.Before(&pcreationTime)
				}()
			}
		}

		// add configmap json data to context
		if err := engine.AddResourceToContext(log, rule.Context, resCache, jsonContext); err != nil {
			log.Info("cannot add configmaps to context", "reason", err.Error())
			return nil, err
		}

		genResource, err := applyRule(log, c.client, rule, resource, ctx, policy.Name, gr, processExisting)
		if err != nil {
			log.Error(err, "failed to apply generate rule", "policy", policy.Name,
				"rule", rule.Name, "resource", resource.GetName())
			return nil, err
		}

		ruleNameToProcessingTime[rule.Name] = time.Since(startTime)
		genResources = append(genResources, genResource)
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

func applyRule(log logr.Logger, client *dclient.Client, rule kyverno.Rule, resource unstructured.Unstructured, ctx context.EvalInterface, policy string, gr kyverno.GenerateRequest, processExisting bool) (kyverno.ResourceSpec, error) {
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
	object, err := variables.SubstituteVars(log, ctx, genUnst.Object)
	if err != nil {
		return noGenResource, err
	}
	genUnst.Object, _ = object.(map[string]interface{})

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

	genAPIVersion, _, err := unstructured.NestedString(genUnst.Object, "apiVersion")
	if err != nil {
		return noGenResource, err
	}
	// Resource to be generated
	newGenResource := kyverno.ResourceSpec{
		APIVersion: genAPIVersion,
		Kind:       genKind,
		Namespace:  genNamespace,
		Name:       genName,
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
		rdata, mode, err = manageData(log, genAPIVersion, genKind, genNamespace, genName, genData, client, resource)
	} else {
		rdata, mode, err = manageClone(log, genAPIVersion, genKind, genNamespace, genName, genCopy, client, resource)
	}

	logger := log.WithValues("genKind", genKind, "genAPIVersion", genAPIVersion, "genNamespace", genNamespace, "genName", genName)

	if err != nil {
		return noGenResource, err
	}

	if rdata == nil {
		// existing resource contains the configuration
		return newGenResource, nil
	}
	if processExisting {
		return noGenResource, nil
	}

	// build the resource template
	newResource := &unstructured.Unstructured{}
	newResource.SetUnstructuredContent(rdata)
	newResource.SetName(genName)
	newResource.SetNamespace(genNamespace)
	if newResource.GetKind() == "" {
		newResource.SetKind(genKind)
	}

	newResource.SetAPIVersion(genAPIVersion)
	// manage labels
	// - app.kubernetes.io/managed-by: kyverno
	// - kyverno.io/generated-by: kind/namespace/name (trigger resource)
	manageLabels(newResource, resource)
	// Add Synchronize label
	label := newResource.GetLabels()
	label["policy.kyverno.io/policy-name"] = policy
	label["policy.kyverno.io/gr-name"] = gr.Name
	newResource.SetLabels(label)
	if mode == Create {
		if rule.Generation.Synchronize {
			label["policy.kyverno.io/synchronize"] = "enable"
		} else {
			label["policy.kyverno.io/synchronize"] = "disable"
		}
		// Reset resource version
		newResource.SetResourceVersion("")
		// Create the resource
		logger.V(4).Info("creating new resource")
		_, err = client.CreateResource(genAPIVersion, genKind, genNamespace, newResource, false)
		if err != nil {
			logger.Error(err, "failed to create resource", "resource", newResource.GetName())
			// Failed to create resource
			return noGenResource, err
		}
		logger.V(4).Info("created new resource")

	} else if mode == Update {
		var isUpdate bool
		label := newResource.GetLabels()
		isUpdate = false
		if rule.Generation.Synchronize {
			if label["policy.kyverno.io/synchronize"] == "enable" {
				isUpdate = true
			}
		} else {
			if label["policy.kyverno.io/synchronize"] == "enable" {
				isUpdate = true
			}
		}
		if rule.Generation.Synchronize {
			label["policy.kyverno.io/synchronize"] = "enable"
		} else {
			label["policy.kyverno.io/synchronize"] = "disable"
		}
		if isUpdate {
			logger.V(4).Info("updating existing resource")
			newResource.SetLabels(label)
			_, err := client.UpdateResource(genAPIVersion, genKind, genNamespace, newResource, false)
			if err != nil {
				logger.Error(err, "updating existing resource")
				return noGenResource, err
			}
			logger.V(4).Info("updated new resource")
		} else {
			resource := &unstructured.Unstructured{}
			resource.SetUnstructuredContent(rdata)
			resource.SetLabels(label)
			_, err := client.UpdateResource(genAPIVersion, genKind, genNamespace, resource, false)
			if err != nil {
				logger.Error(err, "updating existing resource")
				return noGenResource, err
			}
			logger.V(4).Info("updated new resource")
		}

		logger.V(4).Info("Synchronize resource is disabled")
	}
	return newGenResource, nil
}

func manageData(log logr.Logger, apiVersion, kind, namespace, name string, data map[string]interface{}, client *dclient.Client, resource unstructured.Unstructured) (map[string]interface{}, ResourceMode, error) {
	// check if resource to be generated exists
	obj, err := client.GetResource(apiVersion, kind, namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(err, "resource does not exist, will try to create", "genKind", kind, "genAPIVersion", apiVersion, "genNamespace", namespace, "genName", name)
			return data, Create, nil
		}
		//something wrong while fetching resource
		// client-errors
		return nil, Skip, err
	}
	updateObj := &unstructured.Unstructured{}
	updateObj.SetUnstructuredContent(data)
	updateObj.SetResourceVersion(obj.GetResourceVersion())
	return updateObj.UnstructuredContent(), Update, nil
}

func manageClone(log logr.Logger, apiVersion, kind, namespace, name string, clone map[string]interface{}, client *dclient.Client, resource unstructured.Unstructured) (map[string]interface{}, ResourceMode, error) {
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

	// check if the resource as reference in clone exists?
	obj, err := client.GetResource(apiVersion, kind, newRNs, newRName)
	if err != nil {
		return nil, Skip, fmt.Errorf("reference clone resource %s/%s/%s/%s not found. %v", apiVersion, kind, newRNs, newRName, err)
	}

	// check if resource to be generated exists
	newResource, err := client.GetResource(apiVersion, kind, namespace, name)
	if err == nil {
		obj.SetUID(newResource.GetUID())
		obj.SetSelfLink(newResource.GetSelfLink())
		obj.SetCreationTimestamp(newResource.GetCreationTimestamp())
		obj.SetManagedFields(newResource.GetManagedFields())
		obj.SetResourceVersion(newResource.GetResourceVersion())
		if reflect.DeepEqual(obj, newResource) {
			return nil, Skip, nil
		}
		return obj.UnstructuredContent(), Update, nil
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
