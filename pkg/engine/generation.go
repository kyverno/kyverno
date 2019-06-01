package engine

import (
	"fmt"

	"github.com/golang/glog"
	kubepolicy "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Generate should be called to process generate rules on the resource
func Generate(client *client.Client, policy kubepolicy.Policy, rawResource []byte, gvk metav1.GroupVersionKind) {
	// configMapGenerator and secretGenerator can be applied only to namespaces
	// TODO: support for any resource
	if gvk.Kind != "Namespace" {
		return
	}

	for _, rule := range policy.Spec.Rules {
		ok := ResourceMeetsDescription(rawResource, rule.ResourceDescription, gvk)

		if !ok {
			glog.Infof("Rule is not applicable to the request: rule name = %s in policy %s \n", rule.Name, policy.ObjectMeta.Name)
			continue
		}

		err := applyRuleGenerator(client, rawResource, rule.Generation, gvk)
		if err != nil {
			glog.Warningf("Failed to apply rule generator: %v", err)
		}
	}
}

// Applies "configMapGenerator" and "secretGenerator" described in PolicyRule
// TODO: plan to support all kinds of generator
func applyRuleGenerator(client *client.Client, rawResource []byte, generator *kubepolicy.Generation, gvk metav1.GroupVersionKind) error {
	if generator == nil {
		return nil
	}

	var err error

	namespace := ParseNameFromObject(rawResource)
	err = client.GenerateResource(*generator, namespace)
	if err != nil {
		return fmt.Errorf("Unable to apply generator for %s '%s/%s' : %v", generator.Kind, namespace, generator.Name, err)
	}
	glog.Infof("Successfully applied generator %s/%s", generator.Kind, generator.Name)
	return nil
}
