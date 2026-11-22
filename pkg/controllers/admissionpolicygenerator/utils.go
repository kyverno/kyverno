package admissionpolicygenerator

import (
	"context"
	"fmt"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type ownerPolicy interface {
	GetKind() string
	GetName() string
	GetUID() types.UID
}

// isOwnedBy reports whether obj was generated for this instance of the policy: the generator makes
// the policy the owner of each native admission policy and binding it writes. The API version is
// not compared, because the policy API is served in several versions and older releases recorded
// older ones.
func isOwnedBy(obj metav1.Object, policy ownerPolicy) bool {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.UID == policy.GetUID() && ref.Kind == policy.GetKind() && ref.Name == policy.GetName() {
			return true
		}
	}
	return false
}

// generatedObject is a native admission policy or binding generated for a policy.
type generatedObject struct {
	name   string
	delete func(context.Context, string, metav1.DeleteOptions) error
}

// appendIfGenerated appends the named object when the informer cache holds it and it was generated
// for the policy. An object that only shares the generated name, such as one created by hand, is
// left out, so that it is never deleted.
func appendIfGenerated[T metav1.Object](objs []generatedObject, policy ownerPolicy, get func(string) (T, error), name string, del func(context.Context, string, metav1.DeleteOptions) error) []generatedObject {
	if obj, err := get(name); err == nil && isOwnedBy(obj, policy) {
		return append(objs, generatedObject{name: name, delete: del})
	}
	return objs
}

// deleteGenerated deletes the objects in order. A failed delete is returned so that the workqueue
// retries it.
func deleteGenerated(ctx context.Context, objs []generatedObject) error {
	for _, obj := range objs {
		if err := obj.delete(ctx, obj.name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
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
