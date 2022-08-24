package cleanup

import (
	"github.com/go-logr/logr"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/dclient"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

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
