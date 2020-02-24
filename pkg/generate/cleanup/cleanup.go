package cleanup

import (
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func (c *Controller) processGR(gr kyverno.GenerateRequest) error {
	// 1- Corresponding policy has been deleted
	// then we dont delete the generated resources

	// 2- The trigger resource is deleted, then delete the generated resources
	if !ownerResourceExists(c.client, gr) {
		if err := deleteGeneratedResources(c.client, gr); err != nil {
			return err
		}
		// - trigger-resource is deleted
		// - generated-resources are deleted
		// - > Now delete the GenerateRequest CR
		return c.control.Delete(gr.Name)
	}
	return nil
}

func ownerResourceExists(client *dclient.Client, gr kyverno.GenerateRequest) bool {
	_, err := client.GetResource(gr.Spec.Resource.Kind, gr.Spec.Resource.Namespace, gr.Spec.Resource.Name)
	// trigger resources has been deleted
	if apierrors.IsNotFound(err) {
		return false
	}
	if err != nil {
		glog.V(4).Infof("Failed to get resource %s/%s/%s: error : %s", gr.Spec.Resource.Kind, gr.Spec.Resource.Namespace, gr.Spec.Resource.Name, err)
	}
	// if there was an error while querying the resources we dont delete the generated resources
	// but expect the deletion in next reconciliation loop
	return true
}

func deleteGeneratedResources(client *dclient.Client, gr kyverno.GenerateRequest) error {
	for _, genResource := range gr.Status.GeneratedResources {
		err := client.DeleteResource(genResource.Kind, genResource.Namespace, genResource.Name, false)
		if apierrors.IsNotFound(err) {
			glog.V(4).Infof("resource %s/%s/%s not found, will no delete", genResource.Kind, genResource.Namespace, genResource.Name)
			continue
		}
		if err != nil {
			return err
		}

	}
	return nil
}
