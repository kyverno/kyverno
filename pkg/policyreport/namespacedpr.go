package policyreport

import (
	"fmt"
	kyvernov1 "github.com/nirmata/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"

	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	policyreportv1alpha1 "github.com/nirmata/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha1"
	policyreportlister "github.com/nirmata/kyverno/pkg/client/listers/policyreport/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/policystatus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//namespacedPR ...
type namespacedPR struct {
	// dynamic client
	dclient *client.Client
	// get/list namespaced policy violation
	nsprLister policyreportlister.PolicyV1alpha1Interface
	// policy violation interface
	policyreportInterface policyreportv1alpha1.PolicyReportInterface
	// logger
	log logr.Logger
	// update policy status with violationCount
	policyStatusListener policystatus.Listener
}

func newNamespacedPV(log logr.Logger, dclient *client.Client,
	nsprLister policyreportlister.PolicyReportLister,
	policyreportInterface policyreportv1alpha1.PolicyV1alpha1Interface,
	policyStatus policystatus.Listener,
) *namespacedPR {
	nspr := namespacedPR{
		dclient:              dclient,
		nsprLister:           nsprLister,
		policyreportInterface:     policyreportInterface,
		log:                  log,
		policyStatusListener: policyStatus,
	}
	return &nspr
}

func (nspr *namespacedPR) create(pv kyverno.PolicyViolationTemplate) error {
	policyName := fmt.Sprintf("kyverno-policyreport",)
	clusterpr,err:= nspr.policyreportInterface.Get(context.Background(),policyName,v1.GetOptions{});
	if err != nil {
		return err
	}
	cpr := PolicyViolationsToPolicyReport(&pv,clusterpr)
	cpr,err = nspr.policyreportInterface.Update(context.Background(),cpr,v1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}
