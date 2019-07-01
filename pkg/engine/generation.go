package engine

import (
	"fmt"

	"github.com/golang/glog"
	kubepolicy "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/info"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Generate should be called to process generate rules on the resource
func Generate(client *client.Client, policy kubepolicy.Policy, rawResource []byte, gvk metav1.GroupVersionKind, processExisting bool) []*info.RuleInfo {
	ris := []*info.RuleInfo{}

	for _, rule := range policy.Spec.Rules {
		if rule.Generation == nil {
			continue
		}

		ri := info.NewRuleInfo(rule.Name, info.Generation)

		ok := ResourceMeetsDescription(rawResource, rule.ResourceDescription, gvk)
		if !ok {
			glog.Infof("Rule is not applicable to the request: rule name = %s in policy %s \n", rule.Name, policy.ObjectMeta.Name)
			continue
		}

		err := applyRuleGenerator(client, rawResource, rule.Generation, gvk, processExisting)
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

func applyRuleGenerator(client *client.Client, rawResource []byte, generator *kubepolicy.Generation, gvk metav1.GroupVersionKind, processExistingResources bool) error {

	var err error

	namespace := ParseNameFromObject(rawResource)
	err = client.GenerateResource(*generator, namespace, processExistingResources)
	if err != nil {
		return fmt.Errorf("Unable to apply generator for %s '%s/%s' : %v", generator.Kind, namespace, generator.Name, err)
	}
	glog.Infof("Successfully applied generator %s '%s/%s'", generator.Kind, namespace, generator.Name)
	return nil
}
