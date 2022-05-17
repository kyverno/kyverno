package common

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// StatusControlInterface provides interface to update status subresource
type StatusControlInterface interface {
	Failed(ur kyvernov1beta1.UpdateRequest, message string, genResources []kyvernov1.ResourceSpec) error
	Success(ur kyvernov1beta1.UpdateRequest, genResources []kyvernov1.ResourceSpec) error
	Skip(ur kyvernov1beta1.UpdateRequest, genResources []kyvernov1.ResourceSpec) error
}

// StatusControl is default implementaation of GRStatusControlInterface
type StatusControl struct {
	Client kyvernoclient.Interface
}

// Failed sets ur status.state to failed with message
func (sc StatusControl) Failed(ur kyvernov1beta1.UpdateRequest, message string, genResources []kyvernov1.ResourceSpec) error {
	genR := &kyvernov1beta1.UpdateRequestStatus{
		State:   kyvernov1beta1.Failed,
		Message: message,
	}
	if genResources != nil {
		genR.GeneratedResources = genResources
	}

	patch := jsonutils.NewPatch(
		"/status",
		"replace",
		genR,
	)
	_, err := PatchUpdateRequest(&ur, patch, sc.Client, "status")
	if err != nil && !errors.IsNotFound(err) {
		log.Log.Error(err, "failed to patch update request status", "name", ur.Name)
		return err
	}
	log.Log.V(3).Info("updated update request status", "name", ur.Name, "status", string(kyvernov1.Failed))
	return nil
}

// Success sets the ur status.state to completed and clears message
func (sc StatusControl) Success(ur kyvernov1beta1.UpdateRequest, genResources []kyvernov1.ResourceSpec) error {
	genR := &kyvernov1beta1.UpdateRequestStatus{
		State:   kyvernov1beta1.Completed,
		Message: "",
	}

	if genResources != nil {
		genR.GeneratedResources = genResources
	}

	patch := jsonutils.NewPatch(
		"/status",
		"replace",
		genR,
	)
	_, err := PatchUpdateRequest(&ur, patch, sc.Client, "status")
	if err != nil && !errors.IsNotFound(err) {
		log.Log.Error(err, "failed to patch update request status", "name", ur.Name)
		return err
	}
	log.Log.V(3).Info("updated update request status", "name", ur.Name, "status", string(kyvernov1beta1.Completed))
	return nil
}

// Success sets the ur status.state to completed and clears message
func (sc StatusControl) Skip(ur kyvernov1beta1.UpdateRequest, genResources []kyvernov1.ResourceSpec) error {
	genR := &kyvernov1beta1.UpdateRequestStatus{
		State:   kyvernov1beta1.Skip,
		Message: "",
	}

	if genResources != nil {
		genR.GeneratedResources = genResources
	}

	patch := jsonutils.NewPatch(
		"/status",
		"replace",
		genR,
	)
	_, err := PatchUpdateRequest(&ur, patch, sc.Client, "status")
	if err != nil && !errors.IsNotFound(err) {
		log.Log.Error(err, "failed to update UR status", "name", ur.Name)
		return err
	}
	log.Log.V(3).Info("updated UR status", "name", ur.Name, "status", string(kyvernov1.Skip))
	return nil
}
