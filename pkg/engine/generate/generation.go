package generate

import (
	"time"

	"fmt"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/engine/context"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"github.com/nirmata/kyverno/pkg/engine/validate"
	"github.com/nirmata/kyverno/pkg/engine/variables"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

//ApplyRuleGenerator apply generate rules
func ApplyRuleGenerator(ctx context.EvalInterface, client *client.Client, ns unstructured.Unstructured, rule kyverno.Rule, policyCreationTime metav1.Time) (resp response.RuleResponse) {
	startTime := time.Now()
	glog.V(4).Infof("started applying generation rule %q (%v)", rule.Name, startTime)
	resp.Name = rule.Name
	resp.Type = "Generation"
	defer func() {
		resp.RuleStats.ProcessingTime = time.Since(startTime)
		glog.V(4).Infof("finished applying generation rule %q (%v)", resp.Name, resp.RuleStats.ProcessingTime)
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
		// perform variable substituion in generate resource pattern
		newData := variables.SubstituteVariables(ctx, rule.Generation.Data)
		glog.V(4).Info("generate rule: creates new resource")
		// 1> Check if resource exists
		obj, err := client.GetResource(rule.Generation.Kind, ns.GetName(), rule.Generation.Name)
		if err == nil {
			glog.V(4).Infof("generate rule: resource %s/%s/%s already present. checking if it contains the required configuration", rule.Generation.Kind, ns.GetName(), rule.Generation.Name)
			// 2> If already exsists, then verify the content is contained
			// found the resource
			// check if the rule is create, if yes, then verify if the specified configuration is present in the resource
			ok, err := checkResource(ctx, newData, obj)
			if err != nil {
				glog.V(4).Infof("generate rule: unable to check if configuration %v, is present in resource '%s/%s' in namespace '%s'", rule.Generation.Data, rule.Generation.Kind, rule.Generation.Name, ns.GetName())
				resp.Success = false
				resp.Message = fmt.Sprintf("unable to check if configuration %v, is present in resource '%s/%s' in namespace '%s'", rule.Generation.Data, rule.Generation.Kind, rule.Generation.Name, ns.GetName())
				return resp
			}
			if !ok {
				glog.V(4).Infof("generate rule: configuration %v not present in resource '%s/%s' in namespace '%s'", rule.Generation.Data, rule.Generation.Kind, rule.Generation.Name, ns.GetName())
				resp.Success = false
				resp.Message = fmt.Sprintf("configuration %v not present in resource '%s/%s' in namespace '%s'", rule.Generation.Data, rule.Generation.Kind, rule.Generation.Name, ns.GetName())
				return resp
			}
			resp.Success = true
			resp.Message = fmt.Sprintf("required configuration %v is present in resource '%s/%s' in namespace '%s'", rule.Generation.Data, rule.Generation.Kind, rule.Generation.Name, ns.GetName())
			return resp
		}
		rdata, err = runtime.DefaultUnstructuredConverter.ToUnstructured(&newData)
		if err != nil {
			glog.Error(err)
			resp.Success = false
			resp.Message = fmt.Sprintf("failed to parse the specified resource spec %v: %v", newData, err)
			return resp
		}
	}
	if rule.Generation.Clone != (kyverno.CloneFrom{}) {
		glog.V(4).Info("generate rule: clone resource")
		// 1> Check if resource exists
		_, err := client.GetResource(rule.Generation.Kind, ns.GetName(), rule.Generation.Name)
		if err == nil {
			glog.V(4).Infof("generate rule: resource '%s/%s' already present in namespace '%s'", rule.Generation.Kind, rule.Generation.Name, ns.GetName())
			resp.Success = true
			resp.Message = fmt.Sprintf("resource '%s/%s' already present in namespace '%s'", rule.Generation.Kind, rule.Generation.Name, ns.GetName())
			return resp
		}
		// 2> If clone already exists return
		resource, err = client.GetResource(rule.Generation.Kind, rule.Generation.Clone.Namespace, rule.Generation.Clone.Name)
		if err != nil {
			glog.V(4).Infof("generate rule: clone reference resource '%s/%s' not present in namespace '%s': %v", rule.Generation.Kind, rule.Generation.Clone.Name, rule.Generation.Clone.Namespace, err)
			resp.Success = false
			resp.Message = fmt.Sprintf("clone reference resource '%s/%s' not present in namespace '%s': %v", rule.Generation.Kind, rule.Generation.Clone.Name, rule.Generation.Clone.Namespace, err)
			return resp
		}
		glog.V(4).Infof("generate rule: clone reference resource '%s/%s'  present in namespace '%s'", rule.Generation.Kind, rule.Generation.Clone.Name, rule.Generation.Clone.Namespace)
		rdata = resource.UnstructuredContent()
	}
	if processExisting {
		glog.V(4).Infof("resource '%s/%s' not found in existing namespace '%s'", rule.Generation.Kind, rule.Generation.Name, ns.GetName())
		resp.Success = false
		resp.Message = fmt.Sprintf("resource '%s/%s' not found in existing namespace '%s'", rule.Generation.Kind, rule.Generation.Name, ns.GetName())
		// for existing resources we generate an error which indirectly generates a policy violation
		return resp
	}
	resource.SetUnstructuredContent(rdata)
	resource.SetName(rule.Generation.Name)
	resource.SetNamespace(ns.GetName())
	// Reset resource version
	resource.SetResourceVersion("")
	_, err = client.CreateResource(rule.Generation.Kind, ns.GetName(), resource, false)
	if err != nil {
		glog.V(4).Infof("generate rule: unable to create resource %s/%s/%s: %v", rule.Generation.Kind, resource.GetNamespace(), resource.GetName(), err)
		resp.Success = false
		resp.Message = fmt.Sprintf("unable to create resource %s/%s/%s: %v", rule.Generation.Kind, resource.GetNamespace(), resource.GetName(), err)
		return resp
	}
	glog.V(4).Infof("generate rule: created resource %s/%s/%s", rule.Generation.Kind, resource.GetNamespace(), resource.GetName())
	resp.Success = true
	resp.Message = fmt.Sprintf("created resource %s/%s/%s", rule.Generation.Kind, resource.GetNamespace(), resource.GetName())
	return resp
}

//checkResource checks if the config is present in th eresource
func checkResource(ctx context.EvalInterface, config interface{}, resource *unstructured.Unstructured) (bool, error) {
	// we are checking if config is a subset of resource with default pattern
	path, err := validate.ValidateResourceWithPattern(ctx, resource.Object, config)
	if err != nil {
		glog.V(4).Infof("config not a subset of resource. failed at path %s: %v", path, err)
		return false, err
	}
	return true, nil
}
