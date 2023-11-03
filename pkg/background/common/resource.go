package common

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	retryutils "github.com/kyverno/kyverno/pkg/utils/retry"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetResource(client dclient.Interface, urSpec kyvernov1beta1.UpdateRequestSpec, log logr.Logger) (*unstructured.Unstructured, error) {
	resourceSpec := urSpec.GetResource()

	getByName := func() (*unstructured.Unstructured, error) {
		if resourceSpec.Kind == "Namespace" {
			resourceSpec.Namespace = ""
		}
		resource, err := client.GetResource(context.TODO(), resourceSpec.APIVersion, resourceSpec.Kind, resourceSpec.Namespace, resourceSpec.Name)
		if err != nil {
			if urSpec.GetRequestType() == kyvernov1beta1.Mutate && errors.IsNotFound(err) && urSpec.Context.AdmissionRequestInfo.Operation == admissionv1.Delete {
				log.V(4).Info("trigger resource does not exist for mutateExisting rule", "operation", urSpec.Context.AdmissionRequestInfo.Operation)
				return nil, nil
			}

			return nil, fmt.Errorf("resource %s/%s/%s/%s: %v", resourceSpec.APIVersion, resourceSpec.Kind, resourceSpec.Namespace, resourceSpec.Name, err)
		}

		return resource, nil
	}

	getByUID := func() (*unstructured.Unstructured, error) {
		gv, err := resourceSpec.GetGroupVersion()
		if err != nil {
			return nil, err
		}

		// fetch targets that have the source UID label
		triggerSelector := map[string]string{
			GenerateTriggerGroupLabel:   gv.Group,
			GenerateTriggerVersionLabel: gv.Version,
			GenerateTriggerKindLabel:    resourceSpec.GetKind(),
			GenerateTriggerNSLabel:      resourceSpec.GetNamespace(),
			GenerateTriggerUIDLabel:     string(resourceSpec.GetUID()),
		}
		triggers, err := FindDownstream(client, resourceSpec.GetAPIVersion(), resourceSpec.GetKind(), triggerSelector)
		if err != nil {
			return nil, fmt.Errorf("failed to list trigger resources: %v", err)
		}

		if len(triggers.Items) == 0 {
			return nil, fmt.Errorf("no trigger resource found for %s", resourceSpec.String())
		}
		return &triggers.Items[0], nil
	}

	var resource *unstructured.Unstructured
	var err error
	retry := func(_ context.Context) error {
		if resourceSpec.GetUID() != "" {
			resource, err = getByUID()
		} else {
			resource, err = getByName()
		}
		return err
	}

	f := retryutils.RetryFunc(context.TODO(), time.Second, 5*time.Second, log.WithName("getResource"), "failed to get resource", retry)
	if err := f(); err != nil {
		return nil, err
	}

	if resource == nil && urSpec.Context.AdmissionRequestInfo.AdmissionRequest != nil {
		request := urSpec.Context.AdmissionRequestInfo.AdmissionRequest
		raw := request.Object.Raw
		if request.Operation == admissionv1.Delete {
			raw = request.OldObject.Raw
		}

		resource, err = kubeutils.BytesToUnstructured(raw)
	}

	log.V(3).Info("fetched trigger resource", "resourceSpec", resourceSpec)
	return resource, err
}
