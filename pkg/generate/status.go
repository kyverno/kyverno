package generate

import (
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
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
	patch := []PatchOp{
		{
			Op:   "replace",
			Path: "/status",
			Value: &kyverno.GenerateRequestStatus{
				State:              kyverno.Failed,
				Message:            message,
				GeneratedResources: genResources, // Update Generated Resources
			},
		},
	}

	_, err := PatchGenerateRequest(&gr, patch, sc.client, "status")
	if err != nil && !errors.IsNotFound(err) {
		log.Log.Error(err, "failed to patch generate request status", "name", gr.Name)
		return err
	}

	log.Log.V(3).Info("updated generate request status", "name", gr.Name, "status", string(kyverno.Failed))
	return nil
}

// Success sets the gr status.state to completed and clears message
func (sc StatusControl) Success(gr kyverno.GenerateRequest, genResources []kyverno.ResourceSpec) error {
	patch := []PatchOp{
		{
			Op:   "replace",
			Path: "/status",
			Value: &kyverno.GenerateRequestStatus{
				State:              kyverno.Completed,
				Message:            "",
				GeneratedResources: genResources, // Update Generated Resources
			},
		},
	}

	_, err := PatchGenerateRequest(&gr, patch, sc.client, "status")
	if err != nil && !errors.IsNotFound(err) {
		log.Log.Error(err, "failed to patch generate request status", "name", gr.Name)
		return err
	}

	log.Log.V(3).Info("updated generate request status", "name", gr.Name, "status", string(kyverno.Completed))
	return nil
}

// Success sets the gr status.state to completed and clears message
func (sc StatusControl) Skip(gr kyverno.GenerateRequest, genResources []kyverno.ResourceSpec) error {
	patch := []PatchOp{
		{
			Op:   "replace",
			Path: "/status",
			Value: &kyverno.GenerateRequestStatus{
				State:              kyverno.Skip,
				Message:            "",
				GeneratedResources: genResources, // Update Generated Resources
			},
		},
	}

	_, err := PatchGenerateRequest(&gr, patch, sc.client, "status")
	if err != nil && !errors.IsNotFound(err) {
		log.Log.Error(err, "failed to patch generate request status", "name", gr.Name)
		return err
	}

	log.Log.V(3).Info("updated generate request status", "name", gr.Name, "status", string(kyverno.Skip))
	return nil
}
