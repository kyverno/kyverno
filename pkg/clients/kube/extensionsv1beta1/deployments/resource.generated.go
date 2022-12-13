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
	k8s_io_api_extensions_v1beta1 "k8s.io/api/extensions/v1beta1"
	k8s_io_apimachinery_pkg_apis_meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_io_apimachinery_pkg_types "k8s.io/apimachinery/pkg/types"
	k8s_io_apimachinery_pkg_watch "k8s.io/apimachinery/pkg/watch"
	k8s_io_client_go_applyconfigurations_extensions_v1beta1 "k8s.io/client-go/applyconfigurations/extensions/v1beta1"
	k8s_io_client_go_kubernetes_typed_extensions_v1beta1 "k8s.io/client-go/kubernetes/typed/extensions/v1beta1"
)

func WithLogging(inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DeploymentInterface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DeploymentInterface {
	return &withLogging{inner, logger}
}

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DeploymentInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DeploymentInterface {
	return &withMetrics{inner, recorder}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DeploymentInterface, client, kind string) k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DeploymentInterface {
	return &withTracing{inner, client, kind}
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DeploymentInterface
	logger logr.Logger
}

func (c *withLogging) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
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
func (c *withLogging) ApplyScale(arg0 context.Context, arg1 string, arg2 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.ScaleApplyConfiguration, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "ApplyScale")
	ret0, ret1 := c.inner.ApplyScale(arg0, arg1, arg2, arg3)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "ApplyScale failed", "duration", time.Since(start))
	} else {
		logger.Info("ApplyScale done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "ApplyStatus")
	ret0, ret1 := c.inner.ApplyStatus(arg0, arg1, arg2)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "ApplyStatus failed", "duration", time.Since(start))
	} else {
		logger.Info("ApplyStatus done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) Create(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
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
func (c *withLogging) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
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
func (c *withLogging) GetScale(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "GetScale")
	ret0, ret1 := c.inner.GetScale(arg0, arg1, arg2)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "GetScale failed", "duration", time.Since(start))
	} else {
		logger.Info("GetScale done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_extensions_v1beta1.DeploymentList, error) {
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
func (c *withLogging) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
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
func (c *withLogging) Rollback(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.DeploymentRollback, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) error {
	start := time.Now()
	logger := c.logger.WithValues("operation", "Rollback")
	ret0 := c.inner.Rollback(arg0, arg1, arg2)
	if err := multierr.Combine(ret0); err != nil {
		logger.Error(err, "Rollback failed", "duration", time.Since(start))
	} else {
		logger.Info("Rollback done", "duration", time.Since(start))
	}
	return ret0
}
func (c *withLogging) Update(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
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
func (c *withLogging) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_extensions_v1beta1.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "UpdateScale")
	ret0, ret1 := c.inner.UpdateScale(arg0, arg1, arg2, arg3)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "UpdateScale failed", "duration", time.Since(start))
	} else {
		logger.Info("UpdateScale done", "duration", time.Since(start))
	}
	return ret0, ret1
}
func (c *withLogging) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	start := time.Now()
	logger := c.logger.WithValues("operation", "UpdateStatus")
	ret0, ret1 := c.inner.UpdateStatus(arg0, arg1, arg2)
	if err := multierr.Combine(ret1); err != nil {
		logger.Error(err, "UpdateStatus failed", "duration", time.Since(start))
	} else {
		logger.Info("UpdateStatus done", "duration", time.Since(start))
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
	inner    k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DeploymentInterface
	recorder metrics.Recorder
}

