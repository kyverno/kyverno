package resource

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	github_com_google_gnostic_openapiv2 "github.com/google/gnostic/openapiv2"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/multierr"
	k8s_io_apimachinery_pkg_apis_meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_io_apimachinery_pkg_version "k8s.io/apimachinery/pkg/version"
	k8s_io_client_go_discovery "k8s.io/client-go/discovery"
	k8s_io_client_go_openapi "k8s.io/client-go/openapi"
	k8s_io_client_go_rest "k8s.io/client-go/rest"
)

func WithLogging(inner k8s_io_client_go_discovery.DiscoveryInterface, logger logr.Logger) k8s_io_client_go_discovery.DiscoveryInterface {
	return &withLogging{inner, logger}
}

func WithMetrics(inner k8s_io_client_go_discovery.DiscoveryInterface, recorder metrics.Recorder) k8s_io_client_go_discovery.DiscoveryInterface {
	return &withMetrics{inner, recorder}
}

func WithTracing(inner k8s_io_client_go_discovery.DiscoveryInterface, client, kind string) k8s_io_client_go_discovery.DiscoveryInterface {
	return &withTracing{inner, client, kind}
}

type withLogging struct {
	inner  k8s_io_client_go_discovery.DiscoveryInterface
	logger logr.Logger
}

