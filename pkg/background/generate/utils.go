package generate

import (
	"context"
	"fmt"
	"strconv"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func newResourceSpec(genAPIVersion, genKind, genNamespace, genName string) kyvernov1.ResourceSpec {
	return kyvernov1.ResourceSpec{
		APIVersion: genAPIVersion,
		Kind:       genKind,
		Namespace:  genNamespace,
		Name:       genName,
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

func updateRetryAnnotation(kyvernoClient versioned.Interface, ur *kyvernov1beta1.UpdateRequest) error {
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

func TriggerFromLabels(labels map[string]string) kyvernov1.ResourceSpec {
	group := labels[common.GenerateTriggerGroupLabel]
	version := labels[common.GenerateTriggerVersionLabel]
	apiVersion := schema.GroupVersion{Group: group, Version: version}

	return kyvernov1.ResourceSpec{
		Kind:       labels[common.GenerateTriggerKindLabel],
		Namespace:  labels[common.GenerateTriggerNSLabel],
		Name:       labels[common.GenerateTriggerNameLabel],
		APIVersion: apiVersion.String(),
	}
}

func FindDownstream(client dclient.Interface, apiVersion, kind string, labels map[string]string) (*unstructured.UnstructuredList, error) {
	selector := &metav1.LabelSelector{MatchLabels: labels}
	return client.ListResource(context.TODO(), apiVersion, kind, "", selector)
}
