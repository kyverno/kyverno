package common

import (
	"context"
	"fmt"
	"strconv"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
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
	}
}

func increaseRetryAnnotation(ur *kyvernov1beta1.UpdateRequest) (int, map[string]string, error) {
	urAnnotations := ur.Annotations
	if len(urAnnotations) == 0 {
		urAnnotations = map[string]string{
			kyvernov1beta1.URGenerateRetryCountAnnotation: "1",
		}
	}

	retry := 1
	val, ok := urAnnotations[kyvernov1beta1.URGenerateRetryCountAnnotation]
	if !ok {
		urAnnotations[kyvernov1beta1.URGenerateRetryCountAnnotation] = "1"
	} else {
		retryUint, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return retry, urAnnotations, fmt.Errorf("unable to convert retry-count %v: %w", val, err)
		}
		retry = int(retryUint)
		retry += 1
		incrementedRetryString := strconv.Itoa(retry)
		urAnnotations[kyvernov1beta1.URGenerateRetryCountAnnotation] = incrementedRetryString
	}

	return retry, urAnnotations, nil
}

func UpdateRetryAnnotation(kyvernoClient versioned.Interface, ur *kyvernov1beta1.UpdateRequest) error {
	retry, urAnnotations, err := increaseRetryAnnotation(ur)
	if err != nil {
		return err
	}
	if retry > 3 {
		err = kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Delete(context.TODO(), ur.GetName(), metav1.DeleteOptions{})
		if err != nil {
			return errors.Wrapf(err, "exceeds retry limit, failed to delete the UR: %s, retry: %v, resourceVersion: %s", ur.Name, retry, ur.GetResourceVersion())
		}
	} else {
		ur.SetAnnotations(urAnnotations)
		_, err = kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Update(context.TODO(), ur, metav1.UpdateOptions{})
		if err != nil {
			return errors.Wrapf(err, "failed to update annotation in update request: %s for the resource, retry: %v, resourceVersion %s, annotations: %v", ur.Name, retry, ur.GetResourceVersion(), urAnnotations)
		}
	}
	return nil
}
