package resource

import (
	context "context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/multierr"
	k8s_io_api_core_v1 "k8s.io/api/core/v1"
	k8s_io_apimachinery_pkg_apis_meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_io_apimachinery_pkg_fields "k8s.io/apimachinery/pkg/fields"
	k8s_io_apimachinery_pkg_runtime "k8s.io/apimachinery/pkg/runtime"
	k8s_io_apimachinery_pkg_types "k8s.io/apimachinery/pkg/types"
	k8s_io_apimachinery_pkg_watch "k8s.io/apimachinery/pkg/watch"
	k8s_io_client_go_applyconfigurations_core_v1 "k8s.io/client-go/applyconfigurations/core/v1"
	k8s_io_client_go_kubernetes_typed_core_v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func WithLogging(inner k8s_io_client_go_kubernetes_typed_core_v1.EventInterface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_core_v1.EventInterface {
	return &withLogging{inner, logger}
}

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_core_v1.EventInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_core_v1.EventInterface {
	return &withMetrics{inner, recorder}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_core_v1.EventInterface, client, kind string) k8s_io_client_go_kubernetes_typed_core_v1.EventInterface {
	return &withTracing{inner, client, kind}
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_core_v1.EventInterface
	logger logr.Logger
}

func (c *withLogging) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.EventApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Event, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "Apply")
	ret0, ret1 := c.inner.Apply(arg0, arg1, arg2)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "Apply failed", "duration", time.Since(start))
	} else {
		logger.Info("Apply done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Event, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Event, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "Create")
	ret0, ret1 := c.inner.Create(arg0, arg1, arg2)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "Create failed", "duration", time.Since(start))
	} else {
		logger.Info("Create done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) CreateWithEventNamespace(arg0 *k8s_io_api_core_v1.Event) (*k8s_io_api_core_v1.Event, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "CreateWithEventNamespace")
	ret0, ret1 := c.inner.CreateWithEventNamespace(arg0)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "CreateWithEventNamespace failed", "duration", time.Since(start))
	} else {
		logger.Info("CreateWithEventNamespace done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) CreateWithEventNamespaceWithContext(arg0 context.Context, arg1 *k8s_io_api_core_v1.Event) (*k8s_io_api_core_v1.Event, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "CreateWithEventNamespaceWithContext")
	ret0, ret1 := c.inner.CreateWithEventNamespaceWithContext(arg0, arg1)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "CreateWithEventNamespaceWithContext failed", "duration", time.Since(start))
	} else {
		logger.Info("CreateWithEventNamespaceWithContext done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	start := time.Now()
	logger := c.logger.WithValues("operation", "Delete")
	ret0 := c.inner.Delete(arg0, arg1, arg2)
	if err := multierr.Combine(ret0); err != nil {
		logger.Error(err, "Delete failed", "duration", time.Since(start))
	} else {
		logger.Info("Delete done", "duration", time.Since(start))
	}
	return ret0
}
func (c *withLogging) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	start := time.Now()
	logger := c.logger.WithValues("operation", "DeleteCollection")
	ret0 := c.inner.DeleteCollection(arg0, arg1, arg2)
	if err := multierr.Combine(ret0); err != nil {
		logger.Error(err, "DeleteCollection failed", "duration", time.Since(start))
	} else {
		logger.Info("DeleteCollection done", "duration", time.Since(start))
	}
	return ret0
}
func (c *withLogging) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.Event, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "Get")
	ret0, ret1 := c.inner.Get(arg0, arg1, arg2)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "Get failed", "duration", time.Since(start))
	} else {
		logger.Info("Get done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) GetFieldSelector(arg0 *string, arg1 *string, arg2 *string, arg3 *string) k8s_io_apimachinery_pkg_fields.Selector {
	start := time.Now()
	logger := c.logger.WithValues("operation", "GetFieldSelector")
	ret0 := c.inner.GetFieldSelector(arg0, arg1, arg2, arg3)
	logger.Info("GetFieldSelector done", "duration", time.Since(start))
	return ret0
}
func (c *withLogging) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.EventList, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "List")
	ret0, ret1 := c.inner.List(arg0, arg1)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "List failed", "duration", time.Since(start))
	} else {
		logger.Info("List done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Event, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "Patch")
	ret0, ret1 := c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "Patch failed", "duration", time.Since(start))
	} else {
		logger.Info("Patch done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) PatchWithEventNamespace(arg0 *k8s_io_api_core_v1.Event, arg1 []uint8) (*k8s_io_api_core_v1.Event, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "PatchWithEventNamespace")
	ret0, ret1 := c.inner.PatchWithEventNamespace(arg0, arg1)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "PatchWithEventNamespace failed", "duration", time.Since(start))
	} else {
		logger.Info("PatchWithEventNamespace done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) PatchWithEventNamespaceWithContext(arg0 context.Context, arg1 *k8s_io_api_core_v1.Event, arg2 []uint8) (*k8s_io_api_core_v1.Event, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "PatchWithEventNamespaceWithContext")
	ret0, ret1 := c.inner.PatchWithEventNamespaceWithContext(arg0, arg1, arg2)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "PatchWithEventNamespaceWithContext failed", "duration", time.Since(start))
	} else {
		logger.Info("PatchWithEventNamespaceWithContext done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) Search(arg0 *k8s_io_apimachinery_pkg_runtime.Scheme, arg1 k8s_io_apimachinery_pkg_runtime.Object) (*k8s_io_api_core_v1.EventList, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "Search")
	ret0, ret1 := c.inner.Search(arg0, arg1)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "Search failed", "duration", time.Since(start))
	} else {
		logger.Info("Search done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) SearchWithContext(arg0 context.Context, arg1 *k8s_io_apimachinery_pkg_runtime.Scheme, arg2 k8s_io_apimachinery_pkg_runtime.Object) (*k8s_io_api_core_v1.EventList, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "SearchWithContext")
	ret0, ret1 := c.inner.SearchWithContext(arg0, arg1, arg2)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "SearchWithContext failed", "duration", time.Since(start))
	} else {
		logger.Info("SearchWithContext done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Event, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Event, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "Update")
	ret0, ret1 := c.inner.Update(arg0, arg1, arg2)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "Update failed", "duration", time.Since(start))
	} else {
		logger.Info("Update done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) UpdateWithEventNamespace(arg0 *k8s_io_api_core_v1.Event) (*k8s_io_api_core_v1.Event, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "UpdateWithEventNamespace")
	ret0, ret1 := c.inner.UpdateWithEventNamespace(arg0)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "UpdateWithEventNamespace failed", "duration", time.Since(start))
	} else {
		logger.Info("UpdateWithEventNamespace done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) UpdateWithEventNamespaceWithContext(arg0 context.Context, arg1 *k8s_io_api_core_v1.Event) (*k8s_io_api_core_v1.Event, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "UpdateWithEventNamespaceWithContext")
	ret0, ret1 := c.inner.UpdateWithEventNamespaceWithContext(arg0, arg1)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "UpdateWithEventNamespaceWithContext failed", "duration", time.Since(start))
	} else {
		logger.Info("UpdateWithEventNamespaceWithContext done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "Watch")
	ret0, ret1 := c.inner.Watch(arg0, arg1)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "Watch failed", "duration", time.Since(start))
	} else {
		logger.Info("Watch done", "duration", time.Since(start))
	}
	return ret0, ret1
}

