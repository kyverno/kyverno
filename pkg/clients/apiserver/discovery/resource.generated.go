package resource

import (
	context "context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	github_com_google_gnostic_models_openapiv2 "github.com/google/gnostic-models/openapiv2"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/multierr"
	k8s_io_apimachinery_pkg_apis_meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_io_apimachinery_pkg_version "k8s.io/apimachinery/pkg/version"
	k8s_io_client_go_discovery "k8s.io/client-go/discovery"
	k8s_io_client_go_openapi "k8s.io/client-go/openapi"
	k8s_io_client_go_rest "k8s.io/client-go/rest"
)

func WithLogging(inner k8s_io_client_go_discovery.DiscoveryInterfaces, logger logr.Logger) k8s_io_client_go_discovery.DiscoveryInterfaces {
	return &withLogging{inner, logger}
}

func WithMetrics(inner k8s_io_client_go_discovery.DiscoveryInterfaces, recorder metrics.Recorder) k8s_io_client_go_discovery.DiscoveryInterfaces {
	return &withMetrics{inner, recorder}
}

func WithTracing(inner k8s_io_client_go_discovery.DiscoveryInterfaces, client, kind string) k8s_io_client_go_discovery.DiscoveryInterfaces {
	return &withTracing{inner, client, kind}
}

type withLogging struct {
	inner  k8s_io_client_go_discovery.DiscoveryInterfaces
	logger logr.Logger
}

