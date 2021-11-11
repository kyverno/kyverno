package generate

import (
	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

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
