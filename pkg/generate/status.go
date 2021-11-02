package generate

import (
	"context"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/config"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

//StatusControlInterface provides interface to update status subresource
type StatusControlInterface interface {
	Failed(gr kyverno.GenerateRequest, message string, genResources []kyverno.ResourceSpec) error
	Success(gr kyverno.GenerateRequest, genResources []kyverno.ResourceSpec) error
	Skip(gr kyverno.GenerateRequest, genResources []kyverno.ResourceSpec) error
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
	_, err := sc.client.KyvernoV1().GenerateRequests(config.KyvernoNamespace).UpdateStatus(context.TODO(), &gr, v1.UpdateOptions{})
	if err != nil && !errors.IsNotFound(err) {
		log.Log.Error(err, "failed to update generate request status", "name", gr.Name)
		return err
	}
	log.Log.V(3).Info("updated generate request status", "name", gr.Name, "status", string(kyverno.Failed))
	return nil
}

// Success sets the gr status.state to completed and clears message
func (sc StatusControl) Success(gr kyverno.GenerateRequest, genResources []kyverno.ResourceSpec) error {
	gr.Status.State = kyverno.Completed
	gr.Status.Message = ""
	// Update Generated Resources
	gr.Status.GeneratedResources = genResources

	_, err := sc.client.KyvernoV1().GenerateRequests(config.KyvernoNamespace).UpdateStatus(context.TODO(), &gr, v1.UpdateOptions{})
	if err != nil && !errors.IsNotFound(err) {
		log.Log.Error(err, "failed to update generate request status", "name", gr.Name)
		return err
	}

	log.Log.V(3).Info("updated generate request status", "name", gr.Name, "status", string(kyverno.Completed))
	return nil
}

// Success sets the gr status.state to completed and clears message
func (sc StatusControl) Skip(gr kyverno.GenerateRequest, genResources []kyverno.ResourceSpec) error {
	gr.Status.State = kyverno.Skip
	gr.Status.Message = ""
	// Update Generated Resources
	gr.Status.GeneratedResources = genResources

	_, err := sc.client.KyvernoV1().GenerateRequests(config.KyvernoNamespace).UpdateStatus(context.TODO(), &gr, v1.UpdateOptions{})
	if err != nil && !errors.IsNotFound(err) {
		log.Log.Error(err, "failed to update generate request status", "name", gr.Name)
		return err
	}

	log.Log.V(3).Info("updated generate request status", "name", gr.Name, "status", string(kyverno.Skip))
	return nil
}
