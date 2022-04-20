package generate

import (
	"time"

	logr "github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/common"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func getResource(client *dclient.Client, resourceSpec kyverno.ResourceSpec, log logr.Logger) (*unstructured.Unstructured, error) {

	get := func() (*unstructured.Unstructured, error) {
		if resourceSpec.Kind == "Namespace" {
			resourceSpec.Namespace = ""
		}
		resource, err := client.GetResource(resourceSpec.APIVersion, resourceSpec.Kind, resourceSpec.Namespace, resourceSpec.Name)
		if err != nil {
			return nil, err
		}

		if resource.GetDeletionTimestamp() != nil {
			return nil, nil
		}

		return resource, nil
	}

	retry := func() error {
		_, err := get()
		return err
	}

	f := common.RetryFunc(time.Second, 30*time.Second, retry, log.WithName("getResource"))
	if err := f(); err != nil {
		return nil, err
	}

	return get()
}
