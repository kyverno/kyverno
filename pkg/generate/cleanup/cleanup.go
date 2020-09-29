package cleanup

import (
	"context"

	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func (c *Controller) processGR(gr kyverno.GenerateRequest) error {
	logger := c.log.WithValues("kind", gr.Kind, "namespace", gr.Namespace, "name", gr.Name)
	// 1- Corresponding policy has been deleted
	// then we dont delete the generated resources

	// 2- The trigger resource is deleted, then delete the generated resources
	if !ownerResourceExists(c.ctx, logger, c.client, gr) {
		if err := deleteGeneratedResources(c.ctx, logger, c.client, gr); err != nil {
			return err
		}
		// - trigger-resource is deleted
		// - generated-resources are deleted
		// - > Now delete the GenerateRequest CR
		return c.control.Delete(gr.Name)
	}
	return nil
}

func ownerResourceExists(ctx context.Context, log logr.Logger, client *dclient.Client, gr kyverno.GenerateRequest) bool {
	_, err := client.GetResource(ctx, "", gr.Spec.Resource.Kind, gr.Spec.Resource.Namespace, gr.Spec.Resource.Name)
	// trigger resources has been deleted
	if apierrors.IsNotFound(err) {
		return false
	}
	if err != nil {
		log.Error(err, "failed to get resource", "genKind", gr.Spec.Resource.Kind, "genNamespace", gr.Spec.Resource.Namespace, "genName", gr.Spec.Resource.Name)
	}
	// if there was an error while querying the resources we dont delete the generated resources
	// but expect the deletion in next reconciliation loop
	return true
}

func deleteGeneratedResources(ctx context.Context, log logr.Logger, client *dclient.Client, gr kyverno.GenerateRequest) error {
	for _, genResource := range gr.Status.GeneratedResources {
		err := client.DeleteResource(ctx, "", genResource.Kind, genResource.Namespace, genResource.Name, false)
		if apierrors.IsNotFound(err) {
			log.Error(err, "resource not foundl will not delete", "genKind", gr.Spec.Resource.Kind, "genNamespace", gr.Spec.Resource.Namespace, "genName", gr.Spec.Resource.Name)
			continue
		}
		if err != nil {
			return err
		}

	}
	return nil
}
