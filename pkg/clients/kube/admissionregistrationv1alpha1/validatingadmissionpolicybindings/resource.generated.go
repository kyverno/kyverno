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
	k8s_io_api_admissionregistration_v1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	k8s_io_apimachinery_pkg_apis_meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_io_apimachinery_pkg_types "k8s.io/apimachinery/pkg/types"
	k8s_io_apimachinery_pkg_watch "k8s.io/apimachinery/pkg/watch"
	k8s_io_client_go_applyconfigurations_admissionregistration_v1alpha1 "k8s.io/client-go/applyconfigurations/admissionregistration/v1alpha1"
	k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1alpha1"
)

func WithLogging(inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBindingInterface, logger logr.Logger) k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBindingInterface {
	return &withLogging{inner, logger}
}

func WithMetrics(inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBindingInterface, recorder metrics.Recorder) k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBindingInterface {
	return &withMetrics{inner, recorder}
}

func WithTracing(inner k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBindingInterface, client, kind string) k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBindingInterface {
	return &withTracing{inner, client, kind}
}

type withLogging struct {
	inner  k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBindingInterface
	logger logr.Logger
}

func (c *withLogging) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBindingApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBinding, error) {
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
func (c *withLogging) Create(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBinding, error) {
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
func (c *withLogging) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBinding, error) {
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
func (c *withLogging) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBindingList, error) {
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
func (c *withLogging) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBinding, error) {
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
func (c *withLogging) Update(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBinding, error) {
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
	inner    k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBindingInterface
	recorder metrics.Recorder
}

func (c *withMetrics) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBindingApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBinding, error) {
	defer c.recorder.RecordWithContext(arg0, "apply")
	return c.inner.Apply(arg0, arg1, arg2)
}
func (c *withMetrics) Create(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBinding, error) {
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
func (c *withMetrics) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBinding, error) {
	defer c.recorder.RecordWithContext(arg0, "get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *withMetrics) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBindingList, error) {
	defer c.recorder.RecordWithContext(arg0, "list")
	return c.inner.List(arg0, arg1)
}
func (c *withMetrics) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBinding, error) {
	defer c.recorder.RecordWithContext(arg0, "patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *withMetrics) Update(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBinding, error) {
	defer c.recorder.RecordWithContext(arg0, "update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *withMetrics) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.RecordWithContext(arg0, "watch")
	return c.inner.Watch(arg0, arg1)
}

type withTracing struct {
	inner  k8s_io_client_go_kubernetes_typed_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBindingInterface
	client string
	kind   string
}

func (c *withTracing) Apply(arg0 context.Context, arg1 *k8s_io_client_go_applyconfigurations_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBindingApplyConfiguration, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ApplyOptions) (*k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBinding, error) {
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
func (c *withTracing) Create(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBinding, error) {
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
func (c *withTracing) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBinding, error) {
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
func (c *withTracing) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBindingList, error) {
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
func (c *withTracing) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBinding, error) {
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
func (c *withTracing) Update(arg0 context.Context, arg1 *k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBinding, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*k8s_io_api_admissionregistration_v1alpha1.ValidatingAdmissionPolicyBinding, error) {
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
