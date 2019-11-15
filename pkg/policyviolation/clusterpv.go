package policyviolation

import (
	"fmt"
	"reflect"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernov1 "github.com/nirmata/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	labels "k8s.io/apimachinery/pkg/labels"
)

func createPVS(dclient *client.Client, pvs []kyverno.ClusterPolicyViolation, pvLister kyvernolister.ClusterPolicyViolationLister, pvInterface kyvernov1.KyvernoV1Interface) error {
	for _, pv := range pvs {
		if err := createPVNew(dclient, pv, pvLister, pvInterface); err != nil {
			return err
		}
	}
	return nil
}

func (gen *Generator) createCusterPV(info Info) error {
	var pvs []kyverno.ClusterPolicyViolation
	if !info.Blocked {
		pvs = append(pvs, buildPV(info))
	} else {
		// blocked
		// get owners
		pvs = buildPVWithOwners(gen.dclient, info)
	}
	// create policy violation
	if err := createPVS(gen.dclient, pvs, gen.pvLister, gen.pvInterface); err != nil {
		return err
	}

	glog.V(3).Infof("Created cluster policy violation policy=%s, resource=%s/%s/%s",
		info.PolicyName, info.Resource.GetKind(), info.Resource.GetNamespace(), info.Resource.GetName())
	return nil

}
func createPVNew(dclient *client.Client, pv kyverno.ClusterPolicyViolation, pvLister kyvernolister.ClusterPolicyViolationLister, pvInterface kyvernov1.KyvernoV1Interface) error {
	var err error
	// PV already exists
	ePV, err := getExistingPVIfAny(pvLister, pv)
	if err != nil {
		glog.Error(err)
		return fmt.Errorf("failed to get existing pv on resource '%s': %v", pv.Spec.ResourceSpec.ToKey(), err)
	}
	if ePV == nil {
		// Create a New PV
		glog.V(4).Infof("creating new policy violation for policy %s & resource %s/%s", pv.Spec.Policy, pv.Spec.ResourceSpec.Kind, pv.Spec.ResourceSpec.Name)
		err := retryGetResource(pv.Namespace, dclient, pv.Spec.ResourceSpec)
		if err != nil {
			return fmt.Errorf("failed to retry getting resource for policy violation %s/%s: %v", pv.Name, pv.Spec.Policy, err)
		}

		_, err = pvInterface.ClusterPolicyViolations().Create(&pv)
		if err != nil {
			glog.Error(err)
			return fmt.Errorf("failed to create cluster policy violation: %v", err)
		}
		glog.Infof("policy violation created for resource %v", pv.Spec.ResourceSpec)
		return nil
	}
	// Update existing PV if there any changes
	if reflect.DeepEqual(pv.Spec, ePV.Spec) {
		glog.V(4).Infof("policy violation spec %v did not change so not updating it", pv.Spec)
		return nil
	}

	pv.SetName(ePV.Name)
	_, err = pvInterface.ClusterPolicyViolations().Update(&pv)
	if err != nil {
		glog.Error(err)
		return fmt.Errorf("failed to update cluster polciy violation: %v", err)
	}
	glog.Infof("policy violation updated for resource %v", pv.Spec.ResourceSpec)
	return nil
}

// build PV without owners
func buildPV(info Info) kyverno.ClusterPolicyViolation {
	pv := buildPVObj(info.PolicyName, kyverno.ResourceSpec{
		Kind: info.Resource.GetKind(),
		Name: info.Resource.GetName(),
	}, info.Rules,
	)
	return pv
}

// build PV object
func buildPVObj(policyName string, resourceSpec kyverno.ResourceSpec, rules []kyverno.ViolatedRule) kyverno.ClusterPolicyViolation {
	pv := kyverno.ClusterPolicyViolation{
		Spec: kyverno.PolicyViolationSpec{
			Policy:        policyName,
			ResourceSpec:  resourceSpec,
			ViolatedRules: rules,
		},
	}

	labelMap := map[string]string{
		"policy":   policyName,
		"resource": resourceSpec.ToKey(),
	}
	pv.SetLabels(labelMap)
	pv.SetGenerateName("pv-")
	return pv
}

// build PV with owners
func buildPVWithOwners(dclient *client.Client, info Info) []kyverno.ClusterPolicyViolation {
	var pvs []kyverno.ClusterPolicyViolation
	// as its blocked resource, the violation is created on owner
	ownerMap := map[kyverno.ResourceSpec]interface{}{}
	GetOwner(dclient, ownerMap, info.Resource)

	// standaloneresource, set pvResourceSpec with resource itself
	if len(ownerMap) == 0 {
		pvResourceSpec := kyverno.ResourceSpec{
			Kind: info.Resource.GetKind(),
			Name: info.Resource.GetName(),
		}
		return append(pvs, buildPVObj(info.PolicyName, pvResourceSpec, info.Rules))
	}

	// Generate owner on all owners
	for owner := range ownerMap {
		pv := buildPVObj(info.PolicyName, owner, info.Rules)
		pvs = append(pvs, pv)
	}
	return pvs
}

func getExistingPVIfAny(pvLister kyvernolister.ClusterPolicyViolationLister, currpv kyverno.ClusterPolicyViolation) (*kyverno.ClusterPolicyViolation, error) {
	pvs, err := pvLister.List(labels.Everything())
	if err != nil {
		glog.Errorf("unable to list policy violations : %v", err)
		return nil, err
	}

	for _, pv := range pvs {
		// find a policy on same resource and policy combination
		if pv.Spec.Policy == currpv.Spec.Policy &&
			pv.Spec.ResourceSpec.Kind == currpv.Spec.ResourceSpec.Kind &&
			pv.Spec.ResourceSpec.Name == currpv.Spec.ResourceSpec.Name {
			return pv, nil
		}
	}
	return nil, nil
}