func (c *withMetrics) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	defer c.recorder.RecordWithContext(arg0, "apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *withMetrics) ApplyScale(arg0 context.Context, arg1 string, arg2 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.ScaleApplyConfiguration, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	defer c.recorder.RecordWithContext(arg0, "apply_scale")
	return c.inner.ApplyScale(arg0, arg1, arg2, arg3)
}
func (c *withMetrics) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	defer c.recorder.RecordWithContext(arg0, "apply_status")
	return c.inner.ApplyStatus(arg0, arg1, arg2)
}
func (c *withMetrics) Create(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	defer c.recorder.RecordWithContext(arg0, "create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *withMetrics) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.RecordWithContext(arg0, "delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *withMetrics) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.RecordWithContext(arg0, "delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *withMetrics) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	defer c.recorder.RecordWithContext(arg0, "get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *withMetrics) GetScale(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	defer c.recorder.RecordWithContext(arg0, "get_scale")
	return c.inner.GetScale(arg0, arg1, arg2)
}
func (c *withMetrics) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_extensions_v1beta1.DeploymentList, error) {
	defer c.recorder.RecordWithContext(arg0, "list")
	return c.inner.List(arg0, arg1)
}
func (c *withMetrics) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	defer c.recorder.RecordWithContext(arg0, "patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *withMetrics) Rollback(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.DeploymentRollback, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) error {
	defer c.recorder.RecordWithContext(arg0, "rollback")
	return c.inner.Rollback(arg0, arg1, arg2)
}
func (c *withMetrics) Update(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	defer c.recorder.RecordWithContext(arg0, "update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *withMetrics) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_extensions_v1beta1.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	defer c.recorder.RecordWithContext(arg0, "update_scale")
	return c.inner.UpdateScale(arg0, arg1, arg2, arg3)
}
func (c *withMetrics) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	defer c.recorder.RecordWithContext(arg0, "update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *withMetrics) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.RecordWithContext(arg0, "watch")
	return c.inner.Watch(arg0, arg1)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_extensions_v1beta1.DeploymentInterface
	client string
	kind   string
}

func (c *withTracing) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
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
func (c *withTracing) ApplyScale(arg0 context.Context, arg1 string, arg2 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.ScaleApplyConfiguration, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "ApplyScale"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("ApplyScale"),
			),
		)
		defer span.End()
	}
	ret0, ret1 := c.inner.ApplyScale(arg0, arg1, arg2, arg3)
	if span != nil {
		tracing.SetSpanStatus(span, ret1)
	}
	return ret0, ret1
}
func (c *withTracing) ApplyStatus(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_extensions_v1beta1.DeploymentApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "ApplyStatus"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("ApplyStatus"),
			),
		)
		defer span.End()
	}
	ret0, ret1 := c.inner.ApplyStatus(arg0, arg1, arg2)
	if span != nil {
		tracing.SetSpanStatus(span, ret1)
	}
	return ret0, ret1
}
func (c *withTracing) Create(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
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
func (c *withTracing) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
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
func (c *withTracing) GetScale(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "GetScale"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("GetScale"),
			),
		)
		defer span.End()
	}
	ret0, ret1 := c.inner.GetScale(arg0, arg1, arg2)
	if span != nil {
		tracing.SetSpanStatus(span, ret1)
	}
	return ret0, ret1
}
func (c *withTracing) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_extensions_v1beta1.DeploymentList, error) {
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
func (c *withTracing) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
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
func (c *withTracing) Rollback(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.DeploymentRollback, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) error {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "Rollback"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("Rollback"),
			),
		)
		defer span.End()
	}
	ret0 := c.inner.Rollback(arg0, arg1, arg2)
	if span != nil {
		tracing.SetSpanStatus(span, ret0)
	}
	return ret0
}
func (c *withTracing) Update(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
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
func (c *withTracing) UpdateScale(arg0 context.Context, arg1 string, arg2 *k8s_io_api_extensions_v1beta1.Scale, arg3 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Scale, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "UpdateScale"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("UpdateScale"),
			),
		)
		defer span.End()
	}
	ret0, ret1 := c.inner.UpdateScale(arg0, arg1, arg2, arg3)
	if span != nil {
		tracing.SetSpanStatus(span, ret1)
	}
	return ret0, ret1
}
func (c *withTracing) UpdateStatus(arg0 context.Context, arg1 *k8s_io_api_extensions_v1beta1.Deployment, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_extensions_v1beta1.Deployment, error) {
	var span trace.Span
	if tracing.IsInSpan(arg0) {
		arg0, span = tracing.StartChildSpan(
			arg0,
			"",
			fmt.Sprintf("KUBE %s/%s/%s", c.client, c.kind, "UpdateStatus"),
			trace.WithAttributes(
				tracing.KubeClientGroupKey.String(c.client),
				tracing.KubeClientKindKey.String(c.kind),
				tracing.KubeClientOperationKey.String("UpdateStatus"),
			),
		)
		defer span.End()
	}
	ret0, ret1 := c.inner.UpdateStatus(arg0, arg1, arg2)
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
