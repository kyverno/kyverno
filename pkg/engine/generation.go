package engine

import (
	"encoding/json"
	"errors"
	"time"

	"fmt"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/info"
	"github.com/nirmata/kyverno/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

//Generate apply generation rules on a resource
func Generate(client *client.Client, policy kyverno.Policy, ns unstructured.Unstructured) (response EngineResponse) {
	startTime := time.Now()
	glog.V(4).Infof("started applying generation rules of policy %q (%v)", policy.Name, startTime)
	defer func() {
		response.ExecutionTime = time.Since(startTime)
		glog.V(4).Infof("Finished applying generation rules policy %q (%v)", policy.Name, response.ExecutionTime)
		glog.V(4).Infof("Generation Rules appplied  count %q for policy %q", response.RulesAppliedCount, policy.Name)
	}()
	incrementAppliedRuleCount := func() {
		// rules applied succesfully count
		response.RulesAppliedCount++
	}

	ris := []info.RuleInfo{}
	for _, rule := range policy.Spec.Rules {
		if rule.Generation == (kyverno.Generation{}) {
			continue
		}
		glog.V(4).Infof("applying policy %s generate rule %s on resource %s/%s/%s", policy.Name, rule.Name, ns.GetKind(), ns.GetNamespace(), ns.GetName())
		ri := info.NewRuleInfo(rule.Name, info.Generation)
		err := applyRuleGenerator(client, ns, rule.Generation, policy.GetCreationTimestamp())
		if err != nil {
			ri.Fail()
			ri.Addf("Failed to apply rule generator, err %v.", rule.Name, err)
			glog.Infof("failed to apply policy %s rule %s on resource %s/%s/%s: %v", policy.Name, rule.Name, ns.GetKind(), ns.GetNamespace(), ns.GetName(), err)
		} else {
			ri.Addf("Generation succesfully.", rule.Name)
			glog.Infof("succesfully applied  policy %s rule %s on resource %s/%s/%s", policy.Name, rule.Name, ns.GetKind(), ns.GetNamespace(), ns.GetName())
		}
		ris = append(ris, ri)
		incrementAppliedRuleCount()
	}
	response.RuleInfos = ris
	return response
}

func applyRuleGenerator(client *client.Client, ns unstructured.Unstructured, gen kyverno.Generation, policyCreationTime metav1.Time) error {
	var err error
	resource := &unstructured.Unstructured{}
	var rdata map[string]interface{}
	// To manage existing resource , we compare the creation time for the default resource to be generate and policy creation time
	processExisting := func() bool {
		nsCreationTime := ns.GetCreationTimestamp()
		return nsCreationTime.Before(&policyCreationTime)
	}()
	if gen.Data != nil {
		glog.V(4).Info("generate rule: creates new resource")
		// 1> Check if resource exists
		obj, err := client.GetResource(gen.Kind, ns.GetName(), gen.Name)
		if err == nil {
			glog.V(4).Infof("generate rule: resource %s/%s/%s already present. checking if it contains the required configuration", gen.Kind, ns.GetName(), gen.Name)
			// 2> If already exsists, then verify the content is contained
			// found the resource
			// check if the rule is create, if yes, then verify if the specified configuration is present in the resource
			ok, err := checkResource(gen.Data, obj)
			if err != nil {
				glog.V(4).Infof("generate rule:: unable to check if configuration %v, is present in resource %s/%s/%s", gen.Data, gen.Kind, ns.GetName(), gen.Name)
				return err
			}
			if !ok {
				glog.V(4).Infof("generate rule:: configuration %v not present in resource %s/%s/%s", gen.Data, gen.Kind, ns.GetName(), gen.Name)
				return errors.New("rule configuration not present in resource")
			}
			glog.V(4).Infof("generate rule: required configuration %v is present in resource %s/%s/%s", gen.Data, gen.Kind, ns.GetName(), gen.Name)
			return nil
		}
		rdata, err = runtime.DefaultUnstructuredConverter.ToUnstructured(&gen.Data)
		if err != nil {
			glog.Error(err)
			return err
		}
	}
	if gen.Clone != (kyverno.CloneFrom{}) {
		glog.V(4).Info("generate rule: clone resource")
		// 1> Check if resource exists
		_, err := client.GetResource(gen.Kind, ns.GetName(), gen.Name)
		if err == nil {
			glog.V(4).Infof("generate rule: resource %s/%s/%s already present", gen.Kind, ns.GetName(), gen.Name)
			return nil
		}
		// 2> If clone already exists return
		resource, err = client.GetResource(gen.Kind, gen.Clone.Namespace, gen.Clone.Name)
		if err != nil {
			glog.V(4).Infof("generate rule: clone reference resource %s/%s/%s  not present: %v", gen.Kind, gen.Kind, gen.Clone.Namespace, gen.Clone.Name, err)
			return err
		}
		glog.V(4).Infof("generate rule: clone reference resource %s/%s/%s  present", gen.Kind, gen.Kind, gen.Clone.Namespace, gen.Clone.Name)
		rdata = resource.UnstructuredContent()
	}
	if processExisting {
		// for existing resources we generate an error which indirectly generates a policy violation
		return fmt.Errorf("resource %s not found in existing namespace %s", gen.Name, ns.GetName())
	}
	resource.SetUnstructuredContent(rdata)
	resource.SetName(gen.Name)
	resource.SetNamespace(ns.GetName())
	// Reset resource version
	resource.SetResourceVersion("")
	_, err = client.CreateResource(gen.Kind, ns.GetName(), resource, false)
	if err != nil {
		glog.V(4).Infof("generate rule: unable to create resource %s/%s/%s: %v", gen.Kind, resource.GetNamespace(), resource.GetName(), err)
		return err
	}
	glog.V(4).Infof("generate rule: created resource %s/%s/%s", gen.Kind, resource.GetNamespace(), resource.GetName())
	return nil
}

//checkResource checks if the config is present in th eresource
func checkResource(config interface{}, resource *unstructured.Unstructured) (bool, error) {
	var err error

	objByte, err := resource.MarshalJSON()
	if err != nil {
		// unable to parse the json
		return false, err
	}
	err = resource.UnmarshalJSON(objByte)
	if err != nil {
		// unable to parse the json
		return false, err
	}
	// marshall and unmarshall json to verify if its right format
	configByte, err := json.Marshal(config)
	if err != nil {
		// unable to marshall the config
		return false, err
	}
	var configData interface{}
	err = json.Unmarshal(configByte, &configData)
	if err != nil {
		// unable to unmarshall
		return false, err
	}

	var objData interface{}
	err = json.Unmarshal(objByte, &objData)
	if err != nil {
		// unable to unmarshall
		return false, err
	}

	// Check if the config is a subset of resource
	return utils.JSONsubsetValue(configData, objData), nil
}
