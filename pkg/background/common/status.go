package common

import (
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

//StatusControlInterface provides interface to update status subresource
type StatusControlInterface interface {
	Failed(gr urkyverno.UpdateRequest, message string, genResources []kyverno.ResourceSpec) error
	Success(gr urkyverno.UpdateRequest, genResources []kyverno.ResourceSpec) error
	Skip(gr urkyverno.UpdateRequest, genResources []kyverno.ResourceSpec) error
}

// StatusControl is default implementaation of GRStatusControlInterface
type StatusControl struct {
	Client kyvernoclient.Interface
}

//Failed sets gr status.state to failed with message
func (sc StatusControl) Failed(gr urkyverno.UpdateRequest, message string, genResources []kyverno.ResourceSpec) error {
	patch := jsonutils.NewPatch(
		"/status",
		"replace",
		&urkyverno.UpdateRequestStatus{
			State:              urkyverno.Failed,
			Message:            message,
			GeneratedResources: genResources, // Update Generated Resources
		},
	)
	_, err := PatchGenerateRequest(&gr, patch, sc.Client, "status")
	if err != nil && !errors.IsNotFound(err) {
		log.Log.Error(err, "failed to patch generate request status", "name", gr.Name)
		return err
	}
	log.Log.V(3).Info("updated generate request status", "name", gr.Name, "status", string(kyverno.Failed))
	return nil
}

// Success sets the gr status.state to completed and clears message
func (sc StatusControl) Success(gr urkyverno.UpdateRequest, genResources []kyverno.ResourceSpec) error {
	patch := jsonutils.NewPatch(
		"/status",
		"replace",
		&kyverno.GenerateRequestStatus{
			State:              kyverno.Completed,
			Message:            "",
			GeneratedResources: genResources, // Update Generated Resources
		},
	)
	_, err := PatchGenerateRequest(&gr, patch, sc.Client, "status")
	if err != nil && !errors.IsNotFound(err) {
		log.Log.Error(err, "failed to patch generate request status", "name", gr.Name)
		return err
	}
	log.Log.V(3).Info("updated generate request status", "name", gr.Name, "status", string(kyverno.Completed))
	return nil
}

// Success sets the gr status.state to completed and clears message
func (sc StatusControl) Skip(gr urkyverno.UpdateRequest, genResources []kyverno.ResourceSpec) error {
	patch := jsonutils.NewPatch(
		"/status",
		"replace",
		&urkyverno.UpdateRequestStatus{
			State:              urkyverno.Skip,
			Message:            "",
			GeneratedResources: genResources, // Update Generated Resources
		},
	)
	_, err := PatchGenerateRequest(&gr, patch, sc.Client, "status")
	if err != nil && !errors.IsNotFound(err) {
		log.Log.Error(err, "failed to patch generate request status", "name", gr.Name)
		return err
	}
	log.Log.V(3).Info("updated generate request status", "name", gr.Name, "status", string(kyverno.Skip))
	return nil
}
