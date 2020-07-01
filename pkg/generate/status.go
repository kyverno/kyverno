package generate

import (
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	"github.com/nirmata/kyverno/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

//StatusControlInterface provides interface to update status subresource
type StatusControlInterface interface {
	Failed(gr kyverno.GenerateRequest, message string, genResources []kyverno.ResourceSpec) error
	Success(gr kyverno.GenerateRequest, genResources []kyverno.ResourceSpec) error
}

// StatusControl is default implementaation of GRStatusControlInterface
type StatusControl struct {
	client kyvernoclient.Interface
}

//Failed sets gr status.state to failed with message
func (sc StatusControl) Failed(gr kyverno.GenerateRequest, message string, genResources []kyverno.ResourceSpec) error {
	gr.Status.State = kyverno.Failed
	gr.Status.Message = message
	// Update Generated Resources
	gr.Status.GeneratedResources = genResources
	_, err := sc.client.KyvernoV1().GenerateRequests(config.KubePolicyNamespace).UpdateStatus(&gr)
	if err != nil {
		log.Log.Error(err, "failed to update generate request status", "name", gr.Name)
		return err
	}
	log.Log.Info("updated generate request status", "name", gr.Name, "status", string(kyverno.Failed))
	return nil
}

// Success sets the gr status.state to completed and clears message
func (sc StatusControl) Success(gr kyverno.GenerateRequest, genResources []kyverno.ResourceSpec) error {
	gr.Status.State = kyverno.Completed
	gr.Status.Message = ""
	// Update Generated Resources
	gr.Status.GeneratedResources = genResources

	_, err := sc.client.KyvernoV1().GenerateRequests(config.KubePolicyNamespace).UpdateStatus(&gr)
	if err != nil {
		log.Log.Error(err, "failed to update generate request status", "name", gr.Name)
		return err
	}
	log.Log.Info("updated generate request status", "name", gr.Name, "status", string(kyverno.Completed))
	return nil
}
