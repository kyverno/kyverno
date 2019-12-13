package engine

import (
	"time"

	"fmt"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

//Generate apply generation rules on a resource
func Generate(policyContext PolicyContext) (response EngineResponse) {
	policy := policyContext.Policy
	ns := policyContext.NewResource
	client := policyContext.Client

	startTime := time.Now()
	// policy information
	func() {
		// set policy information
		response.PolicyResponse.Policy = policy.Name
		// resource details
		response.PolicyResponse.Resource.Name = ns.GetName()
		response.PolicyResponse.Resource.Kind = ns.GetKind()
		response.PolicyResponse.Resource.APIVersion = ns.GetAPIVersion()
	}()
	glog.V(4).Infof("started applying generation rules of policy %q (%v)", policy.Name, startTime)
	defer func() {
		response.PolicyResponse.ProcessingTime = time.Since(startTime)
		glog.V(4).Infof("finished applying generation rules policy %v (%v)", policy.Name, response.PolicyResponse.ProcessingTime)
		glog.V(4).Infof("Generation Rules appplied succesfully count %v for policy %q", response.PolicyResponse.RulesAppliedCount, policy.Name)
	}()
	incrementAppliedRuleCount := func() {
		// rules applied succesfully count
		response.PolicyResponse.RulesAppliedCount++
	}
	for _, rule := range policy.Spec.Rules {
		if !rule.HasGenerate() {
			continue
		}
		glog.V(4).Infof("applying policy %s generate rule %s on resource %s/%s/%s", policy.Name, rule.Name, ns.GetKind(), ns.GetNamespace(), ns.GetName())
		ruleResponse := applyRuleGenerator(client, ns, rule, policy.GetCreationTimestamp())
		response.PolicyResponse.Rules = append(response.PolicyResponse.Rules, ruleResponse)
		incrementAppliedRuleCount()
	}
	// set resource in reponse
	response.PatchedResource = ns
	return response
}

func applyRuleGenerator(client *client.Client, ns unstructured.Unstructured, rule kyverno.Rule, policyCreationTime metav1.Time) (response RuleResponse) {
	startTime := time.Now()
	glog.V(4).Infof("started applying generation rule %q (%v)", rule.Name, startTime)
	response.Name = rule.Name
	response.Type = Generation.String()
	defer func() {
		response.RuleStats.ProcessingTime = time.Since(startTime)
		glog.V(4).Infof("finished applying generation rule %q (%v)", response.Name, response.RuleStats.ProcessingTime)
	}()

	var err error
	resource := &unstructured.Unstructured{}
	var rdata map[string]interface{}
	// To manage existing resource , we compare the creation time for the default resource to be generate and policy creation time
	processExisting := func() bool {
		nsCreationTime := ns.GetCreationTimestamp()
		return nsCreationTime.Before(&policyCreationTime)
	}()
	if rule.Generation.Data != nil {
		glog.V(4).Info("generate rule: creates new resource")
		// 1> Check if resource exists
		obj, err := client.GetResource(rule.Generation.Kind, ns.GetName(), rule.Generation.Name)
		if err == nil {
			glog.V(4).Infof("generate rule: resource %s/%s/%s already present. checking if it contains the required configuration", rule.Generation.Kind, ns.GetName(), rule.Generation.Name)
			// 2> If already exsists, then verify the content is contained
			// found the resource
			// check if the rule is create, if yes, then verify if the specified configuration is present in the resource
			ok, err := checkResource(rule.Generation.Data, obj)
			if err != nil {
				glog.V(4).Infof("generate rule: unable to check if configuration %v, is present in resource '%s/%s' in namespace '%s'", rule.Generation.Data, rule.Generation.Kind, rule.Generation.Name, ns.GetName())
				response.Success = false
				response.Message = fmt.Sprintf("unable to check if configuration %v, is present in resource '%s/%s' in namespace '%s'", rule.Generation.Data, rule.Generation.Kind, rule.Generation.Name, ns.GetName())
				return response
			}
			if !ok {
				glog.V(4).Infof("generate rule: configuration %v not present in resource '%s/%s' in namespace '%s'", rule.Generation.Data, rule.Generation.Kind, rule.Generation.Name, ns.GetName())
				response.Success = false
				response.Message = fmt.Sprintf("configuration %v not present in resource '%s/%s' in namespace '%s'", rule.Generation.Data, rule.Generation.Kind, rule.Generation.Name, ns.GetName())
				return response
			}
			response.Success = true
			response.Message = fmt.Sprintf("required configuration %v is present in resource '%s/%s' in namespace '%s'", rule.Generation.Data, rule.Generation.Kind, rule.Generation.Name, ns.GetName())
			return response
		}
		rdata, err = runtime.DefaultUnstructuredConverter.ToUnstructured(&rule.Generation.Data)
		if err != nil {
			glog.Error(err)
			response.Success = false
			response.Message = fmt.Sprintf("failed to parse the specified resource spec %v: %v", rule.Generation.Data, err)
			return response
		}
	}
	if rule.Generation.Clone != (kyverno.CloneFrom{}) {
		glog.V(4).Info("generate rule: clone resource")
		// 1> Check if resource exists
		_, err := client.GetResource(rule.Generation.Kind, ns.GetName(), rule.Generation.Name)
		if err == nil {
			glog.V(4).Infof("generate rule: resource '%s/%s' already present in namespace '%s'", rule.Generation.Kind, rule.Generation.Name, ns.GetName())
			response.Success = true
			response.Message = fmt.Sprintf("resource '%s/%s' already present in namespace '%s'", rule.Generation.Kind, rule.Generation.Name, ns.GetName())
			return response
		}
		// 2> If clone already exists return
		resource, err = client.GetResource(rule.Generation.Kind, rule.Generation.Clone.Namespace, rule.Generation.Clone.Name)
		if err != nil {
			glog.V(4).Infof("generate rule: clone reference resource '%s/%s' not present in namespace '%s': %v", rule.Generation.Kind, rule.Generation.Clone.Name, rule.Generation.Clone.Namespace, err)
			response.Success = false
			response.Message = fmt.Sprintf("clone reference resource '%s/%s' not present in namespace '%s': %v", rule.Generation.Kind, rule.Generation.Clone.Name, rule.Generation.Clone.Namespace, err)
			return response
		}
		glog.V(4).Infof("generate rule: clone reference resource '%s/%s'  present in namespace '%s'", rule.Generation.Kind, rule.Generation.Clone.Name, rule.Generation.Clone.Namespace)
		rdata = resource.UnstructuredContent()
	}
	if processExisting {
		glog.V(4).Infof("resource '%s/%s' not found in existing namespace '%s'", rule.Generation.Kind, rule.Generation.Name, ns.GetName())
		response.Success = false
		response.Message = fmt.Sprintf("resource '%s/%s' not found in existing namespace '%s'", rule.Generation.Kind, rule.Generation.Name, ns.GetName())
		// for existing resources we generate an error which indirectly generates a policy violation
		return response
	}
	resource.SetUnstructuredContent(rdata)
	resource.SetName(rule.Generation.Name)
	resource.SetNamespace(ns.GetName())
	// Reset resource version
	resource.SetResourceVersion("")
	_, err = client.CreateResource(rule.Generation.Kind, ns.GetName(), resource, false)
	if err != nil {
		glog.V(4).Infof("generate rule: unable to create resource %s/%s/%s: %v", rule.Generation.Kind, resource.GetNamespace(), resource.GetName(), err)
		response.Success = false
		response.Message = fmt.Sprintf("unable to create resource %s/%s/%s: %v", rule.Generation.Kind, resource.GetNamespace(), resource.GetName(), err)
		return response
	}
	glog.V(4).Infof("generate rule: created resource %s/%s/%s", rule.Generation.Kind, resource.GetNamespace(), resource.GetName())
	response.Success = true
	response.Message = fmt.Sprintf("created resource %s/%s/%s", rule.Generation.Kind, resource.GetNamespace(), resource.GetName())
	return response
}

//checkResource checks if the config is present in th eresource
func checkResource(config interface{}, resource *unstructured.Unstructured) (bool, error) {

	// we are checking if config is a subset of resource with default pattern
	path, err := validateResourceWithPattern(resource.Object, config)
	if err != nil {
		glog.V(4).Infof("config not a subset of resource. failed at path %s: %v", path, err)
		return false, err
	}
	return true, nil
}
