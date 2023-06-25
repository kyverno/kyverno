package adapters

import (
	"context"
	"io"

	"github.com/kyverno/kyverno/pkg/auth"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type dclientAdapter struct {
	client dclient.Interface
}

func Client(client dclient.Interface) engineapi.Client {
	return &dclientAdapter{client}
}

func (a *dclientAdapter) RawAbsPath(ctx context.Context, path, method string, dataReader io.Reader) ([]byte, error) {
	return a.client.RawAbsPath(ctx, path, method, dataReader)
}

func (a *dclientAdapter) GetResources(ctx context.Context, group, version, kind, subresource, namespace, name string) ([]engineapi.Resource, error) {
	resources, err := dclient.GetResources(ctx, a.client, group, version, kind, subresource, namespace, name)
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

func (a *dclientAdapter) GetResource(ctx context.Context, apiVersion, kind, namespace, name string, subresources ...string) (*unstructured.Unstructured, error) {
	return a.client.GetResource(ctx, apiVersion, kind, namespace, name, subresources...)
}

func (a *dclientAdapter) CanI(ctx context.Context, kind, namespace, verb, subresource, user string) (bool, error) {
	canI := auth.NewCanI(a.client.Discovery(), a.client.GetKubeClient().AuthorizationV1().SubjectAccessReviews(), kind, namespace, verb, subresource, user)
	ok, err := canI.RunAccessCheck(ctx)
	if err != nil {
		return false, err
	}
	return ok, nil
}
