package common

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetResource(client dclient.Interface, resourceSpec kyvernov1.ResourceSpec, urSpec kyvernov2.UpdateRequestSpec, log logr.Logger) (resource *unstructured.Unstructured, err error) {
	obj := resourceSpec
	if reflect.DeepEqual(obj, kyvernov1.ResourceSpec{}) {
		obj = urSpec.GetResource()
	}

	if obj.GetUID() != "" {
		triggers, err := client.ListResource(context.TODO(), resourceSpec.GetAPIVersion(), resourceSpec.GetKind(), resourceSpec.GetNamespace(), nil)
		if err != nil {
			return nil, fmt.Errorf("failed to list trigger resources: %v", err)
		}

		for _, trigger := range triggers.Items {
			if resourceSpec.GetUID() == trigger.GetUID() {
				return &trigger, nil
			}
		}
	} else if obj.GetName() != "" {
		if resourceSpec.Kind == "Namespace" {
			resourceSpec.Namespace = ""
		}
		resource, err := client.GetResource(context.TODO(), resourceSpec.APIVersion, resourceSpec.Kind, resourceSpec.Namespace, resourceSpec.Name)
		if err != nil {
			if urSpec.GetRequestType() == kyvernov2.Mutate && errors.IsNotFound(err) && urSpec.Context.AdmissionRequestInfo.Operation == admissionv1.Delete {
				log.V(4).Info("trigger resource does not exist for mutateExisting rule", "operation", urSpec.Context.AdmissionRequestInfo.Operation)
				return nil, nil
			}

			return nil, fmt.Errorf("resource %s/%s/%s/%s: %v", resourceSpec.APIVersion, resourceSpec.Kind, resourceSpec.Namespace, resourceSpec.Name, err)
		}

		return resource, nil
	}

	if urSpec.Context.AdmissionRequestInfo.AdmissionRequest != nil {
		request := urSpec.Context.AdmissionRequestInfo.AdmissionRequest
		raw := request.Object.Raw
		if request.Operation == admissionv1.Delete {
			raw = request.OldObject.Raw
		}

		resource, err = kubeutils.BytesToUnstructured(raw)
		if err != nil {
			return nil, fmt.Errorf("failed to convert raw object to unstructured: %v", err)
		} else {
			return resource, nil
		}
	}

	return nil, fmt.Errorf("resource not found")
}
