package cleanup

import (
	"strconv"

	"github.com/go-logr/logr"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func (c *Controller) processUR(ur kyvernov1beta1.UpdateRequest) error {
	logger := c.log.WithValues("kind", ur.Kind, "namespace", ur.Namespace, "name", ur.Name)
	// 1- Corresponding policy has been deleted
	// then we don't delete the generated resources

	// 2- The trigger resource is deleted, then delete the generated resources
	if !ownerResourceExists(logger, c.client, ur) {
		deleteUR := false
		// check retry count in annotaion
		urAnnotations := ur.Annotations
		if val, ok := urAnnotations[kyvernov1beta1.URGenerateRetryCountAnnotation]; ok {
			retryCount, err := strconv.ParseUint(val, 10, 32)
			if err != nil {
				logger.Error(err, "unable to convert retry-count")
				return err
			}

			if retryCount >= 5 {
				deleteUR = true
			}
		}

		if deleteUR {
			if err := deleteGeneratedResources(logger, c.client, ur); err != nil {
				return err
			}
			// - trigger-resource is deleted
			// - generated-resources are deleted
			// - > Now delete the UpdateRequest CR
			return c.control.Delete(ur.Name)
		}
	}
	return nil
}

func ownerResourceExists(log logr.Logger, client dclient.Interface, ur kyvernov1beta1.UpdateRequest) bool {
	_, err := client.GetResource("", ur.Spec.Resource.Kind, ur.Spec.Resource.Namespace, ur.Spec.Resource.Name)
	// trigger resources has been deleted
	if apierrors.IsNotFound(err) {
		return false
	}
	if err != nil {
		log.Error(err, "failed to get resource", "genKind", ur.Spec.Resource.Kind, "genNamespace", ur.Spec.Resource.Namespace, "genName", ur.Spec.Resource.Name)
	}
	// if there was an error while querying the resources we don't delete the generated resources
	// but expect the deletion in next reconciliation loop
	return true
}

func deleteGeneratedResources(log logr.Logger, client dclient.Interface, ur kyvernov1beta1.UpdateRequest) error {
	for _, genResource := range ur.Status.GeneratedResources {
		err := client.DeleteResource("", genResource.Kind, genResource.Namespace, genResource.Name, false)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}

		log.V(3).Info("generated resource deleted", "genKind", ur.Spec.Resource.Kind, "genNamespace", ur.Spec.Resource.Namespace, "genName", ur.Spec.Resource.Name)
	}
	return nil
}
