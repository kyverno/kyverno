package common

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetTrigger(client dclient.Interface, spec kyvernov2.UpdateRequestSpec, i int, logger logr.Logger) (*unstructured.Unstructured, error) {
	resourceSpec := spec.RuleContext[i].Trigger
	logger.V(4).Info("fetching trigger", "trigger", resourceSpec.String())
	admissionRequest := spec.Context.AdmissionRequestInfo.AdmissionRequest
	if admissionRequest == nil {
		return GetResource(client, resourceSpec, spec, logger)
	} else {
		operation := spec.Context.AdmissionRequestInfo.Operation
		if operation == admissionv1.Delete {
			return getTriggerForDeleteOperation(client, spec, i, logger)
		} else if operation == admissionv1.Create {
			return getTriggerForCreateOperation(client, spec, i, logger)
		} else {
			newResource, oldResource, err := admissionutils.ExtractResources(nil, *admissionRequest)
			if err != nil {
				logger.Error(err, "failed to extract resources from admission review request")
				return nil, err
			}

			trigger := &newResource
			if newResource.Object == nil {
				trigger = &oldResource
			}
			return trigger, nil
		}
	}
}

func getTriggerForDeleteOperation(client dclient.Interface, spec kyvernov2.UpdateRequestSpec, i int, logger logr.Logger) (*unstructured.Unstructured, error) {
	request := spec.Context.AdmissionRequestInfo.AdmissionRequest
	_, oldResource, err := admissionutils.ExtractResources(nil, *request)
	if err != nil {
		return nil, fmt.Errorf("failed to load resource from context: %w", err)
	}
	labels := oldResource.GetLabels()
	resourceSpec := spec.RuleContext[i].Trigger
	if labels[GeneratePolicyLabel] != "" {
		// non-trigger deletion, get trigger from ur spec
		logger.V(4).Info("non-trigger resource is deleted, fetching the trigger from the UR spec", "trigger", spec.Resource.String())
		return GetResource(client, resourceSpec, spec, logger)
	}
	return &oldResource, nil
}

func getTriggerForCreateOperation(client dclient.Interface, spec kyvernov2.UpdateRequestSpec, i int, logger logr.Logger) (*unstructured.Unstructured, error) {
	admissionRequest := spec.Context.AdmissionRequestInfo.AdmissionRequest
	resourceSpec := spec.RuleContext[i].Trigger
	trigger, err := GetResource(client, resourceSpec, spec, logger)
	if err != nil || trigger == nil {
		if admissionRequest.SubResource == "" {
			return nil, err
		} else {
			logger.V(4).Info("trigger resource not found for subresource, reverting to resource in AdmissionReviewRequest", "subresource", admissionRequest.SubResource)
			newResource, _, err := admissionutils.ExtractResources(nil, *admissionRequest)
			if err != nil {
				logger.Error(err, "failed to extract resources from admission review request")
				return nil, err
			}
			return &newResource, nil
		}
	}
	return trigger, err
}

func GetResource(client dclient.Interface, resourceSpec kyvernov1.ResourceSpec, urSpec kyvernov2.UpdateRequestSpec, log logr.Logger) (resource *unstructured.Unstructured, err error) {
	obj := resourceSpec
	if reflect.DeepEqual(obj, kyvernov1.ResourceSpec{}) {
		obj = urSpec.GetResource()
	}

	fmt.Println("Fetching resource", "resourceSpec", resourceSpec.String())
	fmt.Println("uid: ", obj.GetUID(), "name:", obj.GetName(), "namespace:", obj.GetNamespace(), "kind:", resourceSpec.GetKind(), "apiVersion:", resourceSpec.GetAPIVersion())
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
