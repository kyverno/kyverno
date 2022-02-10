package cleanup

import (
	"strconv"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func (c *Controller) processGR(gr kyverno.GenerateRequest) error {
	logger := c.log.WithValues("kind", gr.Kind, "namespace", gr.Namespace, "name", gr.Name)
	// 1- Corresponding policy has been deleted
	// then we don't delete the generated resources

	// 2- The trigger resource is deleted, then delete the generated resources
	if !ownerResourceExists(logger, c.client, gr) {
		deleteGR := false
		// check retry count in annotaion
		grAnnotations := gr.Annotations
		if val, ok := grAnnotations["generate.kyverno.io/retry-count"]; ok {
			retryCount, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				logger.Error(err, "unable to convert retry-count")
				return err
			}

			if retryCount >= 5 {
				deleteGR = true
			}
		}

		if deleteGR {
			if err := deleteGeneratedResources(logger, c.client, gr); err != nil {
				return err
			}
			// - trigger-resource is deleted
			// - generated-resources are deleted
			// - > Now delete the GenerateRequest CR
			return c.control.Delete(gr.Name)
		}
	}
	return nil
}

func ownerResourceExists(log logr.Logger, client *dclient.Client, gr kyverno.GenerateRequest) bool {
	_, err := client.GetResource("", gr.Spec.Resource.Kind, gr.Spec.Resource.Namespace, gr.Spec.Resource.Name)
	// trigger resources has been deleted
	if apierrors.IsNotFound(err) {
		return false
	}
	if err != nil {
		log.Error(err, "failed to get resource", "genKind", gr.Spec.Resource.Kind, "genNamespace", gr.Spec.Resource.Namespace, "genName", gr.Spec.Resource.Name)
	}
	// if there was an error while querying the resources we don't delete the generated resources
	// but expect the deletion in next reconciliation loop
	return true
}

func deleteGeneratedResources(log logr.Logger, client *dclient.Client, gr kyverno.GenerateRequest) error {
	for _, genResource := range gr.Status.GeneratedResources {
		err := client.DeleteResource("", genResource.Kind, genResource.Namespace, genResource.Name, false)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}

		log.V(3).Info("generated resource deleted", "genKind", gr.Spec.Resource.Kind, "genNamespace", gr.Spec.Resource.Namespace, "genName", gr.Spec.Resource.Name)
	}
	return nil
}