type withMetrics struct {
	inner    k8s_io_client_go_kubernetes_typed_core_v1.EventInterface
	recorder metrics.Recorder
}

func (c *withMetrics) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.EventApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Event, error) {
	defer c.recorder.RecordWithContext(arg0, "apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *withMetrics) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Event, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Event, error) {
	defer c.recorder.RecordWithContext(arg0, "create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *withMetrics) CreateWithEventNamespace(arg0 *k8s_io_api_core_v1.Event) (*k8s_io_api_core_v1.Event, error) {
	defer c.recorder.Record("create_with_event_namespace")
	return c.inner.CreateWithEventNamespace(arg0)
}
func (c *withMetrics) CreateWithEventNamespaceWithContext(arg0 context.Context, arg1 *k8s_io_api_core_v1.Event) (*k8s_io_api_core_v1.Event, error) {
	defer c.recorder.RecordWithContext(arg0, "create_with_event_namespace_with_context")
	return c.inner.CreateWithEventNamespaceWithContext(arg0, arg1)
}
func (c *withMetrics) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.RecordWithContext(arg0, "delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *withMetrics) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.RecordWithContext(arg0, "delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *withMetrics) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.Event, error) {
	defer c.recorder.RecordWithContext(arg0, "get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *withMetrics) GetFieldSelector(arg0 *string, arg1 *string, arg2 *string, arg3 *string) k8s_io_apimachinery_pkg_fields.Selector {
	defer c.recorder.Record("get_field_selector")
	return c.inner.GetFieldSelector(arg0, arg1, arg2, arg3)
}
func (c *withMetrics) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.EventList, error) {
	defer c.recorder.RecordWithContext(arg0, "list")
	return c.inner.List(arg0, arg1)
}
func (c *withMetrics) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Event, error) {
	defer c.recorder.RecordWithContext(arg0, "patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *withMetrics) PatchWithEventNamespace(arg0 *k8s_io_api_core_v1.Event, arg1 []uint8) (*k8s_io_api_core_v1.Event, error) {
	defer c.recorder.Record("patch_with_event_namespace")
	return c.inner.PatchWithEventNamespace(arg0, arg1)
}
func (c *withMetrics) PatchWithEventNamespaceWithContext(arg0 context.Context, arg1 *k8s_io_api_core_v1.Event, arg2 []uint8) (*k8s_io_api_core_v1.Event, error) {
	defer c.recorder.RecordWithContext(arg0, "patch_with_event_namespace_with_context")
	return c.inner.PatchWithEventNamespaceWithContext(arg0, arg1, arg2)
}
func (c *withMetrics) Search(arg0 *k8s_io_apimachinery_pkg_runtime.Scheme, arg1 k8s_io_apimachinery_pkg_runtime.Object) (*k8s_io_api_core_v1.EventList, error) {
	defer c.recorder.Record("search")
	return c.inner.Search(arg0, arg1)
}
func (c *withMetrics) SearchWithContext(arg0 context.Context, arg1 *k8s_io_apimachinery_pkg_runtime.Scheme, arg2 k8s_io_apimachinery_pkg_runtime.Object) (*k8s_io_api_core_v1.EventList, error) {
	defer c.recorder.RecordWithContext(arg0, "search_with_context")
	return c.inner.SearchWithContext(arg0, arg1, arg2)
}
func (c *withMetrics) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Event, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Event, error) {
	defer c.recorder.RecordWithContext(arg0, "update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *withMetrics) UpdateWithEventNamespace(arg0 *k8s_io_api_core_v1.Event) (*k8s_io_api_core_v1.Event, error) {
	defer c.recorder.Record("update_with_event_namespace")
	return c.inner.UpdateWithEventNamespace(arg0)
}
func (c *withMetrics) UpdateWithEventNamespaceWithContext(arg0 context.Context, arg1 *k8s_io_api_core_v1.Event) (*k8s_io_api_core_v1.Event, error) {
	defer c.recorder.RecordWithContext(arg0, "update_with_event_namespace_with_context")
	return c.inner.UpdateWithEventNamespaceWithContext(arg0, arg1)
}
func (c *withMetrics) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.RecordWithContext(arg0, "watch")
	return c.inner.Watch(arg0, arg1)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_core_v1.EventInterface
	client string
	kind   string
}

