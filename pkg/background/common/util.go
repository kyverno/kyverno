package common

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/logging"
	errors "github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func UpdateStatus(client versioned.Interface, urLister kyvernov1beta1listers.UpdateRequestNamespaceLister, name string, state kyvernov1beta1.UpdateRequestState, message string, genResources []kyvernov1.ResourceSpec) (*kyvernov1beta1.UpdateRequest, error) {
	var latest *kyvernov1beta1.UpdateRequest
	ur, err := client.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return ur, errors.Wrapf(err, "failed to fetch update request")
	}
	latest = ur.DeepCopy()
	latest.Status.State = state
	latest.Status.Message = message
	if genResources != nil {
		latest.Status.GeneratedResources = genResources
	}

	if state == kyvernov1beta1.Failed {
		if latest, err = retryOrDeleteOnFailure(client, latest, 3); err != nil {
			return nil, err
		}
	}
	new, err := client.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), latest, metav1.UpdateOptions{})
	if err != nil {
		return ur, errors.Wrapf(err, "failed to update ur status to %s", string(state))
	}

	logging.V(3).Info("updated update request status", "name", name, "status", string(state), "state", new.Status.State)
	return ur, nil
}

func PolicyKey(namespace, name string) string {
	if namespace != "" {
		return namespace + "/" + name
	}
	return name
}

func ResourceSpecFromUnstructured(obj unstructured.Unstructured) kyvernov1.ResourceSpec {
	return kyvernov1.ResourceSpec{
		APIVersion: obj.GetAPIVersion(),
		Kind:       obj.GetKind(),
		Namespace:  obj.GetNamespace(),
		Name:       obj.GetName(),
		UID:        obj.GetUID(),
	}
}

func retryOrDeleteOnFailure(kyvernoClient versioned.Interface, ur *kyvernov1beta1.UpdateRequest, limit int) (latest *kyvernov1beta1.UpdateRequest, err error) {
	if ur.Status.RetryCount > limit {
		err = kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Delete(context.TODO(), ur.GetName(), metav1.DeleteOptions{})
		if err != nil {
			return nil, errors.Wrapf(err, "exceeds retry limit, failed to delete the UR: %s, retry: %v, resourceVersion: %s", ur.Name, ur.Status.RetryCount, ur.GetResourceVersion())
		}
	} else {
		ur.Status.RetryCount++
	}

	return ur, nil
}

func FindDownstream(client dclient.Interface, apiVersion, kind string, labels map[string]string) (*unstructured.UnstructuredList, error) {
	selector := &metav1.LabelSelector{MatchLabels: labels}
	return client.ListResource(context.TODO(), apiVersion, kind, "", selector)
}
