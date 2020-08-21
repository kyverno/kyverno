package policy

import (
	"fmt"
	"github.com/nirmata/kyverno/pkg/policyreport"
	"os"
	"reflect"

	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"k8s.io/apimachinery/pkg/labels"
)

func (pc *PolicyController) cleanUp(ers []response.EngineResponse) {
	for _, er := range ers {

		if !er.IsSuccessful() && os.Getenv("POLICY-TYPE") == "POLICYVIOLATION" {
			continue
		}

		if len(er.PolicyResponse.Rules) == 0 {
			continue
		}
		// clean up after the policy has been corrected

		pc.cleanUpPolicyViolation(er)

	}
}

func (pc *PolicyController) cleanUpPolicyViolation(erResponse response.EngineResponse) {
	logger := pc.log
	var er []response.EngineResponse
	er = append(er, erResponse)
	pResponse := erResponse.PolicyResponse
	pvInfo := policyreport.GeneratePRsFromEngineResponse(er, logger)
	if os.Getenv("POLICY-TYPE") == "POLICYREPORT" {
		resource, err := pc.client.GetResource(pResponse.Resource.APIVersion, pResponse.Resource.Kind, pResponse.Resource.Namespace, pResponse.Resource.Name)
		if err != nil {
			logger.Error(err, "failed to get resource")
			return
		}
		labels := resource.GetLabels()
		_, okChart := labels["app"]
		_, okRelease := labels["release"]

		if okChart && okRelease {
			pv := &kyverno.PolicyViolation{}
			pv.Spec.Policy = pResponse.Policy
			pv.Namespace = pResponse.Resource.Namespace
			pv.Name = pResponse.Resource.Name
			appName := fmt.Sprintf("%s-%s", labels["app"], pv.Namespace)
			if err := pc.pvControl.DeleteHelmNamespacedPolicyViolation(pv.Name, pv.Namespace, appName, pvInfo); err != nil {
				logger.Error(err, "failed to delete cluster policy violation", "policy", pResponse.Policy)
			} else {
				logger.Info("deleted cluster policy violation", "policy", pResponse.Policy)
			}
			return
		}
	}else if pResponse.Resource.Namespace == "" {
		if os.Getenv("POLICY-TYPE") == "POLICYREPORT" {
			pv := &kyverno.ClusterPolicyViolation{}
			pv.Spec.Policy = pResponse.Policy
			if err := pc.pvControl.DeleteClusterPolicyViolation(pResponse.Policy, pvInfo); err != nil {
				logger.Error(err, "failed to delete cluster policy violation", "policy", pResponse.Policy)
			} else {
				logger.Info("deleted cluster policy violation", "policy", pResponse.Policy)
			}
			return
		}
		pv, err := getClusterPV(pc.cpvLister, pResponse.Policy, pResponse.Resource.Kind, pResponse.Resource.Name, logger)
		if err != nil {
			logger.Error(err, "failed to get cluster policy violation on policy and resource", "policy", pResponse.Policy, "kind", pResponse.Resource.Kind, "name", pResponse.Resource.Name)
			return
		}

		if reflect.DeepEqual(pv, kyverno.ClusterPolicyViolation{}) {
			return
		}
		if err := pc.pvControl.DeleteClusterPolicyViolation(pResponse.Policy, pvInfo); err != nil {
			logger.Error(err, "failed to delete cluster policy violation", "name", pv.Name)
		} else {
			logger.Info("deleted cluster policy violation", "name", pv.Name)
		}
		return
	}
	if os.Getenv("POLICY-TYPE") == "POLICYREPORT" {
		pv := &kyverno.PolicyViolation{}
		pv.Spec.Policy = pResponse.Policy
		pv.Namespace = pResponse.Resource.Namespace
		pv.Name = pResponse.Resource.Name
		if err := pc.pvControl.DeleteNamespacedPolicyViolation(pv.Name, pv.Namespace, pvInfo); err != nil {
			logger.Error(err, "failed to delete cluster policy violation", "policy", pResponse.Policy)
		} else {
			logger.Info("deleted cluster policy violation", "policy", pResponse.Policy)
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
	if err := pc.pvControl.DeleteNamespacedPolicyViolation(nspv.Name, nspv.Namespace, pvInfo); err != nil {
		logger.Error(err, "failed to delete cluster policy violation", "name", nspv.Name, "namespace", nspv.Namespace)
	} else {
		logger.Info("deleted namespaced policy violation", "name", nspv.Name, "namespace", nspv.Namespace)
	}
	return
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
