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

//ClusterPV ...
type clusterPV struct {
	// dynamic client
	dclient *client.Client
	// get/list cluster policy violation
	cpvLister kyvernolister.ClusterPolicyViolationLister
	// policy violation interface
	kyvernoInterface kyvernov1.KyvernoV1Interface
	// logger
	log logr.Logger
	// update policy stats with violationCount
	policyStatusListener policystatus.Listener
}

func newClusterPV(log logr.Logger, dclient *client.Client,
	cpvLister kyvernolister.ClusterPolicyViolationLister,
	kyvernoInterface kyvernov1.KyvernoV1Interface,
	policyStatus policystatus.Listener,
) *clusterPV {
	cpv := clusterPV{
		dclient:              dclient,
		cpvLister:            cpvLister,
		kyvernoInterface:     kyvernoInterface,
		log:                  log,
		policyStatusListener: policyStatus,
	}
	return &cpv
}

func (cpv *clusterPV) create(pv kyverno.PolicyViolationTemplate) error {
	newPv := kyverno.ClusterPolicyViolation(pv)
	// PV already exists
	oldPv, err := cpv.getExisting(newPv)
	if err != nil {
		return err
	}
	if oldPv == nil {
		// create a new policy violation
		return cpv.createPV(&newPv)
	}
	// policy violation exists
	// skip if there is not change, else update the violation
	return cpv.updatePV(&newPv, oldPv)
}

func (cpv *clusterPV) getExisting(newPv kyverno.ClusterPolicyViolation) (*kyverno.ClusterPolicyViolation, error) {
	logger := cpv.log.WithValues("namespace", newPv.Namespace, "name", newPv.Name)
	var err error
	// use labels
	policyLabelmap := map[string]string{"policy": newPv.Spec.Policy, "resource": newPv.Spec.ResourceSpec.ToKey()}
	ls, err := converLabelToSelector(policyLabelmap)
	if err != nil {
		return nil, err
	}

	pvs, err := cpv.cpvLister.List(ls)
	if err != nil {
		logger.Error(err, "failed to list cluster policy violations")
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

func (cpv *clusterPV) createPV(newPv *kyverno.ClusterPolicyViolation) error {
	var err error
	logger := cpv.log.WithValues("policy", newPv.Spec.Policy, "kind", newPv.Spec.ResourceSpec.Kind, "namespace", newPv.Spec.ResourceSpec.Namespace, "name", newPv.Spec.ResourceSpec.Name)
	logger.V(4).Info("creating new policy violation")
	obj, err := retryGetResource(cpv.dclient, newPv.Spec.ResourceSpec)
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
	_, err = cpv.kyvernoInterface.ClusterPolicyViolations().Create(newPv)
	if err != nil {
		logger.Error(err, "failed to create cluster policy violation")
		return err
	}

	if newPv.Annotations["fromSync"] != "true" {
		cpv.policyStatusListener.Send(violationCount{policyName: newPv.Spec.Policy, violatedRules: newPv.Spec.ViolatedRules})
	}

	logger.Info("cluster policy violation created")
	return nil
}

func (cpv *clusterPV) updatePV(newPv, oldPv *kyverno.ClusterPolicyViolation) error {
	logger := cpv.log.WithValues("policy", newPv.Spec.Policy, "kind", newPv.Spec.ResourceSpec.Kind, "namespace", newPv.Spec.ResourceSpec.Namespace, "name", newPv.Spec.ResourceSpec.Name)
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
	_, err = cpv.kyvernoInterface.ClusterPolicyViolations().Update(newPv)
	if err != nil {
		return fmt.Errorf("failed to update cluster policy violation: %v", err)
	}
	logger.Info("cluster policy violation updated")

	if newPv.Annotations["fromSync"] != "true" {
		cpv.policyStatusListener.Send(violationCount{policyName: newPv.Spec.Policy, violatedRules: newPv.Spec.ViolatedRules})
	}
	return nil
}
