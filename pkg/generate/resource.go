package generate

import (
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func getResource(client *dclient.Client, resourceSpec kyverno.ResourceSpec) (*unstructured.Unstructured, error) {
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
