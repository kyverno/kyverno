package generate

import (
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func getResource(client *dclient.Client, resourceSpec kyverno.ResourceSpec) (*unstructured.Unstructured, error) {
	return client.GetResource(resourceSpec.APIVersion, resourceSpec.Kind, resourceSpec.Namespace, resourceSpec.Name)
}
