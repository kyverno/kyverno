package resource

import (
	github_com_google_gnostic_openapiv2 "github.com/google/gnostic/openapiv2"
	"github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_apimachinery_pkg_apis_meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_io_apimachinery_pkg_version "k8s.io/apimachinery/pkg/version"
	k8s_io_client_go_discovery "k8s.io/client-go/discovery"
	k8s_io_client_go_openapi "k8s.io/client-go/openapi"
	k8s_io_client_go_rest "k8s.io/client-go/rest"
)

func WithMetrics(inner k8s_io_client_go_discovery.DiscoveryInterface, recorder metrics.Recorder) k8s_io_client_go_discovery.DiscoveryInterface {
	return &withMetrics{inner, recorder}
}

func WithTracing(inner k8s_io_client_go_discovery.DiscoveryInterface, client, kind string) k8s_io_client_go_discovery.DiscoveryInterface {
	return &withTracing{inner, client, kind}
}

type withMetrics struct {
	inner    k8s_io_client_go_discovery.DiscoveryInterface
	recorder metrics.Recorder
}

func (c *withMetrics) OpenAPISchema() (*github_com_google_gnostic_openapiv2.Document, error) {
	defer c.recorder.Record("open_api_schema")
	return c.inner.OpenAPISchema()
}
func (c *withMetrics) OpenAPIV3() k8s_io_client_go_openapi.Client {
	defer c.recorder.Record("open_apiv3")
	return c.inner.OpenAPIV3()
}
func (c *withMetrics) RESTClient() k8s_io_client_go_rest.Interface {
	defer c.recorder.Record("rest_client")
	return c.inner.RESTClient()
}
func (c *withMetrics) ServerGroups() (*k8s_io_apimachinery_pkg_apis_meta_v1.APIGroupList, error) {
	defer c.recorder.Record("server_groups")
	return c.inner.ServerGroups()
}
func (c *withMetrics) ServerGroupsAndResources() ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIGroup, []*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	defer c.recorder.Record("server_groups_and_resources")
	return c.inner.ServerGroupsAndResources()
}
func (c *withMetrics) ServerPreferredNamespacedResources() ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	defer c.recorder.Record("server_preferred_namespaced_resources")
	return c.inner.ServerPreferredNamespacedResources()
}
func (c *withMetrics) ServerPreferredResources() ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	defer c.recorder.Record("server_preferred_resources")
	return c.inner.ServerPreferredResources()
}
func (c *withMetrics) ServerResourcesForGroupVersion(arg0 string) (*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	defer c.recorder.Record("server_resources_for_group_version")
	return c.inner.ServerResourcesForGroupVersion(arg0)
}
func (c *withMetrics) ServerVersion() (*k8s_io_apimachinery_pkg_version.Info, error) {
	defer c.recorder.Record("server_version")
	return c.inner.ServerVersion()
}

type withTracing struct {
	inner  k8s_io_client_go_discovery.DiscoveryInterface
	client string
	kind   string
}

func (c *withTracing) OpenAPISchema() (*github_com_google_gnostic_openapiv2.Document, error) {
	ret0, ret1 := c.inner.OpenAPISchema()
	return ret0, ret1
}
func (c *withTracing) OpenAPIV3() k8s_io_client_go_openapi.Client {
	ret0 := c.inner.OpenAPIV3()
	return ret0
}
func (c *withTracing) RESTClient() k8s_io_client_go_rest.Interface {
	ret0 := c.inner.RESTClient()
	return ret0
}
func (c *withTracing) ServerGroups() (*k8s_io_apimachinery_pkg_apis_meta_v1.APIGroupList, error) {
	ret0, ret1 := c.inner.ServerGroups()
	return ret0, ret1
}
func (c *withTracing) ServerGroupsAndResources() ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIGroup, []*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	ret0, ret1, ret2 := c.inner.ServerGroupsAndResources()
	return ret0, ret1, ret2
}
func (c *withTracing) ServerPreferredNamespacedResources() ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	ret0, ret1 := c.inner.ServerPreferredNamespacedResources()
	return ret0, ret1
}
func (c *withTracing) ServerPreferredResources() ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	ret0, ret1 := c.inner.ServerPreferredResources()
	return ret0, ret1
}
func (c *withTracing) ServerResourcesForGroupVersion(arg0 string) (*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	ret0, ret1 := c.inner.ServerResourcesForGroupVersion(arg0)
	return ret0, ret1
}
func (c *withTracing) ServerVersion() (*k8s_io_apimachinery_pkg_version.Info, error) {
	ret0, ret1 := c.inner.ServerVersion()
	return ret0, ret1
}
