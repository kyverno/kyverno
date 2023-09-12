package adapters

import (
	"context"
	"fmt"
	"io"

	"github.com/kyverno/kyverno/pkg/auth"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (a *dclientAdapter) GetNamespace(ctx context.Context, name string, opts metav1.GetOptions) (*corev1.Namespace, error) {
	return a.client.GetKubeClient().CoreV1().Namespaces().Get(ctx, name, opts)
}

func (a *dclientAdapter) ListResource(ctx context.Context, apiVersion string, kind string, namespace string, lselector *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	return a.client.ListResource(ctx, apiVersion, kind, namespace, lselector)
}

func (a *dclientAdapter) IsNamespaced(group, version, kind string) (bool, error) {
	gvrss, err := a.client.Discovery().FindResources(group, version, kind, "")
	if err != nil {
		return false, err
	}
	if len(gvrss) != 1 {
		return false, fmt.Errorf("function IsNamespaced expects only one resource, got (%d)", len(gvrss))
	}
	for _, apiResource := range gvrss {
		return apiResource.Namespaced, nil
	}
	return false, nil
}

func (a *dclientAdapter) CanI(ctx context.Context, kind, namespace, verb, subresource, user string) (bool, string, error) {
	canI := auth.NewCanI(a.client.Discovery(), a.client.GetKubeClient().AuthorizationV1().SubjectAccessReviews(), kind, namespace, verb, subresource, user)
	ok, reason, err := canI.RunAccessCheck(ctx)
	if err != nil {
		return false, reason, err
	}
	return ok, reason, nil
}
