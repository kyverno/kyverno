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
	labels "k8s.io/apimachinery/pkg/labels"
)

//ClusterPV ...
type clusterPV struct {
	// dynamic client
	dclient *client.Client
	// get/list cluster policy violation
	cpvLister kyvernolister.ClusterPolicyViolationLister
	// policy violation interface
	kyvernoInterface kyvernov1.KyvernoV1Interface
}

func newClusterPV(dclient *client.Client,
	cpvLister kyvernolister.ClusterPolicyViolationLister,
	kyvernoInterface kyvernov1.KyvernoV1Interface,
) *clusterPV {
	cpv := clusterPV{
		dclient:          dclient,
		cpvLister:        cpvLister,
		kyvernoInterface: kyvernoInterface,
	}
	return &cpv
}

func (cpv *clusterPV) create(pv kyverno.PolicyViolation) error {
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
	pvs, err := cpv.cpvLister.List(labels.Everything())
	if err != nil {
		glog.Errorf("unable to list cluster policy violations : %v", err)
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
	glog.V(4).Infof("creating new policy violation for policy %s & resource %s/%s", newPv.Spec.Policy, newPv.Spec.ResourceSpec.Kind, newPv.Spec.ResourceSpec.Name)
	obj, err := retryGetResource(cpv.dclient, newPv.Spec.ResourceSpec)
	if err != nil {
		return fmt.Errorf("failed to retry getting resource for policy violation %s/%s: %v", newPv.Name, newPv.Spec.Policy, err)
	}
	// set owner reference to resource
	ownerRef := createOwnerReference(obj)
	newPv.SetOwnerReferences([]metav1.OwnerReference{ownerRef})

	// create resource
	_, err = cpv.kyvernoInterface.ClusterPolicyViolations().Create(newPv)
	if err != nil {
		glog.V(4).Infof("failed to create Cluster Policy Violation: %v", err)
		return err
	}
	glog.Infof("policy violation created for resource %v", newPv.Spec.ResourceSpec)
	return nil
}

func (cpv *clusterPV) updatePV(newPv, oldPv *kyverno.ClusterPolicyViolation) error {
	var err error
	// check if there is any update
	if reflect.DeepEqual(newPv.Spec, oldPv.Spec) {
		glog.V(4).Infof("policy violation spec %v did not change so not updating it", newPv.Spec)
		return nil
	}
	// set name
	newPv.SetName(oldPv.Name)

	// update resource
	_, err = cpv.kyvernoInterface.ClusterPolicyViolations().Update(newPv)
	if err != nil {
		return fmt.Errorf("failed to update cluster policy violation: %v", err)
	}
	glog.Infof("cluster policy violation updated for resource %v", newPv.Spec.ResourceSpec)

	return nil
}
