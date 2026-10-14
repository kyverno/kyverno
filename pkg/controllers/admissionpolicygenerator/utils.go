package admissionpolicygenerator

import (
	"fmt"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
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
func (c *controller) getValidatingPolicy(name string) (*policiesv1beta1.ValidatingPolicy, error) {
	vpol, err := c.vpolLister.Get(name)
	if err != nil {
		return nil, err
	}
	return vpol, nil
}

// getMutatingPolicy gets the Kyverno MutatingPolicy
func (c *controller) getMutatingPolicy(name string) (*policiesv1beta1.MutatingPolicy, error) {
	mpol, err := c.mpolLister.Get(name)
	if err != nil {
		return nil, err
	}
	return mpol, nil
}

// getValidatingAdmissionPolicy gets the Kubernetes ValidatingAdmissionPolicy
func (c *controller) getValidatingAdmissionPolicy(name string) (*admissionregistrationv1.ValidatingAdmissionPolicy, error) {
	if c.vapLister == nil {
		return nil, fmt.Errorf("ValidatingAdmissionPolicy lister is nil")
	}
	vap, err := c.vapLister.Get(name)
	if err != nil {
		return nil, err
	}
	return vap, nil
}

// getValidatingAdmissionPolicyBinding gets the Kubernetes ValidatingAdmissionPolicyBinding
func (c *controller) getValidatingAdmissionPolicyBinding(name string) (*admissionregistrationv1.ValidatingAdmissionPolicyBinding, error) {
	if c.vapbindingLister == nil {
		return nil, fmt.Errorf("ValidatingAdmissionPolicyBinding lister is nil")
	}
	vapbinding, err := c.vapbindingLister.Get(name)
	if err != nil {
		return nil, err
	}
	return vapbinding, nil
}

// getMutatingAdmissionPolicy gets the Kubernetes MutatingAdmissionPolicy (v1alpha1)
func (c *controller) getMutatingAdmissionPolicy(name string) (*admissionregistrationv1alpha1.MutatingAdmissionPolicy, error) {
	if c.mapAlphaLister == nil {
		return nil, fmt.Errorf("MutatingAdmissionPolicy v1alpha1 lister is nil")
	}
	return c.mapAlphaLister.Get(name)
}

// getMutatingAdmissionPolicyV1 gets the Kubernetes MutatingAdmissionPolicy (v1)
func (c *controller) getMutatingAdmissionPolicyV1(name string) (*admissionregistrationv1.MutatingAdmissionPolicy, error) {
	if c.mapV1Lister == nil {
		return nil, fmt.Errorf("MutatingAdmissionPolicy v1 lister is nil")
	}
	return c.mapV1Lister.Get(name)
}

// getMutatingAdmissionPolicyBindingV1 gets the Kubernetes MutatingAdmissionPolicyBinding (v1)
func (c *controller) getMutatingAdmissionPolicyBindingV1(name string) (*admissionregistrationv1.MutatingAdmissionPolicyBinding, error) {
	if c.mapbindingV1Lister == nil {
		return nil, fmt.Errorf("MutatingAdmissionPolicyBinding v1 lister is nil")
	}
	return c.mapbindingV1Lister.Get(name)
}

// getMutatingAdmissionPolicyBinding gets the Kubernetes MutatingAdmissionPolicyBinding (v1alpha1)
func (c *controller) getMutatingAdmissionPolicyBinding(name string) (*admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding, error) {
	if c.mapbindingAlphaLister == nil {
		return nil, fmt.Errorf("MutatingAdmissionPolicyBinding v1alpha1 lister is nil")
	}
	return c.mapbindingAlphaLister.Get(name)
}

// getMutatingAdmissionPolicyBeta gets the Kubernetes MutatingAdmissionPolicy (v1beta1)
func (c *controller) getMutatingAdmissionPolicyBeta(name string) (*admissionregistrationv1beta1.MutatingAdmissionPolicy, error) {
	if c.mapBetaLister == nil {
		return nil, fmt.Errorf("MutatingAdmissionPolicy v1beta1 lister is nil")
	}
	return c.mapBetaLister.Get(name)
}

// getMutatingAdmissionPolicyBindingBeta gets the Kubernetes MutatingAdmissionPolicyBinding (v1beta1)
func (c *controller) getMutatingAdmissionPolicyBindingBeta(name string) (*admissionregistrationv1beta1.MutatingAdmissionPolicyBinding, error) {
	if c.mapbindingBetaLister == nil {
		return nil, fmt.Errorf("MutatingAdmissionPolicyBinding v1beta1 lister is nil")
	}
	return c.mapbindingBetaLister.Get(name)
}

// getExceptions get PolicyExceptions that match both the ClusterPolicy and the rule if exists.
// Gated the same way as the runtime v2 exceptions path (cmd/internal/engine.go's
// NewExceptionSelector): exceptions are only considered if --enablePolicyException is set,
// and an empty --exceptionNamespace disables exception selection entirely rather than being
// read as cluster-wide (that reading is CEL-only, see getCELExceptions below).
//
// The "*"-vs-namespace branch below mirrors pkg/controllers/exceptions/controller.go's
// listExceptions(), the runtime source of this semantics. Keep the two in sync if that
// logic ever changes.
func (c *controller) getExceptions(policyName, rule string) ([]kyvernov2.PolicyException, error) {
	if !c.polexEnabled {
		return nil, nil
	}
	if c.exceptionNamespace == "" {
		logger.V(2).Info("PolicyExceptions are enabled but no exceptionNamespace is configured; ignoring exceptions", "policy", policyName)
		return nil, nil
	}

	var polexs []*kyvernov2.PolicyException
	var err error
	if c.exceptionNamespace == "*" {
		polexs, err = c.polexLister.List(labels.Everything())
	} else {
		polexs, err = c.polexLister.PolicyExceptions(c.exceptionNamespace).List(labels.Everything())
	}
	if err != nil {
		return nil, err
	}

	var exceptions []kyvernov2.PolicyException
	for _, polex := range polexs {
		if polex.Contains(policyName, rule) {
			exceptions = append(exceptions, *polex)
		}
	}
	return exceptions, nil
}

// getCELExceptions get PolicyExceptions that match the given policy name and kind.
// Delegates to the same runtime helpers the CEL engines use (pkg/cel/engine/exception.go)
// so the enable gate, namespace scoping, expiry, and kind+name matching stay in lockstep
// with runtime enforcement instead of being reimplemented here.
func (c *controller) getCELExceptions(policyName, kind string) ([]policiesv1beta1.PolicyException, error) {
	if !c.polexEnabled {
		return nil, nil
	}

	lister := celengine.NewPolicyExceptionLister(c.celpolexLister, c.exceptionNamespace)
	polexs, err := celengine.ListExceptions(lister, kind, policyName)
	if err != nil {
		return nil, err
	}

	var exceptions []policiesv1beta1.PolicyException
	for _, polex := range polexs {
		exceptions = append(exceptions, *polex)
	}
	return exceptions, nil
}

func constructBindingName(polName string) string {
	return polName + "-binding"
}
