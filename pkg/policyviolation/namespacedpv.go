package policyviolation

import (
	"fmt"

	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernov1 "github.com/nirmata/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/policystatus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//NamespacedPV ...
type namespacedPV struct {
	// dynamic client
	dclient *client.Client
	// get/list namespaced policy violation
	nspvLister kyvernolister.PolicyViolationLister
	// policy violation interface
	kyvernoInterface kyvernov1.KyvernoV1Interface
	// logger
	log logr.Logger
	// update policy status with violationCount
	policyStatusListener policystatus.Listener
}

func newNamespacedPV(log logr.Logger, dclient *client.Client,
	nspvLister kyvernolister.PolicyViolationLister,
	kyvernoInterface kyvernov1.KyvernoV1Interface,
	policyStatus policystatus.Listener,
) *namespacedPV {
	nspv := namespacedPV{
		dclient:              dclient,
		nspvLister:           nspvLister,
		kyvernoInterface:     kyvernoInterface,
		log:                  log,
		policyStatusListener: policyStatus,
	}
	return &nspv
}

func (nspv *namespacedPV) create(pv kyverno.PolicyViolationTemplate) error {
	newPv := kyverno.PolicyViolation(pv)
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

func (nspv *namespacedPV) getExisting(newPv kyverno.PolicyViolation) (*kyverno.PolicyViolation, error) {
	logger := nspv.log.WithValues("namespace", newPv.Namespace, "name", newPv.Name)
	var err error
	// use labels
	policyLabelmap := map[string]string{"policy": newPv.Spec.Policy, "resource": newPv.Spec.ResourceSpec.ToKey()}
	ls, err := converLabelToSelector(policyLabelmap)
	if err != nil {
		return nil, err
	}
	pvs, err := nspv.nspvLister.PolicyViolations(newPv.GetNamespace()).List(ls)
	if err != nil {
		logger.Error(err, "failed to list namespaced policy violations")
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

func (nspv *namespacedPV) createPV(newPv *kyverno.PolicyViolation) error {
	var err error
	logger := nspv.log.WithValues("policy", newPv.Spec.Policy, "kind", newPv.Spec.ResourceSpec.Kind, "namespace", newPv.Spec.ResourceSpec.Namespace, "name", newPv.Spec.ResourceSpec.Name)
	logger.V(4).Info("creating new policy violation")
	obj, err := retryGetResource(nspv.dclient, newPv.Spec.ResourceSpec)
	if err != nil {
		return fmt.Errorf("failed to retry getting resource for policy violation %s/%s: %v", newPv.Name, newPv.Spec.Policy, err)
	}

	if obj.GetDeletionTimestamp() != nil {
		return nil
	}

	// set owner reference to resource
	ownerRef, ok := createOwnerReference(obj)
	if !ok {
		return nil
	}

	newPv.SetOwnerReferences([]metav1.OwnerReference{ownerRef})

	// create resource
	_, err = nspv.kyvernoInterface.PolicyViolations(newPv.GetNamespace()).Create(newPv)
	if err != nil {
		logger.Error(err, "failed to create namespaced policy violation")
		return err
	}

	if newPv.Annotations["fromSync"] != "true" {
		nspv.policyStatusListener.Send(violationCount{policyName: newPv.Spec.Policy, violatedRules: newPv.Spec.ViolatedRules})
	}
	logger.Info("namespaced policy violation created")
	return nil
}

func (nspv *namespacedPV) updatePV(newPv, oldPv *kyverno.PolicyViolation) error {
	logger := nspv.log.WithValues("policy", newPv.Spec.Policy, "kind", newPv.Spec.ResourceSpec.Kind, "namespace", newPv.Spec.ResourceSpec.Namespace, "name", newPv.Spec.ResourceSpec.Name)
	var err error
	// check if there is any update
	if !hasViolationSpecChanged(newPv.Spec.DeepCopy(), oldPv.Spec.DeepCopy()) {
		logger.V(4).Info("policy violation spec did not change, not upadating the resource")
		return nil
	}
	// set name
	newPv.SetName(oldPv.Name)
	newPv.SetResourceVersion(oldPv.ResourceVersion)
	newPv.SetOwnerReferences(oldPv.GetOwnerReferences())
	// update resource
	_, err = nspv.kyvernoInterface.PolicyViolations(newPv.GetNamespace()).Update(newPv)
	if err != nil {
		return fmt.Errorf("failed to update namespaced policy violation: %v", err)
	}

	if newPv.Annotations["fromSync"] != "true" {
		nspv.policyStatusListener.Send(violationCount{policyName: newPv.Spec.Policy, violatedRules: newPv.Spec.ViolatedRules})
	}
	logger.Info("namespaced policy violation updated")
	return nil
}
