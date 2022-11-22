package resource

import (
	context "context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/multierr"
	k8s_io_apimachinery_pkg_apis_meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_io_apimachinery_pkg_types "k8s.io/apimachinery/pkg/types"
	k8s_io_apimachinery_pkg_watch "k8s.io/apimachinery/pkg/watch"
	k8s_io_client_go_metadata "k8s.io/client-go/metadata"
)

func WithLogging(inner k8s_io_client_go_metadata.ResourceInterface, logger logr.Logger) k8s_io_client_go_metadata.ResourceInterface {
	return &withLogging{inner, logger}
}

func WithMetrics(inner k8s_io_client_go_metadata.ResourceInterface, recorder metrics.Recorder) k8s_io_client_go_metadata.ResourceInterface {
	return &withMetrics{inner, recorder}
}

func WithTracing(inner k8s_io_client_go_metadata.ResourceInterface, client, kind string) k8s_io_client_go_metadata.ResourceInterface {
	return &withTracing{inner, client, kind}
}

type withLogging struct {
	inner  k8s_io_client_go_metadata.ResourceInterface
	logger logr.Logger
}

func (c *withLogging) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg3 ...string) error {
	start := time.Now()
	logger := c.logger.WithValues("operation", "Delete")
	ret0 := c.inner.Delete(arg0, arg1, arg2, arg3...)
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
func (c *withLogging) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions, arg3 ...string) (*k8s_io_apimachinery_pkg_apis_meta_v1.PartialObjectMetadata, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "Get")
	ret0, ret1 := c.inner.Get(arg0, arg1, arg2, arg3...)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "Get failed", "duration", time.Since(start))
	} else {
		logger.Info("Get done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_apimachinery_pkg_apis_meta_v1.PartialObjectMetadataList, error) {
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
func (c *withLogging) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_apimachinery_pkg_apis_meta_v1.PartialObjectMetadata, error) {
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
	inner    k8s_io_client_go_metadata.ResourceInterface
	recorder metrics.Recorder
}

func (c *withMetrics) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg3 ...string) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2, arg3...)
}
func (c *withMetrics) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *withMetrics) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions, arg3 ...string) (*k8s_io_apimachinery_pkg_apis_meta_v1.PartialObjectMetadata, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2, arg3...)
}
func (c *withMetrics) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_apimachinery_pkg_apis_meta_v1.PartialObjectMetadataList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *withMetrics) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_apimachinery_pkg_apis_meta_v1.PartialObjectMetadata, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *withMetrics) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

type withTracing struct {
	inner  k8s_io_client_go_metadata.ResourceInterface
	client string
	kind   string
}

func (c *withTracing) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg3 ...string) error {
	ctx, span := tracing.StartSpan(
		arg0,
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "Delete"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "Delete"),
	)
	defer span.End()
	arg0 = ctx
	ret0 := c.inner.Delete(arg0, arg1, arg2, arg3...)
	if ret0 != nil {
		span.RecordError(ret0)
		span.SetStatus(codes.Error, ret0.Error())
	}
	return ret0
}
func (c *withTracing) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := tracing.StartSpan(
		arg0,
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "DeleteCollection"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "DeleteCollection"),
	)
	defer span.End()
	arg0 = ctx
	ret0 := c.inner.DeleteCollection(arg0, arg1, arg2)
	if ret0 != nil {
		span.RecordError(ret0)
		span.SetStatus(codes.Error, ret0.Error())
	}
	return ret0
}
func (c *withTracing) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions, arg3 ...string) (*k8s_io_apimachinery_pkg_apis_meta_v1.PartialObjectMetadata, error) {
	ctx, span := tracing.StartSpan(
		arg0,
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "Get"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "Get"),
	)
	defer span.End()
	arg0 = ctx
	ret0, ret1 := c.inner.Get(arg0, arg1, arg2, arg3...)
	if ret1 != nil {
		span.RecordError(ret1)
		span.SetStatus(codes.Error, ret1.Error())
	}
	return ret0, ret1
}
func (c *withTracing) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_apimachinery_pkg_apis_meta_v1.PartialObjectMetadataList, error) {
	ctx, span := tracing.StartSpan(
		arg0,
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "List"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "List"),
	)
	defer span.End()
	arg0 = ctx
	ret0, ret1 := c.inner.List(arg0, arg1)
	if ret1 != nil {
		span.RecordError(ret1)
		span.SetStatus(codes.Error, ret1.Error())
	}
	return ret0, ret1
}
func (c *withTracing) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_apimachinery_pkg_apis_meta_v1.PartialObjectMetadata, error) {
	ctx, span := tracing.StartSpan(
		arg0,
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "Patch"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "Patch"),
	)
	defer span.End()
	arg0 = ctx
	ret0, ret1 := c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
	if ret1 != nil {
		span.RecordError(ret1)
		span.SetStatus(codes.Error, ret1.Error())
	}
	return ret0, ret1
}
func (c *withTracing) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := tracing.StartSpan(
		arg0,
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "Watch"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "Watch"),
	)
	defer span.End()
	arg0 = ctx
	ret0, ret1 := c.inner.Watch(arg0, arg1)
	if ret1 != nil {
		span.RecordError(ret1)
		span.SetStatus(codes.Error, ret1.Error())
	}
	return ret0, ret1
}
