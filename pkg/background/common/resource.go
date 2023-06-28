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

	get := func() (*unstructured.Unstructured, error) {
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

	var resource *unstructured.Unstructured
	var err error
	retry := func(_ context.Context) error {
		resource, err = get()
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
