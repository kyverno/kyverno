package cluster

import (
	"context"
	"io"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	eventsv1 "k8s.io/client-go/kubernetes/typed/events/v1"
)

// Overwrite write actions to dry run
// prevents performing actual operations
type Client struct {
	inner dclient.Interface
	fake  dclient.Interface
}

func (c *Client) GetKubeClient() kubernetes.Interface {
	return c.inner.GetKubeClient()
}

func (c *Client) GetEventsInterface() eventsv1.EventsV1Interface {
	return c.inner.GetEventsInterface()
}

func (c *Client) GetDynamicInterface() dynamic.Interface {
	return c.inner.GetDynamicInterface()
}

func (c *Client) Discovery() dclient.IDiscovery {
	return c.inner.Discovery()
}

func (c *Client) SetDiscovery(discoveryClient dclient.IDiscovery) {
	c.inner.SetDiscovery(discoveryClient)
}

func (c *Client) RawAbsPath(ctx context.Context, path string, method string, dataReader io.Reader) ([]byte, error) {
	if method == "" {
		method = "GET"
	}

	return c.inner.RawAbsPath(ctx, path, method, dataReader)
}

func (c *Client) GetResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, subresources ...string) (*unstructured.Unstructured, error) {
	if res, _ := c.fake.GetResource(ctx, apiVersion, kind, namespace, name, subresources...); res != nil {
		return res, nil
	}

	return c.inner.GetResource(ctx, apiVersion, kind, namespace, name, subresources...)
}

func (c *Client) PatchResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, patch []byte) (*unstructured.Unstructured, error) {
	return c.fake.PatchResource(ctx, apiVersion, kind, namespace, name, patch)
}

func (c *Client) ListResource(ctx context.Context, apiVersion string, kind string, namespace string, lselector *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	return c.inner.ListResource(ctx, apiVersion, kind, namespace, lselector)
}

func (c *Client) DeleteResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, dryRun bool, options metav1.DeleteOptions) error {
	return c.fake.DeleteResource(ctx, apiVersion, kind, namespace, name, dryRun, options)
}

func (c *Client) CreateResource(ctx context.Context, apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	return c.fake.CreateResource(ctx, apiVersion, kind, namespace, obj, dryRun)
}

func (c *Client) UpdateResource(ctx context.Context, apiVersion string, kind string, namespace string, obj interface{}, dryRun bool, subresources ...string) (*unstructured.Unstructured, error) {
	return c.fake.UpdateResource(ctx, apiVersion, kind, namespace, obj, dryRun, subresources...)
}

func (c *Client) UpdateStatusResource(ctx context.Context, apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	return c.fake.UpdateStatusResource(ctx, apiVersion, kind, namespace, obj, dryRun)
}

func (c *Client) ApplyResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, obj interface{}, dryRun bool, fieldManager string, subresources ...string) (*unstructured.Unstructured, error) {
	return c.fake.ApplyResource(ctx, apiVersion, kind, namespace, name, obj, dryRun, fieldManager, subresources...)
}

func (c *Client) ApplyStatusResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, obj interface{}, dryRun bool, fieldManager string) (*unstructured.Unstructured, error) {
	return c.fake.ApplyStatusResource(ctx, apiVersion, kind, namespace, name, obj, dryRun, fieldManager)
}

func NewWrapper(client dclient.Interface) dclient.Interface {
	return &Client{
		inner: client,
		fake:  dclient.NewEmptyFakeClient(),
	}
}
