package admissionpolicygenerator

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
)

// getClusterPolicy gets the Kyverno ClusterPolicy
func (c *controller) getClusterPolicy(name string) (*kyvernov1.ClusterPolicy, error) {
	cpolicy, err := c.cpolLister.Get(name)
	if err != nil {
		return nil, err
	}
	return cpolicy, nil
}

// getValidatngPolicy gets the Kyverno ValidatingPolicy
func (c *controller) getValidatingPolicy(name string) (*policiesv1alpha1.ValidatingPolicy, error) {
	vpol, err := c.vpolLister.Get(name)
	if err != nil {
		return nil, err
	}
	return vpol, nil
}

// getMutatingPolicy gets the Kyverno MutatingPolicy
func (c *controller) getMutatingPolicy(name string) (*policiesv1alpha1.MutatingPolicy, error) {
	mpol, err := c.mpolLister.Get(name)
	if err != nil {
		return nil, err
	}
	return mpol, nil
}

// getValidatingAdmissionPolicy gets the Kubernetes ValidatingAdmissionPolicy
func (c *controller) getValidatingAdmissionPolicy(name string) (*admissionregistrationv1.ValidatingAdmissionPolicy, error) {
	vap, err := c.vapLister.Get(name)
	if err != nil {
		return nil, err
	}
	return vap, nil
}

// getValidatingAdmissionPolicyBinding gets the Kubernetes ValidatingAdmissionPolicyBinding
func (c *controller) getValidatingAdmissionPolicyBinding(name string) (*admissionregistrationv1.ValidatingAdmissionPolicyBinding, error) {
	vapbinding, err := c.vapbindingLister.Get(name)
	if err != nil {
		return nil, err
	}
	return vapbinding, nil
}

// getMutatingAdmissionPolicy gets the Kubernetes MutatingAdmissionPolicy
func (c *controller) getMutatingAdmissionPolicy(name string) (*admissionregistrationv1alpha1.MutatingAdmissionPolicy, error) {
	mapol, err := c.mapLister.Get(name)
	if err != nil {
		return nil, err
	}
	return mapol, nil
}

// getMutatingAdmissionPolicyBinding gets the Kubernetes MutatingAdmissionPolicyBinding
func (c *controller) getMutatingAdmissionPolicyBinding(name string) (*admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding, error) {
	mapbinding, err := c.mapbindingLister.Get(name)
	if err != nil {
		return nil, err
	}
	return mapbinding, nil
}

// getExceptions get PolicyExceptions that match both the ClusterPolicy and the rule if exists.
func (c *controller) getExceptions(policyName, rule string) ([]kyvernov2.PolicyException, error) {
	var exceptions []kyvernov2.PolicyException
	polexs, err := c.polexLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, polex := range polexs {
		if polex.Contains(policyName, rule) {
			exceptions = append(exceptions, *polex)
		}
	}
	return exceptions, nil
}

// getCELExceptions get PolicyExceptions that match the ValidatingPolicy.
func (c *controller) getCELExceptions(policyName string) ([]policiesv1alpha1.PolicyException, error) {
	var exceptions []policiesv1alpha1.PolicyException
	polexs, err := c.celpolexLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	for _, polex := range polexs {
		for _, policy := range polex.Spec.PolicyRefs {
			if policy.Name == policyName {
				exceptions = append(exceptions, *polex)
			}
		}
	}
	return exceptions, nil
}

func constructBindingName(polName string) string {
	return polName + "-binding"
}
