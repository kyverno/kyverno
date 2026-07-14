package admissionpolicygenerator

import (
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// this file contains the handler functions for MAP and bindings resources.
func (c *controller) addMAPAlpha(obj *admissionregistrationv1alpha1.MutatingAdmissionPolicy) {
	c.enqueueMutatingPolicyByOwner(obj)
}

func (c *controller) updateMAPAlpha(old, obj *admissionregistrationv1alpha1.MutatingAdmissionPolicy) {
	c.enqueueMutatingPolicyByOwnerOnChange(old.Spec, obj.Spec, obj)
}

func (c *controller) deleteMAPAlpha(obj *admissionregistrationv1alpha1.MutatingAdmissionPolicy) {
	c.enqueueMutatingPolicyByOwner(obj)
}

func (c *controller) addMAPbindingAlpha(obj *admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding) {
	c.enqueueMutatingPolicyByOwner(obj)
}

func (c *controller) updateMAPbindingAlpha(old, obj *admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding) {
	c.enqueueMutatingPolicyByOwnerOnChange(old.Spec, obj.Spec, obj)
}

func (c *controller) deleteMAPbindingAlpha(obj *admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding) {
	c.enqueueMutatingPolicyByOwner(obj)
}

func (c *controller) addMAPBeta(obj *admissionregistrationv1beta1.MutatingAdmissionPolicy) {
	c.enqueueMutatingPolicyByOwner(obj)
}

func (c *controller) updateMAPBeta(old, obj *admissionregistrationv1beta1.MutatingAdmissionPolicy) {
	c.enqueueMutatingPolicyByOwnerOnChange(old.Spec, obj.Spec, obj)
}

func (c *controller) deleteMAPBeta(obj *admissionregistrationv1beta1.MutatingAdmissionPolicy) {
	c.enqueueMutatingPolicyByOwner(obj)
}

func (c *controller) addMAPbindingBeta(obj *admissionregistrationv1beta1.MutatingAdmissionPolicyBinding) {
	c.enqueueMutatingPolicyByOwner(obj)
}

func (c *controller) updateMAPbindingBeta(old, obj *admissionregistrationv1beta1.MutatingAdmissionPolicyBinding) {
	c.enqueueMutatingPolicyByOwnerOnChange(old.Spec, obj.Spec, obj)
}

func (c *controller) deleteMAPbindingBeta(obj *admissionregistrationv1beta1.MutatingAdmissionPolicyBinding) {
	c.enqueueMutatingPolicyByOwner(obj)
}

func (c *controller) enqueueMutatingPolicyByOwnerOnChange(oldSpec, newSpec any, obj metav1.Object) {
	if datautils.DeepEqual(oldSpec, newSpec) {
		return
	}
	c.enqueueMutatingPolicyByOwner(obj)
}

func (c *controller) enqueueMutatingPolicyByOwner(obj metav1.Object) {
	c.enqueueMutatingPolicyByOwnerRefs(obj.GetOwnerReferences())
}

func (c *controller) enqueueMutatingPolicyByOwnerRefs(ownerRefs []metav1.OwnerReference) {
	if len(ownerRefs) != 1 || ownerRefs[0].Kind != "MutatingPolicy" {
		return
	}
	mpol, err := c.mpolLister.Get(ownerRefs[0].Name)
	if err != nil {
		return
	}
	c.enqueueMP(mpol)
}
