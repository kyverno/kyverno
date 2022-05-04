package common

import (
	"fmt"
	"time"

	logr "github.com/go-logr/logr"
	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/common"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	v1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func GetResource(client dclient.Interface, urSpec urkyverno.UpdateRequestSpec, log logr.Logger) (*unstructured.Unstructured, error) {
	resourceSpec := urSpec.Resource

	get := func() (*unstructured.Unstructured, error) {
		if resourceSpec.Kind == "Namespace" {
			resourceSpec.Namespace = ""
		}
		resource, err := client.GetResource(resourceSpec.APIVersion, resourceSpec.Kind, resourceSpec.Namespace, resourceSpec.Name)
		if err != nil {
			if urSpec.Type == urkyverno.Mutate && errors.IsNotFound(err) && urSpec.Context.AdmissionRequestInfo.Operation == v1.Delete {
				log.V(4).Info("trigger resource does not exist for mutateExisting rule", "operation", urSpec.Context.AdmissionRequestInfo.Operation)
				return nil, nil
			}

			return nil, fmt.Errorf("resource %s/%s/%s/%s: %v", resourceSpec.APIVersion, resourceSpec.Kind, resourceSpec.Namespace, resourceSpec.Name, err)
		}

		if resource.GetDeletionTimestamp() != nil {
			log.V(4).Info("trigger resource is in termination", "operation", urSpec.Context.AdmissionRequestInfo.Operation)
			return nil, nil
		}

		return resource, nil
	}

	var resource *unstructured.Unstructured
	var err error
	retry := func() error {
		resource, err = get()
		return err
	}

	f := common.RetryFunc(time.Second, 5*time.Second, retry, "failed to get resource", log.WithName("getResource"))
	if err := f(); err != nil {
		return nil, err
	}

	log.Info("fetched trigger resource", "resourceSpec", resourceSpec)
	return resource, err
}
