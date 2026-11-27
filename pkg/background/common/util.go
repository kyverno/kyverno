package common

import (
	"context"

	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/logging"
	errors "github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func UpdateStatus(ctx context.Context, client versioned.Interface, urLister kyvernov2listers.UpdateRequestNamespaceLister, name string, state kyvernov2.UpdateRequestState, message string, genResources []kyverno.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	var latest *kyvernov2.UpdateRequest
	ur, err := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ur, errors.Wrapf(err, "failed to fetch update request")
	}
	latest = ur.DeepCopy()
	latest.Status.State = state
	latest.Status.Message = message
	if genResources != nil {
		latest.Status.GeneratedResources = genResources
	}

	if state == kyvernov2.Failed {
		var deleted bool
		if latest, deleted, err = retryOrDeleteOnFailure(ctx, client, latest, 3); err != nil {
			return nil, err
		} else if deleted {
			// The retry budget is spent and the request is gone, so there is
			// nothing left to carry the status. Skip the write: issuing an
			// UpdateStatus against the deleted object returns a 404 and the
			// terminal cleanup gets reported as a reconciliation error. Keep
			// a single drop record at the default verbosity so a silent drop
			// leaves no ambiguity about the work never completing, but leave
			// the failure message out of it: the message can carry resource
			// data from the failed target expression, and it dies with the
			// deleted object anyway.
			logging.V(2).Info("dropped update request after exhausting retries", "name", name, "retryCount", latest.Status.RetryCount)
			return ur, nil
		}
	}
	new, err := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(ctx, latest, metav1.UpdateOptions{})
	if err != nil {
		return ur, errors.Wrapf(err, "failed to update ur status to %s", string(state))
	}

	logging.V(3).Info("updated update request status", "name", name, "status", string(state), "state", new.Status.State)
	return ur, nil
}

func ResourceSpecFromUnstructured(obj unstructured.Unstructured) kyverno.ResourceSpec {
	return kyverno.ResourceSpec{
		APIVersion: obj.GetAPIVersion(),
		Kind:       obj.GetKind(),
		Namespace:  obj.GetNamespace(),
		Name:       obj.GetName(),
		UID:        obj.GetUID(),
	}
}

func retryOrDeleteOnFailure(ctx context.Context, kyvernoClient versioned.Interface, ur *kyvernov2.UpdateRequest, limit int) (latest *kyvernov2.UpdateRequest, deleted bool, err error) {
	if ur.Status.RetryCount > limit {
		err = kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Delete(ctx, ur.GetName(), metav1.DeleteOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				// Another worker or manual cleanup already removed the
				// request between the fetch and this delete. The terminal
				// state is reached either way, so report the delete as done
				// instead of wrapping the race as an error and requeuing.
				return ur, true, nil
			}
			return nil, false, errors.Wrapf(err, "exceeds retry limit, failed to delete the UR: %s, retry: %v, resourceVersion: %s", ur.Name, ur.Status.RetryCount, ur.GetResourceVersion())
		}
		return ur, true, nil
	}
	ur.Status.RetryCount++
	return ur, false, nil
}
