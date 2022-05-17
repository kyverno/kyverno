package background

import (
	"context"
	"strconv"

	"github.com/go-logr/logr"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/background/mutate"
	"github.com/kyverno/kyverno/pkg/config"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Controller) ProcessUR(ur *kyvernov1beta1.UpdateRequest) error {
	switch ur.Spec.Type {
	case kyvernov1beta1.Mutate:
		ctrl, _ := mutate.NewMutateExistingController(c.kyvernoClient, c.client,
			c.policyLister, c.npolicyLister, c.urLister, c.eventGen, c.log, c.Config)
		return ctrl.ProcessUR(ur)

	case kyvernov1beta1.Generate:
		ctrl, _ := generate.NewGenerateController(c.kyvernoClient, c.client,
			c.policyLister, c.npolicyLister, c.urLister, c.eventGen, c.nsLister, c.log, c.Config,
		)
		return ctrl.ProcessUR(ur)
	}
	return nil
}

func (c *Controller) MarkUR(ur *kyvernov1beta1.UpdateRequest) (*kyvernov1beta1.UpdateRequest, bool, error) {
	handler := ur.Status.Handler
	if handler != "" {
		if handler != config.KyvernoPodName() {
			return nil, false, nil
		}
		return ur, true, nil
	}

	handler = config.KyvernoPodName()
	ur.Status.Handler = handler
	new, err := c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), ur, metav1.UpdateOptions{})
	return new, true, err
}

func (c *Controller) UnmarkUR(ur *kyvernov1beta1.UpdateRequest) error {
	newUR, err := c.urLister.Get(ur.Name)
	if err != nil {
		return err
	}

	if ur.Spec.Type == kyvernov1beta1.Mutate && ur.Status.State == kyvernov1beta1.Completed {
		return c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Delete(context.TODO(), ur.GetName(), metav1.DeleteOptions{})
	}

	newUR.Status.Handler = ""
	_, err = c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), newUR, metav1.UpdateOptions{})
	return err
}

func (c *Controller) HandleDeleteUR(ur kyvernov1beta1.UpdateRequest) error {
	logger := c.log.WithValues("kind", ur.Kind, "namespace", ur.Namespace, "name", ur.Name)
	// 1- Corresponding policy has been deleted
	// then we don't delete the generated resources

	// 2- The trigger resource is deleted, then delete the generated resources
	if !ownerResourceExists(logger, c.client, ur) {
		deleteUR := false
		// check retry count in annotaion
		urAnnotations := ur.Annotations
		if val, ok := urAnnotations["generate.kyverno.io/retry-count"]; ok {
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
			return c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Delete(context.TODO(), ur.Name, metav1.DeleteOptions{})
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