func (c *withTracing) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_core_v1.EventApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_core_v1.Event, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "Apply"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("Apply"),
			),
		)
		defer span.End()
	}
	ret0, ret1 := c.inner.Apply(arg0, arg1, arg2)
	if span != nil {
		tracing.SetSpanStatus(span, ret1)
	}
	return ret0, ret1
}
func (c *withTracing) Create(arg0 context.Context, arg1 *k8s_io_api_core_v1.Event, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_core_v1.Event, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "Create"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("Create"),
			),
		)
		defer span.End()
	}
	ret0, ret1 := c.inner.Create(arg0, arg1, arg2)
	if span != nil {
		tracing.SetSpanStatus(span, ret1)
	}
	return ret0, ret1
}
func (c *withTracing) CreateWithEventNamespace(arg0 *k8s_io_api_core_v1.Event) (*k8s_io_api_core_v1.Event, error) {
	return c.inner.CreateWithEventNamespace(arg0)
}
func (c *withTracing) CreateWithEventNamespaceWithContext(arg0 context.Context, arg1 *k8s_io_api_core_v1.Event) (*k8s_io_api_core_v1.Event, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "CreateWithEventNamespaceWithContext"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("CreateWithEventNamespaceWithContext"),
			),
		)
		defer span.End()
	}
	ret0, ret1 := c.inner.CreateWithEventNamespaceWithContext(arg0, arg1)
	if span != nil {
		tracing.SetSpanStatus(span, ret1)
	}
	return ret0, ret1
}
func (c *withTracing) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "Delete"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("Delete"),
			),
		)
		defer span.End()
	}
	ret0 := c.inner.Delete(arg0, arg1, arg2)
	if span != nil {
		tracing.SetSpanStatus(span, ret0)
	}
	return ret0
}
func (c *withTracing) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "DeleteCollection"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("DeleteCollection"),
			),
		)
		defer span.End()
	}
	ret0 := c.inner.DeleteCollection(arg0, arg1, arg2)
	if span != nil {
		tracing.SetSpanStatus(span, ret0)
	}
	return ret0
}
func (c *withTracing) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_core_v1.Event, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "Get"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("Get"),
			),
		)
		defer span.End()
	}
	ret0, ret1 := c.inner.Get(arg0, arg1, arg2)
	if span != nil {
		tracing.SetSpanStatus(span, ret1)
	}
	return ret0, ret1
}
func (c *withTracing) GetFieldSelector(arg0 *string, arg1 *string, arg2 *string, arg3 *string) k8s_io_apimachinery_pkg_fields.Selector {
	return c.inner.GetFieldSelector(arg0, arg1, arg2, arg3)
}
func (c *withTracing) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_core_v1.EventList, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "List"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("List"),
			),
		)
		defer span.End()
	}
	ret0, ret1 := c.inner.List(arg0, arg1)
	if span != nil {
		tracing.SetSpanStatus(span, ret1)
	}
	return ret0, ret1
}
func (c *withTracing) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_core_v1.Event, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "Patch"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("Patch"),
			),
		)
		defer span.End()
	}
	ret0, ret1 := c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
	if span != nil {
		tracing.SetSpanStatus(span, ret1)
	}
	return ret0, ret1
}
func (c *withTracing) PatchWithEventNamespace(arg0 *k8s_io_api_core_v1.Event, arg1 []uint8) (*k8s_io_api_core_v1.Event, error) {
	return c.inner.PatchWithEventNamespace(arg0, arg1)
}
func (c *withTracing) PatchWithEventNamespaceWithContext(arg0 context.Context, arg1 *k8s_io_api_core_v1.Event, arg2 []uint8) (*k8s_io_api_core_v1.Event, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "PatchWithEventNamespaceWithContext"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("PatchWithEventNamespaceWithContext"),
			),
		)
		defer span.End()
	}
	ret0, ret1 := c.inner.PatchWithEventNamespaceWithContext(arg0, arg1, arg2)
	if span != nil {
		tracing.SetSpanStatus(span, ret1)
	}
	return ret0, ret1
}
func (c *withTracing) Search(arg0 *k8s_io_apimachinery_pkg_runtime.Scheme, arg1 k8s_io_apimachinery_pkg_runtime.Object) (*k8s_io_api_core_v1.EventList, error) {
	return c.inner.Search(arg0, arg1)
}
func (c *withTracing) SearchWithContext(arg0 context.Context, arg1 *k8s_io_apimachinery_pkg_runtime.Scheme, arg2 k8s_io_apimachinery_pkg_runtime.Object) (*k8s_io_api_core_v1.EventList, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "SearchWithContext"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("SearchWithContext"),
			),
		)
		defer span.End()
	}
	ret0, ret1 := c.inner.SearchWithContext(arg0, arg1, arg2)
	if span != nil {
		tracing.SetSpanStatus(span, ret1)
	}
	return ret0, ret1
}
func (c *withTracing) Update(arg0 context.Context, arg1 *k8s_io_api_core_v1.Event, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_core_v1.Event, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "Update"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("Update"),
			),
		)
		defer span.End()
	}
	ret0, ret1 := c.inner.Update(arg0, arg1, arg2)
	if span != nil {
		tracing.SetSpanStatus(span, ret1)
	}
	return ret0, ret1
}
func (c *withTracing) UpdateWithEventNamespace(arg0 *k8s_io_api_core_v1.Event) (*k8s_io_api_core_v1.Event, error) {
	return c.inner.UpdateWithEventNamespace(arg0)
}
func (c *withTracing) UpdateWithEventNamespaceWithContext(arg0 context.Context, arg1 *k8s_io_api_core_v1.Event) (*k8s_io_api_core_v1.Event, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "UpdateWithEventNamespaceWithContext"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("UpdateWithEventNamespaceWithContext"),
			),
		)
		defer span.End()
	}
	ret0, ret1 := c.inner.UpdateWithEventNamespaceWithContext(arg0, arg1)
	if span != nil {
		tracing.SetSpanStatus(span, ret1)
	}
	return ret0, ret1
}
func (c *withTracing) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "Watch"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("Watch"),
			),
		)
		defer span.End()
	}
	ret0, ret1 := c.inner.Watch(arg0, arg1)
	if span != nil {
		tracing.SetSpanStatus(span, ret1)
	}
	return ret0, ret1
}
