package policyengine

import (
	"fmt"

	kubepolicy "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kube-policy/pkg/policyengine/mutation"
)

// TODO: To be reworked due to spec policy-v2

// Applies "configMapGenerator" and "secretGenerator" described in PolicyRule
func (p *policyEngine) applyRuleGenerators(rawResource []byte, rule kubepolicy.Rule) error {
	kind := mutation.ParseKindFromObject(rawResource)

	// configMapGenerator and secretGenerator can be applied only to namespaces
	if kind == "Namespace" {
		namespaceName := mutation.ParseNameFromObject(rawResource)

		err := p.applyConfigGenerator(rule.Generation, namespaceName, "ConfigMap")
		if err == nil {
			err = p.applyConfigGenerator(rule.Generation, namespaceName, "Secret")
		}
		return err
	}
	return nil
}

// Creates resourceKind (ConfigMap or Secret) with parameters specified in generator in cluster specified in request.
func (p *policyEngine) applyConfigGenerator(generator *kubepolicy.Generation, namespace string, configKind string) error {
	if generator == nil {
		return nil
	}

	err := generator.Validate()
	if err != nil {
		return fmt.Errorf("Generator for '%s' is invalid: %s", configKind, err)
	}

	switch configKind {
	case "ConfigMap":
		err = p.kubeClient.GenerateConfigMap(*generator, namespace)
	case "Secret":
		err = p.kubeClient.GenerateSecret(*generator, namespace)
	default:
		err = fmt.Errorf("Unsupported config Kind '%s'", configKind)
	}

	if err != nil {
		return fmt.Errorf("Unable to apply generator for %s '%s/%s' : %s", configKind, namespace, generator.Name, err)
	}

	return nil
}
