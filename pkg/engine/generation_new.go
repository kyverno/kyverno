package engine

import (
	"fmt"

	"github.com/golang/glog"
	v1alpha1 "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/info"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

//GenerateNew apply generation rules on a resource
func GenerateNew(client *client.Client, policy *v1alpha1.Policy, ns *corev1.Namespace, processExisting bool) []*info.RuleInfo {
	ris := []*info.RuleInfo{}
	for _, rule := range policy.Spec.Rules {
		if rule.Generation == nil {
			continue
		}
		ri := info.NewRuleInfo(rule.Name, info.Generation)
		err := applyRuleGeneratorNew(client, ns, rule.Generation, processExisting)
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

func applyRuleGeneratorNew(client *client.Client, ns *corev1.Namespace, gen *v1alpha1.Generation, processExisting bool) error {
	var err error
	resource := &unstructured.Unstructured{}
	// get resource from kind
	rGVR := client.DiscoveryClient.GetGVRFromKind(gen.Kind)
	if rGVR.Resource == "" {
		return fmt.Errorf("Kind to Resource Name conversion failed for %s", gen.Kind)
	}
	// If processing Existing resource, we only check if the resource
	// already exists
	if processExisting {
		obj, err := client.GetResource(rGVR.Resource, ns.Name, gen.Name)
		if err != nil {
			return err
		}
		data := []byte{}
		if err := obj.UnmarshalJSON(data); err != nil {
			fmt.Println(err)
		}
		fmt.Println(string(data))
	}

	var rdata map[string]interface{}
	// data -> create new resource
	if gen.Data != nil {
		rdata, err = runtime.DefaultUnstructuredConverter.ToUnstructured(&gen.Data)
		if err != nil {
			glog.Error(err)
			return err
		}
	}
	// clone -> copy from existing resource
	if gen.Clone != nil {
		resource, err = client.GetResource(rGVR.Resource, gen.Clone.Namespace, gen.Clone.Name)
		if err != nil {
			return err
		}
		rdata = resource.UnstructuredContent()
	}
	resource.SetUnstructuredContent(rdata)
	resource.SetName(gen.Name)
	resource.SetNamespace(ns.Name)
	// Reset resource version
	resource.SetResourceVersion("")

	_, err = client.CreateResource(rGVR.Resource, ns.Name, resource, false)
	if err != nil {
		return err
	}
	return nil
}
