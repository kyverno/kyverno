package generate

import (
	"context"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func getResource(ctx context.Context, client *dclient.Client, resourceSpec kyverno.ResourceSpec) (*unstructured.Unstructured, error) {
	return client.GetResource(ctx, resourceSpec.APIVersion, resourceSpec.Kind, resourceSpec.Namespace, resourceSpec.Name)
}
