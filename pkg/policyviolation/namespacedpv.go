package policyviolation

import (
	"fmt"
	"reflect"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernov1 "github.com/nirmata/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	labels "k8s.io/apimachinery/pkg/labels"
)

func (gen *Generator) createNamespacedPV(info Info) error {
	// namespaced policy violations
	var pvs []kyverno.PolicyViolation
	if !info.Blocked {
		pvs = append(pvs, buildNamespacedPV(info))
	} else {
		pvs = buildNamespacedPVWithOwner(gen.dclient, info)
	}

	if err := createNamespacedPV(info.Resource.GetNamespace(), gen.dclient, gen.nspvLister, gen.pvInterface, pvs); err != nil {
		return err
	}

	glog.V(3).Infof("Created namespaced policy violation policy=%s, resource=%s/%s/%s",
		info.PolicyName, info.Resource.GetKind(), info.Resource.GetNamespace(), info.Resource.GetName())
	return nil
}

func buildNamespacedPV(info Info) kyverno.PolicyViolation {
	return buildNamespacedPVObj(info.PolicyName,
		kyverno.ResourceSpec{
			Kind: info.Resource.GetKind(),
			Name: info.Resource.GetName(),
		},
		info.Rules)
}

//buildNamespacedPVObj returns an value of type PolicyViolation
func buildNamespacedPVObj(policy string, resource kyverno.ResourceSpec, fRules []kyverno.ViolatedRule) kyverno.PolicyViolation {
	pv := kyverno.PolicyViolation{
		Spec: kyverno.PolicyViolationSpec{
			Policy:        policy,
			ResourceSpec:  resource,
			ViolatedRules: fRules,
		},
	}

	labelMap := map[string]string{
		"policy":   policy,
		"resource": resource.ToKey(),
	}
	pv.SetGenerateName(fmt.Sprintf("%s-", policy))
	pv.SetLabels(labelMap)
	return pv
}

func buildNamespacedPVWithOwner(dclient *dclient.Client, info Info) (pvs []kyverno.PolicyViolation) {
	// create violation on resource owner (if exist) when action is set to enforce
	ownerMap := map[kyverno.ResourceSpec]interface{}{}
	GetOwner(dclient, ownerMap, info.Resource)

	// standaloneresource, set pvResourceSpec with resource itself
	if len(ownerMap) == 0 {
		pvResourceSpec := kyverno.ResourceSpec{
			Kind: info.Resource.GetKind(),
			Name: info.Resource.GetName(),
		}
		return append(pvs, buildNamespacedPVObj(info.PolicyName, pvResourceSpec, info.Rules))
	}

	for owner := range ownerMap {
		pvs = append(pvs, buildNamespacedPVObj(info.PolicyName, owner, info.Rules))
	}
	return
}

func createNamespacedPV(namespace string, dclient *dclient.Client, pvLister kyvernolister.PolicyViolationLister, pvInterface kyvernov1.KyvernoV1Interface, pvs []kyverno.PolicyViolation) error {
	for _, newPv := range pvs {
		glog.V(4).Infof("creating namespaced policyViolation resource for policy %s and resource %s", newPv.Spec.Policy, newPv.Spec.ResourceSpec.ToKey())
		// check if there was a previous policy voilation for policy & resource combination
		curPv, err := getExistingNamespacedPVIfAny(pvLister, newPv)
		if err != nil {
			return fmt.Errorf("failed to get existing namespaced pv on resource '%s': %v", newPv.Spec.ResourceSpec.ToKey(), err)
		}

		if reflect.DeepEqual(curPv, kyverno.PolicyViolation{}) {
			// no existing policy violation, create a new one
			if reflect.DeepEqual(curPv, kyverno.PolicyViolation{}) {
				glog.V(4).Infof("creating new namespaced policy violation for policy %s & resource %s", newPv.Spec.Policy, newPv.Spec.ResourceSpec.ToKey())

				if err := retryGetResource(namespace, dclient, newPv.Spec.ResourceSpec); err != nil {
					return fmt.Errorf("failed to get resource for policy violation on resource '%s': %v", newPv.Spec.ResourceSpec.ToKey(), err)
				}

				if _, err := pvInterface.PolicyViolations(namespace).Create(&newPv); err != nil {
					return fmt.Errorf("failed to create namespaced policy violation: %v", err)
				}

				glog.Infof("namespaced policy violation created for resource %s", newPv.Spec.ResourceSpec.ToKey())
			}
			return nil
		}

		// compare the policyviolation spec for existing resource if present else
		if reflect.DeepEqual(curPv.Spec, newPv.Spec) {
			// if they are equal there has been no change so dont update the polivy violation
			glog.V(3).Infof("namespaced policy violation '%s' spec did not change so not updating it", newPv.Spec.ToKey())
			glog.V(4).Infof("namespaced policy violation spec %v did not change so not updating it", newPv.Spec)
			continue
		}

		// set newPv name with curPv, as we are updating the resource itself
		newPv.SetName(curPv.Name)

		// spec changed so update the policyviolation
		glog.V(4).Infof("creating new policy violation for policy %s & resource %s", curPv.Spec.Policy, curPv.Spec.ResourceSpec.ToKey())
		//TODO: using a generic name, but would it be helpful to have naming convention for policy violations
		// as we can only have one policy violation for each (policy + resource) combination
		if _, err = pvInterface.PolicyViolations(namespace).Update(&newPv); err != nil {
			return fmt.Errorf("failed to update namespaced policy violation: %v", err)
		}
		glog.Infof("namespaced policy violation updated for resource %s", newPv.Spec.ResourceSpec.ToKey())
	}
	return nil
}

func getExistingNamespacedPVIfAny(nspvLister kyvernolister.PolicyViolationLister, newPv kyverno.PolicyViolation) (kyverno.PolicyViolation, error) {
	// TODO(shuting): list pvs by labels
	pvs, err := nspvLister.PolicyViolations(newPv.GetNamespace()).List(labels.NewSelector())
	if err != nil {
		return kyverno.PolicyViolation{}, fmt.Errorf("failed to list namespaced policy violations err: %v", err)
	}

	for _, pv := range pvs {
		if pv.Spec.Policy == newPv.Spec.Policy && reflect.DeepEqual(pv.Spec.ResourceSpec, newPv.Spec.ResourceSpec) {
			return *pv, nil
		}
	}

	return kyverno.PolicyViolation{}, nil
}
