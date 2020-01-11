package generate

import (
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
)

type StatusControlInterface interface {
	Failed(gr kyverno.GenerateRequest, message string, genResources []kyverno.ResourceSpec) error
	Success(gr kyverno.GenerateRequest, genResources []kyverno.ResourceSpec) error
}

// StatusControl is default implementaation of GRStatusControlInterface
type StatusControl struct {
	client kyvernoclient.Interface
}

//FailedGR sets gr status.state to failed with message
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

// SuccessGR sets the gr status.state to completed and clears message
func (sc StatusControl) Success(gr kyverno.GenerateRequest, genResources []kyverno.ResourceSpec) error {
	gr.Status.State = kyverno.Completed
	gr.Status.Message = ""
	// Update Generated Resources
	gr.Status.GeneratedResources = genResources

	_, err := sc.client.KyvernoV1().GenerateRequests("kyverno").UpdateStatus(&gr)
	if err != nil {
		glog.V(4).Infof("FAILED: updated gr %s status to %s", gr.Name, string(kyverno.Completed))
		return err
	}
	glog.V(4).Infof("updated gr %s status to %s", gr.Name, string(kyverno.Completed))
	return nil
}
