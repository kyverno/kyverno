package generate

import (
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
)

func getResource(client *dclient.Client, resourceSpec kyverno.ResourceSpec) (*unstructured.Unstructured,error) {
	return client.GetResource(resourceSpec.Kind, resourceSpec.Namespace, resourceSpec.Name)
}
