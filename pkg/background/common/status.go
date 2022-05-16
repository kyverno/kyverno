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
	Failed(ur urkyverno.UpdateRequest, message string, genResources []kyverno.ResourceSpec) error
	Success(ur urkyverno.UpdateRequest, genResources []kyverno.ResourceSpec) error
	Skip(ur urkyverno.UpdateRequest, genResources []kyverno.ResourceSpec) error
}

// StatusControl is default implementaation of GRStatusControlInterface
type StatusControl struct {
	Client kyvernoclient.Interface
}

//Failed sets ur status.state to failed with message
func (sc StatusControl) Failed(ur urkyverno.UpdateRequest, message string, genResources []kyverno.ResourceSpec) error {
	genR := &urkyverno.UpdateRequestStatus{
		State:   urkyverno.Failed,
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
	log.Log.V(3).Info("updated update request status", "name", ur.Name, "status", string(kyverno.Failed))
	return nil
}

// Success sets the ur status.state to completed and clears message
func (sc StatusControl) Success(ur urkyverno.UpdateRequest, genResources []kyverno.ResourceSpec) error {
	genR := &urkyverno.UpdateRequestStatus{
		State:   urkyverno.Completed,
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
	log.Log.V(3).Info("updated update request status", "name", ur.Name, "status", string(urkyverno.Completed))
	return nil
}

// Success sets the ur status.state to completed and clears message
func (sc StatusControl) Skip(ur urkyverno.UpdateRequest, genResources []kyverno.ResourceSpec) error {
	genR := &urkyverno.UpdateRequestStatus{
		State:   urkyverno.Skip,
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
	log.Log.V(3).Info("updated UR status", "name", ur.Name, "status", string(kyverno.Skip))
	return nil
}