func (c *withLogging) OpenAPISchema() (*github_com_google_gnostic_models_openapiv2.Document, error) {
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
func (c *withLogging) OpenAPISchemaWithContext(arg0 context.Context) (*github_com_google_gnostic_models_openapiv2.Document, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "OpenAPISchemaWithContext")
	ret0, ret1 := c.inner.OpenAPISchemaWithContext(arg0)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "OpenAPISchemaWithContext failed", "duration", time.Since(start))
	} else {
		logger.Info("OpenAPISchemaWithContext done", "duration", time.Since(start))
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
func (c *withLogging) OpenAPIV3WithContext(arg0 context.Context) k8s_io_client_go_openapi.ClientWithContext {
	start := time.Now()
	logger := c.logger.WithValues("operation", "OpenAPIV3WithContext")
	ret0 := c.inner.OpenAPIV3WithContext(arg0)
	logger.Info("OpenAPIV3WithContext done", "duration", time.Since(start))
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
func (c *withLogging) ServerGroupsAndResourcesWithContext(arg0 context.Context) ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIGroup, []*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "ServerGroupsAndResourcesWithContext")
	ret0, ret1, ret2 := c.inner.ServerGroupsAndResourcesWithContext(arg0)
	if err := multierr.Combine(ret2); err != nil {
		logger.Error(err, "ServerGroupsAndResourcesWithContext failed", "duration", time.Since(start))
	} else {
		logger.Info("ServerGroupsAndResourcesWithContext done", "duration", time.Since(start))
	}
	return ret0, ret1, ret2
}
func (c *withLogging) ServerGroupsWithContext(arg0 context.Context) (*k8s_io_apimachinery_pkg_apis_meta_v1.APIGroupList, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "ServerGroupsWithContext")
	ret0, ret1 := c.inner.ServerGroupsWithContext(arg0)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "ServerGroupsWithContext failed", "duration", time.Since(start))
	} else {
		logger.Info("ServerGroupsWithContext done", "duration", time.Since(start))
	}
	return ret0, ret1
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
func (c *withLogging) ServerPreferredNamespacedResourcesWithContext(arg0 context.Context) ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "ServerPreferredNamespacedResourcesWithContext")
	ret0, ret1 := c.inner.ServerPreferredNamespacedResourcesWithContext(arg0)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "ServerPreferredNamespacedResourcesWithContext failed", "duration", time.Since(start))
	} else {
		logger.Info("ServerPreferredNamespacedResourcesWithContext done", "duration", time.Since(start))
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
func (c *withLogging) ServerPreferredResourcesWithContext(arg0 context.Context) ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "ServerPreferredResourcesWithContext")
	ret0, ret1 := c.inner.ServerPreferredResourcesWithContext(arg0)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "ServerPreferredResourcesWithContext failed", "duration", time.Since(start))
	} else {
		logger.Info("ServerPreferredResourcesWithContext done", "duration", time.Since(start))
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
func (c *withLogging) ServerResourcesForGroupVersionWithContext(arg0 context.Context, arg1 string) (*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "ServerResourcesForGroupVersionWithContext")
	ret0, ret1 := c.inner.ServerResourcesForGroupVersionWithContext(arg0, arg1)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "ServerResourcesForGroupVersionWithContext failed", "duration", time.Since(start))
	} else {
		logger.Info("ServerResourcesForGroupVersionWithContext done", "duration", time.Since(start))
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
func (c *withLogging) ServerVersionWithContext(arg0 context.Context) (*k8s_io_apimachinery_pkg_version.Info, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "ServerVersionWithContext")
	ret0, ret1 := c.inner.ServerVersionWithContext(arg0)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "ServerVersionWithContext failed", "duration", time.Since(start))
	} else {
		logger.Info("ServerVersionWithContext done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) WithLegacy() k8s_io_client_go_discovery.DiscoveryInterface {
	start := time.Now()
	logger := c.logger.WithValues("operation", "WithLegacy")
	ret0 := c.inner.WithLegacy()
	logger.Info("WithLegacy done", "duration", time.Since(start))
	return ret0
}
func (c *withLogging) WithLegacyWithContext(arg0 context.Context) k8s_io_client_go_discovery.DiscoveryInterfaceWithContext {
	start := time.Now()
	logger := c.logger.WithValues("operation", "WithLegacyWithContext")
	ret0 := c.inner.WithLegacyWithContext(arg0)
	logger.Info("WithLegacyWithContext done", "duration", time.Since(start))
	return ret0
}

type withMetrics struct {
	inner    k8s_io_client_go_discovery.DiscoveryInterfaces
	recorder metrics.Recorder
}

func (c *withMetrics) OpenAPISchema() (*github_com_google_gnostic_models_openapiv2.Document, error) {
	defer c.recorder.Record("open_api_schema")
	return c.inner.OpenAPISchema()
}
func (c *withMetrics) OpenAPISchemaWithContext(arg0 context.Context) (*github_com_google_gnostic_models_openapiv2.Document, error) {
	defer c.recorder.RecordWithContext(arg0, "open_api_schema_with_context")
	return c.inner.OpenAPISchemaWithContext(arg0)
}
func (c *withMetrics) OpenAPIV3() k8s_io_client_go_openapi.Client {
	defer c.recorder.Record("open_apiv3")
	return c.inner.OpenAPIV3()
}
func (c *withMetrics) OpenAPIV3WithContext(arg0 context.Context) k8s_io_client_go_openapi.ClientWithContext {
	defer c.recorder.RecordWithContext(arg0, "open_apiv3_with_context")
	return c.inner.OpenAPIV3WithContext(arg0)
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
func (c *withMetrics) ServerGroupsAndResourcesWithContext(arg0 context.Context) ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIGroup, []*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	defer c.recorder.RecordWithContext(arg0, "server_groups_and_resources_with_context")
	return c.inner.ServerGroupsAndResourcesWithContext(arg0)
}
func (c *withMetrics) ServerGroupsWithContext(arg0 context.Context) (*k8s_io_apimachinery_pkg_apis_meta_v1.APIGroupList, error) {
	defer c.recorder.RecordWithContext(arg0, "server_groups_with_context")
	return c.inner.ServerGroupsWithContext(arg0)
}
func (c *withMetrics) ServerPreferredNamespacedResources() ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	defer c.recorder.Record("server_preferred_namespaced_resources")
	return c.inner.ServerPreferredNamespacedResources()
}
func (c *withMetrics) ServerPreferredNamespacedResourcesWithContext(arg0 context.Context) ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	defer c.recorder.RecordWithContext(arg0, "server_preferred_namespaced_resources_with_context")
	return c.inner.ServerPreferredNamespacedResourcesWithContext(arg0)
}
func (c *withMetrics) ServerPreferredResources() ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	defer c.recorder.Record("server_preferred_resources")
	return c.inner.ServerPreferredResources()
}
func (c *withMetrics) ServerPreferredResourcesWithContext(arg0 context.Context) ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	defer c.recorder.RecordWithContext(arg0, "server_preferred_resources_with_context")
	return c.inner.ServerPreferredResourcesWithContext(arg0)
}
func (c *withMetrics) ServerResourcesForGroupVersion(arg0 string) (*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	defer c.recorder.Record("server_resources_for_group_version")
	return c.inner.ServerResourcesForGroupVersion(arg0)
}
func (c *withMetrics) ServerResourcesForGroupVersionWithContext(arg0 context.Context, arg1 string) (*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	defer c.recorder.RecordWithContext(arg0, "server_resources_for_group_version_with_context")
	return c.inner.ServerResourcesForGroupVersionWithContext(arg0, arg1)
}
func (c *withMetrics) ServerVersion() (*k8s_io_apimachinery_pkg_version.Info, error) {
	defer c.recorder.Record("server_version")
	return c.inner.ServerVersion()
}
func (c *withMetrics) ServerVersionWithContext(arg0 context.Context) (*k8s_io_apimachinery_pkg_version.Info, error) {
	defer c.recorder.RecordWithContext(arg0, "server_version_with_context")
	return c.inner.ServerVersionWithContext(arg0)
}
func (c *withMetrics) WithLegacy() k8s_io_client_go_discovery.DiscoveryInterface {
	defer c.recorder.Record("with_legacy")
	return c.inner.WithLegacy()
}
func (c *withMetrics) WithLegacyWithContext(arg0 context.Context) k8s_io_client_go_discovery.DiscoveryInterfaceWithContext {
	defer c.recorder.RecordWithContext(arg0, "with_legacy_with_context")
	return c.inner.WithLegacyWithContext(arg0)
}

