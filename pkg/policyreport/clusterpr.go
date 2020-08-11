package policyreport

import (
	"context"

	"github.com/go-logr/logr"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	policyreportv1alpha1 "github.com/nirmata/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha1"
	policyreportlister "github.com/nirmata/kyverno/pkg/client/listers/policyreport/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/policystatus"
)

//clusterPR ...
type clusterPR struct {
	// dynamic client
	dclient *client.Client
	// get/list cluster policy report
	cprLister policyreportlister.ClusterPolicyReportLister
	// policy violation interface
	policyreportInterface policyreportv1alpha1.PolicyV1alpha1Interface
	// logger
	log logr.Logger
	// update policy stats with violationCount
	policyStatusListener policystatus.Listener
}

func newClusterPR(log logr.Logger, dclient *client.Client,
	cprLister policyreportlister.ClusterPolicyReportLister,
	policyreportInterface policyreportv1alpha1.PolicyV1alpha1Interface,
	policyStatus policystatus.Listener,
) *clusterPR {
	cpv := clusterPR{
		dclient:              dclient,
		cprLister:            cprLister,
		policyreportInterface:     policyreportInterface,
		log:                  log,
		policyStatusListener: policyStatus,
	}
	return &cpv
}

func (cpr *clusterPR) create(pv kyverno.PolicyViolationTemplate) error {
	clusterpr,err:= cpr.policyreportInterface.ClusterPolicyReports("").Get(context.Background(),"kyverno-clusterpolicyreport",v1.GetOptions{});
	if err != nil {
		return err
	}
	clusterpr := ClusterPolicyViolationsToClusterPolicyReport(&pv,clusterpr)

	_,err = cpr.policyreportInterface.ClusterPolicyReports("").Update(context.Background(),clusterpr,v1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}
