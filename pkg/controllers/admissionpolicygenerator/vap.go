package admissionpolicygenerator

import (
	"strings"

	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

// this file contains the handler functions for VAP and bindings resources.
func (c *controller) addVAP(obj *admissionregistrationv1.ValidatingAdmissionPolicy) {
	c.enqueueVAP(obj)
}

func (c *controller) updateVAP(old, obj *admissionregistrationv1.ValidatingAdmissionPolicy) {
	if datautils.DeepEqual(old.Spec, obj.Spec) {
		return
	}
	c.enqueueVAP(obj)
}

func (c *controller) deleteVAP(obj *admissionregistrationv1.ValidatingAdmissionPolicy) {
	c.enqueueVAP(obj)
	c.enqueueVAPName(obj.Name)
}

func (c *controller) enqueueVAP(v *admissionregistrationv1.ValidatingAdmissionPolicy) {
	if len(v.OwnerReferences) == 1 {
		if v.OwnerReferences[0].Kind == "ValidatingPolicy" {
			vpol, err := c.vpolLister.Get(v.OwnerReferences[0].Name)
			if err != nil {
				return
			}
			c.enqueueVP(vpol)
		}
	}
}

func (c *controller) addVAPbinding(obj *admissionregistrationv1.ValidatingAdmissionPolicyBinding) {
	c.enqueueVAPbinding(obj)
}

func (c *controller) updateVAPbinding(old, obj *admissionregistrationv1.ValidatingAdmissionPolicyBinding) {
	if datautils.DeepEqual(old.Spec, obj.Spec) {
		return
	}
	c.enqueueVAPbinding(obj)
}

func (c *controller) deleteVAPbinding(obj *admissionregistrationv1.ValidatingAdmissionPolicyBinding) {
	c.enqueueVAPbinding(obj)
	if name, ok := strings.CutSuffix(obj.Name, "-binding"); ok {
		c.enqueueVAPName(name)
	}
}

func (c *controller) enqueueVAPbinding(vb *admissionregistrationv1.ValidatingAdmissionPolicyBinding) {
	if len(vb.OwnerReferences) == 1 {
		if vb.OwnerReferences[0].Kind == "ValidatingPolicy" {
			vpol, err := c.vpolLister.Get(vb.OwnerReferences[0].Name)
			if err != nil {
				return
			}
			c.enqueueVP(vpol)
		}
	}
}

// enqueueVAPName wakes the policy whose generated name became available, even
// when the deleted resource had no matching owner. This is only used on delete;
// reconciliation still verifies ownership before managing either native object.
func (c *controller) enqueueVAPName(nativeName string) {
	name, ok := strings.CutPrefix(nativeName, "vpol-")
	if !ok || name == "" {
		return
	}
	if vpol, err := c.vpolLister.Get(name); err == nil {
		c.enqueueVP(vpol)
	}
}