type withTracing struct {
	inner  k8s_io_client_go_discovery.DiscoveryInterfaces
	client string
	kind   string
}

func (c *withTracing) OpenAPISchema() (*github_com_google_gnostic_models_openapiv2.Document, error) {
	return c.inner.OpenAPISchema()
}
func (c *withTracing) OpenAPISchemaWithContext(arg0 context.Context) (*github_com_google_gnostic_models_openapiv2.Document, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "OpenAPISchemaWithContext"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("OpenAPISchemaWithContext"),
			),
		)
		defer span.End()
	}
	ret0, ret1 := c.inner.OpenAPISchemaWithContext(arg0)
	if span != nil {
		tracing.SetSpanStatus(span, ret1)
	}
	return ret0, ret1
}
func (c *withTracing) OpenAPIV3() k8s_io_client_go_openapi.Client {
	return c.inner.OpenAPIV3()
}
func (c *withTracing) OpenAPIV3WithContext(arg0 context.Context) k8s_io_client_go_openapi.ClientWithContext {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "OpenAPIV3WithContext"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("OpenAPIV3WithContext"),
			),
		)
		defer span.End()
	}
	ret0 := c.inner.OpenAPIV3WithContext(arg0)
	if span != nil {
	}
	return ret0
}
func (c *withTracing) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}
func (c *withTracing) ServerGroups() (*k8s_io_apimachinery_pkg_apis_meta_v1.APIGroupList, error) {
	return c.inner.ServerGroups()
}
func (c *withTracing) ServerGroupsAndResources() ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIGroup, []*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	return c.inner.ServerGroupsAndResources()
}
func (c *withTracing) ServerGroupsAndResourcesWithContext(arg0 context.Context) ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIGroup, []*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "ServerGroupsAndResourcesWithContext"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("ServerGroupsAndResourcesWithContext"),
			),
		)
		defer span.End()
	}
	ret0, ret1, ret2 := c.inner.ServerGroupsAndResourcesWithContext(arg0)
	if span != nil {
		tracing.SetSpanStatus(span, ret2)
	}
	return ret0, ret1, ret2
}
func (c *withTracing) ServerGroupsWithContext(arg0 context.Context) (*k8s_io_apimachinery_pkg_apis_meta_v1.APIGroupList, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "ServerGroupsWithContext"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("ServerGroupsWithContext"),
			),
		)
		defer span.End()
	}
	ret0, ret1 := c.inner.ServerGroupsWithContext(arg0)
	if span != nil {
		tracing.SetSpanStatus(span, ret1)
	}
	return ret0, ret1
}
func (c *withTracing) ServerPreferredNamespacedResources() ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	return c.inner.ServerPreferredNamespacedResources()
}
func (c *withTracing) ServerPreferredNamespacedResourcesWithContext(arg0 context.Context) ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "ServerPreferredNamespacedResourcesWithContext"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("ServerPreferredNamespacedResourcesWithContext"),
			),
		)
		defer span.End()
	}
	ret0, ret1 := c.inner.ServerPreferredNamespacedResourcesWithContext(arg0)
	if span != nil {
		tracing.SetSpanStatus(span, ret1)
	}
	return ret0, ret1
}
func (c *withTracing) ServerPreferredResources() ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	return c.inner.ServerPreferredResources()
}
func (c *withTracing) ServerPreferredResourcesWithContext(arg0 context.Context) ([]*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "ServerPreferredResourcesWithContext"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("ServerPreferredResourcesWithContext"),
			),
		)
		defer span.End()
	}
	ret0, ret1 := c.inner.ServerPreferredResourcesWithContext(arg0)
	if span != nil {
		tracing.SetSpanStatus(span, ret1)
	}
	return ret0, ret1
}
func (c *withTracing) ServerResourcesForGroupVersion(arg0 string) (*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	return c.inner.ServerResourcesForGroupVersion(arg0)
}
func (c *withTracing) ServerResourcesForGroupVersionWithContext(arg0 context.Context, arg1 string) (*k8s_io_apimachinery_pkg_apis_meta_v1.APIResourceList, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "ServerResourcesForGroupVersionWithContext"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("ServerResourcesForGroupVersionWithContext"),
			),
		)
		defer span.End()
	}
	ret0, ret1 := c.inner.ServerResourcesForGroupVersionWithContext(arg0, arg1)
	if span != nil {
		tracing.SetSpanStatus(span, ret1)
	}
	return ret0, ret1
}
func (c *withTracing) ServerVersion() (*k8s_io_apimachinery_pkg_version.Info, error) {
	return c.inner.ServerVersion()
}
func (c *withTracing) ServerVersionWithContext(arg0 context.Context) (*k8s_io_apimachinery_pkg_version.Info, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "ServerVersionWithContext"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("ServerVersionWithContext"),
			),
		)
		defer span.End()
	}
	ret0, ret1 := c.inner.ServerVersionWithContext(arg0)
	if span != nil {
		tracing.SetSpanStatus(span, ret1)
	}
	return ret0, ret1
}
func (c *withTracing) WithLegacy() k8s_io_client_go_discovery.DiscoveryInterface {
	return c.inner.WithLegacy()
}
func (c *withTracing) WithLegacyWithContext(arg0 context.Context) k8s_io_client_go_discovery.DiscoveryInterfaceWithContext {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "WithLegacyWithContext"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("WithLegacyWithContext"),
			),
		)
		defer span.End()
	}
	ret0 := c.inner.WithLegacyWithContext(arg0)
	if span != nil {
	}
	return ret0
}
