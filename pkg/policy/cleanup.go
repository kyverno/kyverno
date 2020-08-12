package policy

import (
	"fmt"
	"github.com/nirmata/kyverno/pkg/api/policyreport/v1alpha1"
	policylister "github.com/nirmata/kyverno/pkg/client/listers/policyreport/v1alpha1"
	"reflect"

	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"k8s.io/apimachinery/pkg/labels"
)

func (pc *PolicyController) cleanUp(ers []response.EngineResponse) {
	for _, er := range ers {
		if !er.IsSuccessful() {
			continue
		}
		if len(er.PolicyResponse.Rules) == 0 {
			continue
		}
		// clean up after the policy has been corrected
		pc.cleanUpKyvernoPolicyReport(er.PolicyResponse)
	}
}

func (pc *PolicyController) cleanUpKyvernoPolicyReport(pResponse response.PolicyResponse) {
	logger := pc.log
	// - check if there is violation on resource (label:Selector)
	resource,err := pc.client.GetResource(pResponse.Resource.APIVersion,pResponse.Resource.Kind,pResponse.Resource.Namespace,pResponse.Resource.Name)
	if err != nil {
		logger.Error(err, "failed to get resource", "policy", pResponse.Policy, "kind", pResponse.Resource.Kind, "name", pResponse.Resource.Name)
		return
	}
	labels := resource.GetLabels()
	if _,ok := labels[""]; !ok {
		// namespace policy violation
		nspr, err := getHelmPR(pc.nsprLister, pResponse.Policy, pResponse.Resource.Kind, pResponse.Resource.Namespace, pResponse.Resource.Name, logger)
		if err != nil {
			logger.Error(err, "failed to get namespaced policy violation on policy and resource", "policy", pResponse.Policy, "kind", pResponse.Resource.Kind, "namespace", pResponse.Resource.Namespace, "name", pResponse.Resource.Name)
			return
		}

		if reflect.DeepEqual(nspr, v1alpha1.PolicyReport{}) {
			return
		}
		if err := pc.prControl.DeleteNamespacedKyvernoPolicyReport(nspr.Namespace, nspr.Name); err != nil {
			logger.Error(err, "failed to delete cluster policy violation", "name", nspr.Name, "namespace", nspr.Namespace)
		} else {
			logger.Info("deleted namespaced policy violation", "name", nspr.Name, "namespace", nspr.Namespace)
		}
	}else if pResponse.Resource.Namespace == "" {
		pr, err := getClusterPR(pc.cprLister, pResponse.Policy, pResponse.Resource.Kind, pResponse.Resource.Name, logger)
		if err != nil {
			logger.Error(err, "failed to get cluster policy violation on policy and resource", "policy", pResponse.Policy, "kind", pResponse.Resource.Kind, "name", pResponse.Resource.Name)
			return
		}

		if reflect.DeepEqual(pr, v1alpha1.ClusterPolicyReport{}) {
			return
		}
		if err := pc.prControl.DeleteClusterKyvernoPolicyReport(pr.Name); err != nil {
			logger.Error(err, "failed to delete cluster policy violation", "name", pr.Name)
		} else {
			logger.Info("deleted cluster policy violation", "name", pr.Name)
		}
		return
	}

	// namespace policy violation
	nspr, err := getNamespacedPR(pc.nsprLister, pResponse.Policy, pResponse.Resource.Kind, pResponse.Resource.Namespace, pResponse.Resource.Name, logger)
	if err != nil {
		logger.Error(err, "failed to get namespaced policy violation on policy and resource", "policy", pResponse.Policy, "kind", pResponse.Resource.Kind, "namespace", pResponse.Resource.Namespace, "name", pResponse.Resource.Name)
		return
	}

	if reflect.DeepEqual(nspr, v1alpha1.PolicyReport{}) {
		return
	}
	if err := pc.prControl.DeleteNamespacedKyvernoPolicyReport(nspr.Namespace, nspr.Name); err != nil {
		logger.Error(err, "failed to delete cluster policy violation", "name", nspr.Name, "namespace", nspr.Namespace)
	} else {
		logger.Info("deleted namespaced policy violation", "name", nspr.Name, "namespace", nspr.Namespace)
	}
}

// Wont do the claiming of objects, just lookup based on selectors
func getClusterPR(prLister policylister.ClusterPolicyReportLister, policyName, rkind, rname string, log logr.Logger) (v1alpha1.ClusterPolicyReport, error) {
	var err error
	// Check Violation on resource
	prs, err := prLister.List(labels.Everything())
	if err != nil {
		log.Error(err, "failed to list cluster policy violations")
		return v1alpha1.ClusterPolicyReport{}, fmt.Errorf("failed to list cluster pv: %v", err)
	}

	for _, pr := range prs {
		// find a policy on same resource and policy combination
		if pr.Policy == policyName &&
			pr.Spec.ResourceSpec.Kind == rkind &&
			pr.Spec.ResourceSpec.Name == rname {
			return *pr, nil
		}
	}
	return v1alpha1.ClusterPolicyReport{}, nil
}

func getNamespacedPR(nsprLister policylister.PolicyReportLister, policyName, rkind, rnamespace, rname string, log logr.Logger) (v1alpha1.PolicyReport, error) {
	nsprs, err := nsprLister.PolicyReports(rnamespace).List(labels.Everything())
	if err != nil {
		log.Error(err, "failed to list namespaced policy violation")
		return v1alpha1.PolicyReport{}, fmt.Errorf("failed to list namespaced pv: %v", err)
	}

	for _, nspr := range nsprs {
		// find a policy on same resource and policy combination
		if nspr.Spec.Policy == policyName &&
			nspr.Spec.ResourceSpec.Kind == rkind &&
			nspr.Spec.ResourceSpec.Name == rname {
			return *nspr, nil
		}
	}

	return v1alpha1.PolicyReport{}, nil
}

func getHelmPR(nsprLister policylister.PolicyReportLister, policyName, rkind, rnamespace, rname string, log logr.Logger) (v1alpha1.PolicyReport, error) {
	nsprs, err := nsprLister.PolicyReports(rnamespace).List(labels.Everything())
	if err != nil {
		log.Error(err, "failed to list namespaced policy violation")
		return v1alpha1.PolicyReport{}, fmt.Errorf("failed to list namespaced pv: %v", err)
	}

	for _, nspr := range nsprs {
		// find a policy on same resource and policy combination
		if nspr.Spec.Policy == policyName &&
			nspr.Spec.ResourceSpec.Kind == rkind &&
			nspr.Spec.ResourceSpec.Name == rname {
			return *nspr, nil
		}
	}

	return v1alpha1.PolicyReport{}, nil
}
