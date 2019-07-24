package engine

import (
	"encoding/json"
	"errors"

	"github.com/golang/glog"
	v1alpha1 "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/info"
	"github.com/nirmata/kyverno/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

//GenerateNew apply generation rules on a resource
func GenerateNew(client *client.Client, policy *v1alpha1.Policy, ns unstructured.Unstructured) []*info.RuleInfo {
	ris := []*info.RuleInfo{}
	for _, rule := range policy.Spec.Rules {
		if rule.Generation == nil {
			continue
		}
		ri := info.NewRuleInfo(rule.Name, info.Generation)
		err := applyRuleGeneratorNew(client, ns, rule.Generation)
		if err != nil {
			ri.Fail()
			ri.Addf("Rule %s: Failed to apply rule generator, err %v.", rule.Name, err)
		} else {
			ri.Addf("Rule %s: Generation succesfully.", rule.Name)
		}
		ris = append(ris, ri)

	}
	return ris
}

func applyRuleGeneratorNew(client *client.Client, ns unstructured.Unstructured, gen *v1alpha1.Generation) error {
	var err error
	resource := &unstructured.Unstructured{}
	var rdata map[string]interface{}

	if gen.Data != nil {
		// 1> Check if resource exists
		obj, err := client.GetResource(gen.Kind, ns.GetName(), gen.Name)
		if err == nil {
			// 2> If already exsists, then verify the content is contained
			// found the resource
			// check if the rule is create, if yes, then verify if the specified configuration is present in the resource
			ok, err := checkResource(gen.Data, obj)
			if err != nil {
				return err
			}
			if !ok {
				return errors.New("rule configuration not present in resource")
			}
			return nil
		}
		rdata, err = runtime.DefaultUnstructuredConverter.ToUnstructured(&gen.Data)
		if err != nil {
			glog.Error(err)
			return err
		}
	}
	if gen.Clone != nil {
		// 1> Check if resource exists
		_, err := client.GetResource(gen.Kind, ns.GetName(), gen.Name)
		if err == nil {
			return nil
		}
		// 2> If already exists return
		resource, err = client.GetResource(gen.Kind, gen.Clone.Namespace, gen.Clone.Name)
		if err != nil {
			return err
		}
		rdata = resource.UnstructuredContent()
	}
	resource.SetUnstructuredContent(rdata)
	resource.SetName(gen.Name)
	resource.SetNamespace(ns.GetName())
	// Reset resource version
	resource.SetResourceVersion("")

	_, err = client.CreateResource(gen.Kind, ns.GetName(), resource, false)
	if err != nil {
		return err
	}
	return nil
}

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
