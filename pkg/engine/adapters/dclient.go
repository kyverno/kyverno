package adapters

import (
	"context"
	"io"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

type dclientAdapter struct {
	client dclient.Interface
}

func (a *dclientAdapter) RawAbsPath(ctx context.Context, path string, method string, dataReader io.Reader) ([]byte, error) {
	return a.client.RawAbsPath(ctx, path, method, dataReader)
}

func (a *dclientAdapter) GetResources(group, version, kind, subresource, namespace, name string) ([]engineapi.Resource, error) {
	resources, err := dclient.GetResources(a.client, group, version, kind, subresource, namespace, name)
	if err != nil {
		return nil, err
	}
	var result []engineapi.Resource
	for _, resource := range resources {
		result = append(result, engineapi.Resource{
			Group:        resource.Group,
			Version:      resource.Version,
			Resource:     resource.Resource,
			SubResource:  resource.SubResource,
			Unstructured: resource.Unstructured,
		})
	}
	return result, nil
}

func ClientInterface(client dclient.Interface) engineapi.ClientInterface {
	return &dclientAdapter{client}
}