func (c *withLogging) OpenAPISchema() (*github_com_google_gnostic_openapiv2.Document, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "OpenAPISchema")
	ret0, ret1 := c.inner.OpenAPISchema()
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "OpenAPISchema failed", "duration", time.Since(start))
	} else {
		logger.Info("OpenAPISchema done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) OpenAPIV3() k8s_io_client_go_openapi.Client {
	start := time.Now()
	logger := c.logger.WithValues("operation", "OpenAPIV3")
	ret0 := c.inner.OpenAPIV3()
	logger.Info("OpenAPIV3 done", "duration", time.Since(start))
	return ret0
}
func (c *withLogging) RESTClient() k8s_io_client_go_rest.Interface {
	start := time.Now()
	logger := c.logger.WithValues("operation", "RESTClient")
	ret0 := c.inner.RESTClient()
	logger.Info("RESTClient done", "duration", time.Since(start))
	return ret0
}
func (c *withLogging) ServerGroups() (*k8s_io_apimachinery_pkg_apis_meta_v1.APIGroupList, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "ServerGroups")
	ret0, ret1 := c.inner.ServerGroups()
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "ServerGroups failed", "duration", time.Since(start))
	} else {
		logger.Info("ServerGroups done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) ServerGroupsAndResources() ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIGroup, []*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "ServerGroupsAndResources")
	ret0, ret1, ret2 := c.inner.ServerGroupsAndResources()
	if err := multierr.Combine(ret2); err != nil {
		logger.Error(err, "ServerGroupsAndResources failed", "duration", time.Since(start))
	} else {
		logger.Info("ServerGroupsAndResources done", "duration", time.Since(start))
	}
	return ret0, ret1, ret2
}
func (c *withLogging) ServerPreferredNamespacedResources() ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "ServerPreferredNamespacedResources")
	ret0, ret1 := c.inner.ServerPreferredNamespacedResources()
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "ServerPreferredNamespacedResources failed", "duration", time.Since(start))
	} else {
		logger.Info("ServerPreferredNamespacedResources done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) ServerPreferredResources() ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "ServerPreferredResources")
	ret0, ret1 := c.inner.ServerPreferredResources()
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "ServerPreferredResources failed", "duration", time.Since(start))
	} else {
		logger.Info("ServerPreferredResources done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) ServerResourcesForGroupVersion(arg0 string) (*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "ServerResourcesForGroupVersion")
	ret0, ret1 := c.inner.ServerResourcesForGroupVersion(arg0)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "ServerResourcesForGroupVersion failed", "duration", time.Since(start))
	} else {
		logger.Info("ServerResourcesForGroupVersion done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) ServerVersion() (*k8s_io_apimachinery_pkg_version.Info, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "ServerVersion")
	ret0, ret1 := c.inner.ServerVersion()
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "ServerVersion failed", "duration", time.Since(start))
	} else {
		logger.Info("ServerVersion done", "duration", time.Since(start))
	}
	return ret0, ret1
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
	_, span := tracing.StartSpan(
		context.TODO(),
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "OpenAPISchema"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "OpenAPISchema"),
	)
	defer span.End()
	ret0, ret1 := c.inner.OpenAPISchema()
	if ret1 != nil {
		span.RecordError(ret1)
		span.SetStatus(codes.Error, ret1.Error())
	}
	return ret0, ret1
}
func (c *withTracing) OpenAPIV3() k8s_io_client_go_openapi.Client {
	_, span := tracing.StartSpan(
		context.TODO(),
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "OpenAPIV3"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "OpenAPIV3"),
	)
	defer span.End()
	ret0 := c.inner.OpenAPIV3()
	return ret0
}
func (c *withTracing) RESTClient() k8s_io_client_go_rest.Interface {
	_, span := tracing.StartSpan(
		context.TODO(),
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "RESTClient"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "RESTClient"),
	)
	defer span.End()
	ret0 := c.inner.RESTClient()
	return ret0
}
func (c *withTracing) ServerGroups() (*k8s_io_apimachinery_pkg_apis_meta_v1.APIGroupList, error) {
	_, span := tracing.StartSpan(
		context.TODO(),
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "ServerGroups"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "ServerGroups"),
	)
	defer span.End()
	ret0, ret1 := c.inner.ServerGroups()
	if ret1 != nil {
		span.RecordError(ret1)
		span.SetStatus(codes.Error, ret1.Error())
	}
	return ret0, ret1
}
func (c *withTracing) ServerGroupsAndResources() ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIGroup, []*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	_, span := tracing.StartSpan(
		context.TODO(),
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "ServerGroupsAndResources"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "ServerGroupsAndResources"),
	)
	defer span.End()
	ret0, ret1, ret2 := c.inner.ServerGroupsAndResources()
	if ret2 != nil {
		span.RecordError(ret2)
		span.SetStatus(codes.Error, ret2.Error())
	}
	return ret0, ret1, ret2
}
func (c *withTracing) ServerPreferredNamespacedResources() ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	_, span := tracing.StartSpan(
		context.TODO(),
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "ServerPreferredNamespacedResources"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "ServerPreferredNamespacedResources"),
	)
	defer span.End()
	ret0, ret1 := c.inner.ServerPreferredNamespacedResources()
	if ret1 != nil {
		span.RecordError(ret1)
		span.SetStatus(codes.Error, ret1.Error())
	}
	return ret0, ret1
}
func (c *withTracing) ServerPreferredResources() ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	_, span := tracing.StartSpan(
		context.TODO(),
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "ServerPreferredResources"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "ServerPreferredResources"),
	)
	defer span.End()
	ret0, ret1 := c.inner.ServerPreferredResources()
	if ret1 != nil {
		span.RecordError(ret1)
		span.SetStatus(codes.Error, ret1.Error())
	}
	return ret0, ret1
}
func (c *withTracing) ServerResourcesForGroupVersion(arg0 string) (*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	_, span := tracing.StartSpan(
		context.TODO(),
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "ServerResourcesForGroupVersion"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "ServerResourcesForGroupVersion"),
	)
	defer span.End()
	ret0, ret1 := c.inner.ServerResourcesForGroupVersion(arg0)
	if ret1 != nil {
		span.RecordError(ret1)
		span.SetStatus(codes.Error, ret1.Error())
	}
	return ret0, ret1
}
func (c *withTracing) ServerVersion() (*k8s_io_apimachinery_pkg_version.Info, error) {
	_, span := tracing.StartSpan(
		context.TODO(),
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "ServerVersion"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "ServerVersion"),
	)
	defer span.End()
	ret0, ret1 := c.inner.ServerVersion()
	if ret1 != nil {
		span.RecordError(ret1)
		span.SetStatus(codes.Error, ret1.Error())
	}
	return ret0, ret1
}
