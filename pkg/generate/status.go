package generate

import (
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	"github.com/nirmata/kyverno/pkg/policyStatus"
)

//StatusControlInterface provides interface to update status subresource
type StatusControlInterface interface {
	Failed(gr kyverno.GenerateRequest, message string, genResources []kyverno.ResourceSpec) error
	Success(gr kyverno.GenerateRequest, genResources []kyverno.ResourceSpec) error
}

// StatusControl is default implementaation of GRStatusControlInterface
type StatusControl struct {
	client       kyvernoclient.Interface
	policyStatus *policyStatus.Sync
}

//Failed sets gr status.state to failed with message
func (sc StatusControl) Failed(gr kyverno.GenerateRequest, message string, genResources []kyverno.ResourceSpec) error {
	gr.Status.State = kyverno.Failed
	gr.Status.Message = message
	// Update Generated Resources
	gr.Status.GeneratedResources = genResources
	_, err := sc.client.KyvernoV1().GenerateRequests("kyverno").UpdateStatus(&gr)
	if err != nil {
		glog.V(4).Infof("FAILED: updated gr %s status to %s", gr.Name, string(kyverno.Failed))
		return err
	}
	glog.V(4).Infof("updated gr %s status to %s", gr.Name, string(kyverno.Failed))
	return nil
}

// Success sets the gr status.state to completed and clears message
func (sc StatusControl) Success(gr kyverno.GenerateRequest, genResources []kyverno.ResourceSpec) error {
	gr.Status.State = kyverno.Completed
	gr.Status.Message = ""
	// Update Generated Resources
	gr.Status.GeneratedResources = genResources

	go sc.policyStatus.UpdatePolicyStatusWithGeneratedResourceCount(gr)

	_, err := sc.client.KyvernoV1().GenerateRequests("kyverno").UpdateStatus(&gr)
	if err != nil {
		glog.V(4).Infof("FAILED: updated gr %s status to %s", gr.Name, string(kyverno.Completed))
		return err
	}
	glog.V(4).Infof("updated gr %s status to %s", gr.Name, string(kyverno.Completed))
	return nil
}
