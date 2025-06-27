package admissionpolicygenerator

import (
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
)

// this file contains the handler functions for MAP and bindings resources.
func (c *controller) addMAP(obj *admissionregistrationv1alpha1.MutatingAdmissionPolicy) {
	c.enqueueMAP(obj)
}

func (c *controller) updateMAP(old, obj *admissionregistrationv1alpha1.MutatingAdmissionPolicy) {
	if datautils.DeepEqual(old.Spec, obj.Spec) {
		return
	}
	c.enqueueMAP(obj)
}

func (c *controller) deleteMAP(obj *admissionregistrationv1alpha1.MutatingAdmissionPolicy) {
	c.enqueueMAP(obj)
}

func (c *controller) enqueueMAP(m *admissionregistrationv1alpha1.MutatingAdmissionPolicy) {
	if len(m.OwnerReferences) == 1 {
		if m.OwnerReferences[0].Kind == "MutatingPolicy" {
			mpol, err := c.mpolLister.Get(m.OwnerReferences[0].Name)
			if err != nil {
				return
			}
			c.enqueueMP(mpol)
		}
	}
}

func (c *controller) addMAPbinding(obj *admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding) {
	c.enqueueMAPbinding(obj)
}

func (c *controller) updateMAPbinding(old, obj *admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding) {
	if datautils.DeepEqual(old.Spec, obj.Spec) {
		return
	}
	c.enqueueMAPbinding(obj)
}

func (c *controller) deleteMAPbinding(obj *admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding) {
	c.enqueueMAPbinding(obj)
}

func (c *controller) enqueueMAPbinding(mb *admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding) {
	if len(mb.OwnerReferences) == 1 {
		if mb.OwnerReferences[0].Kind == "MutatingPolicy" {
			mpol, err := c.mpolLister.Get(mb.OwnerReferences[0].Name)
			if err != nil {
				return
			}
			c.enqueueMP(mpol)
		}
	}
}
