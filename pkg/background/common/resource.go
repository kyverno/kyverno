package common

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetResource(client dclient.Interface, urSpec kyvernov1beta1.UpdateRequestSpec, log logr.Logger) (resource *unstructured.Unstructured, err error) {
	resourceSpec := urSpec.GetResource()

	if urSpec.GetResource().GetUID() != "" {
		triggers, err := client.ListResource(context.TODO(), resourceSpec.GetAPIVersion(), resourceSpec.GetKind(), resourceSpec.GetNamespace(), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to list trigger resources: %v", err)
		}

		for _, trigger := range triggers.Items {
			if resourceSpec.GetUID() == trigger.GetUID() {
				return &trigger, nil
			}
		}
	} else if urSpec.GetResource().GetName() != "" {
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
