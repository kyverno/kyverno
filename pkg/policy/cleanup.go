package policy

import (
	"fmt"
	"reflect"
	"os"

	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"k8s.io/apimachinery/pkg/labels"
)

func (pc *PolicyController) cleanUp(ers []response.EngineResponse) {
	if os.Getenv("POLICY-TYPE") != "POLICYREPORT" {
		for _, er := range ers {
			if !er.IsSuccessful() {
				continue
			}
			if len(er.PolicyResponse.Rules) == 0 {
				continue
			}
			// clean up after the policy has been corrected
			pc.cleanUpPolicyViolation(er.PolicyResponse)
		}
	}
}

func (pc *PolicyController) cleanUpPolicyViolation(pResponse response.PolicyResponse) {
	logger := pc.log
	// - check if there is violation on resource (label:Selector)
	if pResponse.Resource.Namespace == "" {
		pv, err := getClusterPV(pc.cpvLister, pResponse.Policy, pResponse.Resource.Kind, pResponse.Resource.Name, logger)
		if err != nil {
			logger.Error(err, "failed to get cluster policy violation on policy and resource", "policy", pResponse.Policy, "kind", pResponse.Resource.Kind, "name", pResponse.Resource.Name)
			return
		}

		if reflect.DeepEqual(pv, kyverno.ClusterPolicyViolation{}) {
			return
		}
		if err := pc.pvControl.DeleteClusterPolicyViolation(pv.Name); err != nil {
			logger.Error(err, "failed to delete cluster policy violation", "name", pv.Name)
		} else {
			logger.Info("deleted cluster policy violation", "name", pv.Name)
		}
		return
	}

	// namespace policy violation
	nspv, err := getNamespacedPV(pc.nspvLister, pResponse.Policy, pResponse.Resource.Kind, pResponse.Resource.Namespace, pResponse.Resource.Name, logger)
	if err != nil {
		logger.Error(err, "failed to get namespaced policy violation on policy and resource", "policy", pResponse.Policy, "kind", pResponse.Resource.Kind, "namespace", pResponse.Resource.Namespace, "name", pResponse.Resource.Name)
		return
	}

	if reflect.DeepEqual(nspv, kyverno.PolicyViolation{}) {
		return
	}
	if err := pc.pvControl.DeleteNamespacedPolicyViolation(nspv.Namespace, nspv.Name); err != nil {
		logger.Error(err, "failed to delete cluster policy violation", "name", nspv.Name, "namespace", nspv.Namespace)
	} else {
		logger.Info("deleted namespaced policy violation", "name", nspv.Name, "namespace", nspv.Namespace)
	}
}

// Wont do the claiming of objects, just lookup based on selectors
func getClusterPV(pvLister kyvernolister.ClusterPolicyViolationLister, policyName, rkind, rname string, log logr.Logger) (kyverno.ClusterPolicyViolation, error) {
	var err error
	// Check Violation on resource
	pvs, err := pvLister.List(labels.Everything())
	if err != nil {
		log.Error(err, "failed to list cluster policy violations")
		return kyverno.ClusterPolicyViolation{}, fmt.Errorf("failed to list cluster pv: %v", err)
	}

	for _, pv := range pvs {
		// find a policy on same resource and policy combination
		if pv.Spec.Policy == policyName &&
			pv.Spec.ResourceSpec.Kind == rkind &&
			pv.Spec.ResourceSpec.Name == rname {
			return *pv, nil
		}
	}
	return kyverno.ClusterPolicyViolation{}, nil
}

func getNamespacedPV(nspvLister kyvernolister.PolicyViolationLister, policyName, rkind, rnamespace, rname string, log logr.Logger) (kyverno.PolicyViolation, error) {
	nspvs, err := nspvLister.PolicyViolations(rnamespace).List(labels.Everything())
	if err != nil {
		log.Error(err, "failed to list namespaced policy violation")
		return kyverno.PolicyViolation{}, fmt.Errorf("failed to list namespaced pv: %v", err)
	}

	for _, nspv := range nspvs {
		// find a policy on same resource and policy combination
		if nspv.Spec.Policy == policyName &&
			nspv.Spec.ResourceSpec.Kind == rkind &&
			nspv.Spec.ResourceSpec.Name == rname {
			return *nspv, nil
		}
	}

	return kyverno.PolicyViolation{}, nil
}
