package policyviolation

import (
	"fmt"
	"reflect"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernov1 "github.com/nirmata/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//NamespacedPV ...
type namespacedPV struct {
	// dynamic client
	dclient *client.Client
	// get/list namespaced policy violation
	nspvLister kyvernolister.NamespacedPolicyViolationLister
	// policy violation interface
	kyvernoInterface kyvernov1.KyvernoV1Interface
}

func newNamespacedPV(dclient *client.Client,
	nspvLister kyvernolister.NamespacedPolicyViolationLister,
	kyvernoInterface kyvernov1.KyvernoV1Interface,
) *namespacedPV {
	nspv := namespacedPV{
		dclient:          dclient,
		nspvLister:       nspvLister,
		kyvernoInterface: kyvernoInterface,
	}
	return &nspv
}

func (nspv *namespacedPV) create(pv kyverno.PolicyViolation) error {
	newPv := kyverno.NamespacedPolicyViolation(pv)
	// PV already exists
	oldPv, err := nspv.getExisting(newPv)
	if err != nil {
		return err
	}
	if oldPv == nil {
		// create a new policy violation
		return nspv.createPV(&newPv)
	}
	// policy violation exists
	// skip if there is not change, else update the violation
	return nspv.updatePV(&newPv, oldPv)
}

func (nspv *namespacedPV) getExisting(newPv kyverno.NamespacedPolicyViolation) (*kyverno.NamespacedPolicyViolation, error) {
	var err error
	// use labels
	policyLabelmap := map[string]string{"policy": newPv.Spec.Policy, "resource": newPv.Spec.ResourceSpec.ToKey()}
	ls, err := converLabelToSelector(policyLabelmap)
	if err != nil {
		return nil, err
	}
	pvs, err := nspv.nspvLister.NamespacedPolicyViolations(newPv.GetNamespace()).List(ls)
	if err != nil {
		glog.Errorf("unable to list namespaced policy violations : %v", err)
		return nil, err
	}

	for _, pv := range pvs {
		// find a policy on same resource and policy combination
		if pv.Spec.Policy == newPv.Spec.Policy &&
			pv.Spec.ResourceSpec.Kind == newPv.Spec.ResourceSpec.Kind &&
			pv.Spec.ResourceSpec.Name == newPv.Spec.ResourceSpec.Name {
			return pv, nil
		}
	}
	return nil, nil
}

func (nspv *namespacedPV) createPV(newPv *kyverno.NamespacedPolicyViolation) error {
	var err error
	glog.V(4).Infof("creating new policy violation for policy %s & resource %s/%s/%s", newPv.Spec.Policy, newPv.Spec.ResourceSpec.Kind, newPv.Spec.ResourceSpec.Namespace, newPv.Spec.ResourceSpec.Name)
	obj, err := retryGetResource(nspv.dclient, newPv.Spec.ResourceSpec)
	if err != nil {
		return fmt.Errorf("failed to retry getting resource for policy violation %s/%s: %v", newPv.Name, newPv.Spec.Policy, err)
	}
	// set owner reference to resource
	ownerRef := createOwnerReference(obj)
	newPv.SetOwnerReferences([]metav1.OwnerReference{ownerRef})

	// create resource
	_, err = nspv.kyvernoInterface.NamespacedPolicyViolations(newPv.GetNamespace()).Create(newPv)
	if err != nil {
		glog.V(4).Infof("failed to create Cluster Policy Violation: %v", err)
		return err
	}
	glog.Infof("policy violation created for resource %v", newPv.Spec.ResourceSpec)
	return nil
}

func (nspv *namespacedPV) updatePV(newPv, oldPv *kyverno.NamespacedPolicyViolation) error {
	var err error
	// check if there is any update
	if reflect.DeepEqual(newPv.Spec, oldPv.Spec) {
		glog.V(4).Infof("policy violation spec %v did not change so not updating it", newPv.Spec)
		return nil
	}
	// set name
	newPv.SetName(oldPv.Name)
	newPv.SetResourceVersion(oldPv.ResourceVersion)
	// update resource
	_, err = nspv.kyvernoInterface.NamespacedPolicyViolations(newPv.GetNamespace()).Update(newPv)
	if err != nil {
		return fmt.Errorf("failed to update namespaced polciy violation: %v", err)
	}
	glog.Infof("namespced policy violation updated for resource %v", newPv.Spec.ResourceSpec)
	return nil
}
