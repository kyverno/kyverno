package resource

import (
	context "context"
	"fmt"

	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	k8s_io_api_apps_v1 "k8s.io/api/apps/v1"
	k8s_io_api_autoscaling_v1 "k8s.io/api/autoscaling/v1"
	k8s_io_apimachinery_pkg_apis_meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_io_apimachinery_pkg_types "k8s.io/apimachinery/pkg/types"
	k8s_io_apimachinery_pkg_watch "k8s.io/apimachinery/pkg/watch"
	k8s_io_client_go_applyconfigurations_apps_v1 "k8s.io/client-go/applyconfigurations/apps/v1"
	k8s_io_client_go_applyconfigurations_autoscaling_v1 "k8s.io/client-go/applyconfigurations/autoscaling/v1"
	k8s_io_client_go_kubernetes_typed_apps_v1 "k8s.io/client-go/kubernetes/typed/apps/v1"
)

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_apps_v1.ReplicaSetInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_apps_v1.ReplicaSetInterface {
	return &withMetrics{inner, recorder}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_apps_v1.ReplicaSetInterface, client, kind string) k8s_io_client_go_kubernetes_typed_apps_v1.ReplicaSetInterface {
	return &withTracing{inner, client, kind}
}

type withMetrics struct {
	inner    k8s_io_client_go_kubernetes_typed_apps_v1.ReplicaSetInterface
	recorder metrics.Recorder
}

func (c *withMetrics) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.ReplicaSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	defer c.recorder.Record("apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *withMetrics) ApplyScale(arg0 context.Context, arg1 string, arg2 *k8s_io_client_go_applyconfigurations_autoscaling_v1.ScaleApplyConfiguration, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	defer c.recorder.Record("apply_scale")
	return c.inner.ApplyScale(arg0, arg1, arg2, arg3)
}
func (c *withMetrics) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.ReplicaSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	defer c.recorder.Record("apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *withMetrics) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *withMetrics) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *withMetrics) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *withMetrics) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *withMetrics) GetScale(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	defer c.recorder.Record("get_scale")
	return c.inner.GetScale(arg0, arg1, arg2)
}
func (c *withMetrics) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1.ReplicaSetList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *withMetrics) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *withMetrics) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *withMetrics) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_autoscaling_v1.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	defer c.recorder.Record("update_scale")
	return c.inner.UpdateScale(arg0, arg1, arg2, arg3)
}
func (c *withMetrics) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *withMetrics) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_apps_v1.ReplicaSetInterface
	client string
	kind   string
}

func (c *withTracing) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.ReplicaSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	ctx, span := tracing.StartSpan(
		arg0,
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "Apply"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "Apply"),
	)
	defer span.End()
	arg0 = ctx
	ret0, ret1 := c.inner.Apply(arg0, arg1, arg2)
	if ret1 != nil {
		span.RecordError(ret1)
		span.SetStatus(codes.Error, ret1.Error())
	}
	return ret0, ret1
}
func (c *withTracing) ApplyScale(arg0 context.Context, arg1 string, arg2 *k8s_io_client_go_applyconfigurations_autoscaling_v1.ScaleApplyConfiguration, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := tracing.StartSpan(
		arg0,
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "ApplyScale"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "ApplyScale"),
	)
	defer span.End()
	arg0 = ctx
	ret0, ret1 := c.inner.ApplyScale(arg0, arg1, arg2, arg3)
	if ret1 != nil {
		span.RecordError(ret1)
		span.SetStatus(codes.Error, ret1.Error())
	}
	return ret0, ret1
}
func (c *withTracing) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_apps_v1.ReplicaSetApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	ctx, span := tracing.StartSpan(
		arg0,
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "ApplyStatus"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "ApplyStatus"),
	)
	defer span.End()
	arg0 = ctx
	ret0, ret1 := c.inner.ApplyStatus(arg0, arg1, arg2)
	if ret1 != nil {
		span.RecordError(ret1)
		span.SetStatus(codes.Error, ret1.Error())
	}
	return ret0, ret1
}
func (c *withTracing) Create(arg0 context.Context, arg1 *k8s_io_api_apps_v1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	ctx, span := tracing.StartSpan(
		arg0,
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "Create"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "Create"),
	)
	defer span.End()
	arg0 = ctx
	ret0, ret1 := c.inner.Create(arg0, arg1, arg2)
	if ret1 != nil {
		span.RecordError(ret1)
		span.SetStatus(codes.Error, ret1.Error())
	}
	return ret0, ret1
}
func (c *withTracing) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
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
	ret0 := c.inner.Delete(arg0, arg1, arg2)
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
func (c *withTracing) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
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
	ret0, ret1 := c.inner.Get(arg0, arg1, arg2)
	if ret1 != nil {
		span.RecordError(ret1)
		span.SetStatus(codes.Error, ret1.Error())
	}
	return ret0, ret1
}
func (c *withTracing) GetScale(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := tracing.StartSpan(
		arg0,
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "GetScale"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "GetScale"),
	)
	defer span.End()
	arg0 = ctx
	ret0, ret1 := c.inner.GetScale(arg0, arg1, arg2)
	if ret1 != nil {
		span.RecordError(ret1)
		span.SetStatus(codes.Error, ret1.Error())
	}
	return ret0, ret1
}
func (c *withTracing) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_apps_v1.ReplicaSetList, error) {
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
func (c *withTracing) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_apps_v1.ReplicaSet, error) {
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
func (c *withTracing) Update(arg0 context.Context, arg1 *k8s_io_api_apps_v1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	ctx, span := tracing.StartSpan(
		arg0,
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "Update"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "Update"),
	)
	defer span.End()
	arg0 = ctx
	ret0, ret1 := c.inner.Update(arg0, arg1, arg2)
	if ret1 != nil {
		span.RecordError(ret1)
		span.SetStatus(codes.Error, ret1.Error())
	}
	return ret0, ret1
}
func (c *withTracing) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_autoscaling_v1.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_autoscaling_v1.Scale, error) {
	ctx, span := tracing.StartSpan(
		arg0,
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "UpdateScale"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "UpdateScale"),
	)
	defer span.End()
	arg0 = ctx
	ret0, ret1 := c.inner.UpdateScale(arg0, arg1, arg2, arg3)
	if ret1 != nil {
		span.RecordError(ret1)
		span.SetStatus(codes.Error, ret1.Error())
	}
	return ret0, ret1
}
func (c *withTracing) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_apps_v1.ReplicaSet, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_apps_v1.ReplicaSet, error) {
	ctx, span := tracing.StartSpan(
		arg0,
		"",
		fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "UpdateStatus"),
		attribute.String("client", c.client),
		attribute.String("kind", c.kind),
		attribute.String("operation", "UpdateStatus"),
	)
	defer span.End()
	arg0 = ctx
	ret0, ret1 := c.inner.UpdateStatus(arg0, arg1, arg2)
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
