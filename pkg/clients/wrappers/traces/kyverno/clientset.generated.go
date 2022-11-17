package client

import (
	context "context"

	github_com_kyverno_kyverno_api_kyverno_v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	github_com_kyverno_kyverno_api_kyverno_v1alpha1 "github.com/kyverno/kyverno/api/kyverno/v1alpha1"
	github_com_kyverno_kyverno_api_kyverno_v1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	github_com_kyverno_kyverno_api_kyverno_v1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	github_com_kyverno_kyverno_api_policyreport_v1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha1"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha2"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1beta1"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha2"
	github_com_kyverno_kyverno_pkg_tracing "github.com/kyverno/kyverno/pkg/tracing"
	go_opentelemetry_io_otel_attribute "go.opentelemetry.io/otel/attribute"
	k8s_io_apimachinery_pkg_apis_meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_io_apimachinery_pkg_types "k8s.io/apimachinery/pkg/types"
	k8s_io_apimachinery_pkg_watch "k8s.io/apimachinery/pkg/watch"
	k8s_io_client_go_discovery "k8s.io/client-go/discovery"
	k8s_io_client_go_rest "k8s.io/client-go/rest"
)

// Wrap
func Wrap(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned.Interface) github_com_kyverno_kyverno_pkg_client_clientset_versioned.Interface {
	return &clientset{
		inner:               inner,
		kyvernov1:           newKyvernoV1(inner.KyvernoV1()),
		kyvernov1alpha1:     newKyvernoV1alpha1(inner.KyvernoV1alpha1()),
		kyvernov1alpha2:     newKyvernoV1alpha2(inner.KyvernoV1alpha2()),
		kyvernov1beta1:      newKyvernoV1beta1(inner.KyvernoV1beta1()),
		wgpolicyk8sv1alpha2: newWgpolicyk8sV1alpha2(inner.Wgpolicyk8sV1alpha2()),
	}
}

// NewForConfig
func NewForConfig(c *k8s_io_client_go_rest.Config) (github_com_kyverno_kyverno_pkg_client_clientset_versioned.Interface, error) {
	inner, err := github_com_kyverno_kyverno_pkg_client_clientset_versioned.NewForConfig(c)
	if err != nil {
		return nil, err
	}
	return Wrap(inner), nil
}

// clientset wrapper
type clientset struct {
	inner               github_com_kyverno_kyverno_pkg_client_clientset_versioned.Interface
	kyvernov1           github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.KyvernoV1Interface
	kyvernov1alpha1     github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.KyvernoV1alpha1Interface
	kyvernov1alpha2     github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface
	kyvernov1beta1      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface
	wgpolicyk8sv1alpha2 github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface
}

// Discovery is NOT instrumented
func (c *clientset) Discovery() k8s_io_client_go_discovery.DiscoveryInterface {
	return c.inner.Discovery()
}
func (c *clientset) KyvernoV1() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.KyvernoV1Interface {
	return c.kyvernov1
}
func (c *clientset) KyvernoV1alpha1() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.KyvernoV1alpha1Interface {
	return c.kyvernov1alpha1
}
func (c *clientset) KyvernoV1alpha2() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface {
	return c.kyvernov1alpha2
}
func (c *clientset) KyvernoV1beta1() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface {
	return c.kyvernov1beta1
}
func (c *clientset) Wgpolicyk8sV1alpha2() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface {
	return c.wgpolicyk8sv1alpha2
}

// wrappedKyvernoV1 wrapper
type wrappedKyvernoV1 struct {
	inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.KyvernoV1Interface
}

func newKyvernoV1(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.KyvernoV1Interface) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.KyvernoV1Interface {
	return &wrappedKyvernoV1{inner}
}
func (c *wrappedKyvernoV1) ClusterPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.ClusterPolicyInterface {
	return newKyvernoV1ClusterPolicies(c.inner.ClusterPolicies())
}
func (c *wrappedKyvernoV1) GenerateRequests(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.GenerateRequestInterface {
	return newKyvernoV1GenerateRequests(c.inner.GenerateRequests(namespace))
}
func (c *wrappedKyvernoV1) Policies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.PolicyInterface {
	return newKyvernoV1Policies(c.inner.Policies(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedKyvernoV1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedKyvernoV1alpha1 wrapper
type wrappedKyvernoV1alpha1 struct {
	inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.KyvernoV1alpha1Interface
}

func newKyvernoV1alpha1(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.KyvernoV1alpha1Interface) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.KyvernoV1alpha1Interface {
	return &wrappedKyvernoV1alpha1{inner}
}
func (c *wrappedKyvernoV1alpha1) CleanupPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.CleanupPolicyInterface {
	return newKyvernoV1alpha1CleanupPolicies(c.inner.CleanupPolicies(namespace))
}
func (c *wrappedKyvernoV1alpha1) ClusterCleanupPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.ClusterCleanupPolicyInterface {
	return newKyvernoV1alpha1ClusterCleanupPolicies(c.inner.ClusterCleanupPolicies())
}

// RESTClient is NOT instrumented
func (c *wrappedKyvernoV1alpha1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedKyvernoV1alpha2 wrapper
type wrappedKyvernoV1alpha2 struct {
	inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface
}

func newKyvernoV1alpha2(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface {
	return &wrappedKyvernoV1alpha2{inner}
}
func (c *wrappedKyvernoV1alpha2) AdmissionReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.AdmissionReportInterface {
	return newKyvernoV1alpha2AdmissionReports(c.inner.AdmissionReports(namespace))
}
func (c *wrappedKyvernoV1alpha2) BackgroundScanReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.BackgroundScanReportInterface {
	return newKyvernoV1alpha2BackgroundScanReports(c.inner.BackgroundScanReports(namespace))
}
func (c *wrappedKyvernoV1alpha2) ClusterAdmissionReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterAdmissionReportInterface {
	return newKyvernoV1alpha2ClusterAdmissionReports(c.inner.ClusterAdmissionReports())
}
func (c *wrappedKyvernoV1alpha2) ClusterBackgroundScanReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterBackgroundScanReportInterface {
	return newKyvernoV1alpha2ClusterBackgroundScanReports(c.inner.ClusterBackgroundScanReports())
}

// RESTClient is NOT instrumented
func (c *wrappedKyvernoV1alpha2) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedKyvernoV1beta1 wrapper
type wrappedKyvernoV1beta1 struct {
	inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface
}

func newKyvernoV1beta1(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface {
	return &wrappedKyvernoV1beta1{inner}
}
func (c *wrappedKyvernoV1beta1) UpdateRequests(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.UpdateRequestInterface {
	return newKyvernoV1beta1UpdateRequests(c.inner.UpdateRequests(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedKyvernoV1beta1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedWgpolicyk8sV1alpha2 wrapper
type wrappedWgpolicyk8sV1alpha2 struct {
	inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface
}

func newWgpolicyk8sV1alpha2(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface {
	return &wrappedWgpolicyk8sV1alpha2{inner}
}
func (c *wrappedWgpolicyk8sV1alpha2) ClusterPolicyReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.ClusterPolicyReportInterface {
	return newWgpolicyk8sV1alpha2ClusterPolicyReports(c.inner.ClusterPolicyReports())
}
func (c *wrappedWgpolicyk8sV1alpha2) PolicyReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.PolicyReportInterface {
	return newWgpolicyk8sV1alpha2PolicyReports(c.inner.PolicyReports(namespace))
}

// RESTClient is NOT instrumented
func (c *wrappedWgpolicyk8sV1alpha2) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedKyvernoV1ClusterPolicies wrapper
type wrappedKyvernoV1ClusterPolicies struct {
	inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.ClusterPolicyInterface
}

func newKyvernoV1ClusterPolicies(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.ClusterPolicyInterface) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.ClusterPolicyInterface {
	return &wrappedKyvernoV1ClusterPolicies{inner}
}
func (c *wrappedKyvernoV1ClusterPolicies) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/ClusterPolicies", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1ClusterPolicies) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/ClusterPolicies", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1ClusterPolicies) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/ClusterPolicies", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1ClusterPolicies) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/ClusterPolicies", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1ClusterPolicies) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicyList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/ClusterPolicies", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1ClusterPolicies) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/ClusterPolicies", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1ClusterPolicies) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/ClusterPolicies", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1ClusterPolicies) UpdateStatus(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/ClusterPolicies", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1ClusterPolicies) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/ClusterPolicies", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedKyvernoV1GenerateRequests wrapper
type wrappedKyvernoV1GenerateRequests struct {
	inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.GenerateRequestInterface
}

func newKyvernoV1GenerateRequests(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.GenerateRequestInterface) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.GenerateRequestInterface {
	return &wrappedKyvernoV1GenerateRequests{inner}
}
func (c *wrappedKyvernoV1GenerateRequests) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/GenerateRequests", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "GenerateRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "GenerateRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1GenerateRequests) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/GenerateRequests", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "GenerateRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "GenerateRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1GenerateRequests) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/GenerateRequests", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "GenerateRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "GenerateRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1GenerateRequests) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/GenerateRequests", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "GenerateRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "GenerateRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1GenerateRequests) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequestList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/GenerateRequests", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "GenerateRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "GenerateRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1GenerateRequests) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/GenerateRequests", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "GenerateRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "GenerateRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1GenerateRequests) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/GenerateRequests", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "GenerateRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "GenerateRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1GenerateRequests) UpdateStatus(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/GenerateRequests", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "GenerateRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "GenerateRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1GenerateRequests) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/GenerateRequests", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "GenerateRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "GenerateRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedKyvernoV1Policies wrapper
type wrappedKyvernoV1Policies struct {
	inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.PolicyInterface
}

func newKyvernoV1Policies(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.PolicyInterface) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.PolicyInterface {
	return &wrappedKyvernoV1Policies{inner}
}
func (c *wrappedKyvernoV1Policies) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.Policy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.Policy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/Policies", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Policies"),
		go_opentelemetry_io_otel_attribute.String("kind", "Policy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1Policies) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/Policies", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Policies"),
		go_opentelemetry_io_otel_attribute.String("kind", "Policy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1Policies) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/Policies", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Policies"),
		go_opentelemetry_io_otel_attribute.String("kind", "Policy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1Policies) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.Policy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/Policies", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Policies"),
		go_opentelemetry_io_otel_attribute.String("kind", "Policy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1Policies) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.PolicyList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/Policies", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Policies"),
		go_opentelemetry_io_otel_attribute.String("kind", "Policy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1Policies) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1.Policy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/Policies", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Policies"),
		go_opentelemetry_io_otel_attribute.String("kind", "Policy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1Policies) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.Policy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.Policy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/Policies", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Policies"),
		go_opentelemetry_io_otel_attribute.String("kind", "Policy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1Policies) UpdateStatus(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.Policy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.Policy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/Policies", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Policies"),
		go_opentelemetry_io_otel_attribute.String("kind", "Policy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1Policies) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1/Policies", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1"),
		go_opentelemetry_io_otel_attribute.String("resource", "Policies"),
		go_opentelemetry_io_otel_attribute.String("kind", "Policy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedKyvernoV1alpha1CleanupPolicies wrapper
type wrappedKyvernoV1alpha1CleanupPolicies struct {
	inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.CleanupPolicyInterface
}

func newKyvernoV1alpha1CleanupPolicies(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.CleanupPolicyInterface) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.CleanupPolicyInterface {
	return &wrappedKyvernoV1alpha1CleanupPolicies{inner}
}
func (c *wrappedKyvernoV1alpha1CleanupPolicies) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha1/CleanupPolicies", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CleanupPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "CleanupPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1CleanupPolicies) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha1/CleanupPolicies", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CleanupPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "CleanupPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1CleanupPolicies) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha1/CleanupPolicies", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CleanupPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "CleanupPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1CleanupPolicies) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha1/CleanupPolicies", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CleanupPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "CleanupPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1CleanupPolicies) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicyList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha1/CleanupPolicies", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CleanupPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "CleanupPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1alpha1CleanupPolicies) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha1/CleanupPolicies", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CleanupPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "CleanupPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1alpha1CleanupPolicies) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha1/CleanupPolicies", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CleanupPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "CleanupPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1CleanupPolicies) UpdateStatus(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha1/CleanupPolicies", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CleanupPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "CleanupPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1CleanupPolicies) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha1/CleanupPolicies", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "CleanupPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "CleanupPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedKyvernoV1alpha1ClusterCleanupPolicies wrapper
type wrappedKyvernoV1alpha1ClusterCleanupPolicies struct {
	inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.ClusterCleanupPolicyInterface
}

func newKyvernoV1alpha1ClusterCleanupPolicies(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.ClusterCleanupPolicyInterface) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.ClusterCleanupPolicyInterface {
	return &wrappedKyvernoV1alpha1ClusterCleanupPolicies{inner}
}
func (c *wrappedKyvernoV1alpha1ClusterCleanupPolicies) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha1/ClusterCleanupPolicies", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCleanupPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCleanupPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1ClusterCleanupPolicies) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha1/ClusterCleanupPolicies", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCleanupPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCleanupPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1ClusterCleanupPolicies) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha1/ClusterCleanupPolicies", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCleanupPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCleanupPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1ClusterCleanupPolicies) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha1/ClusterCleanupPolicies", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCleanupPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCleanupPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1ClusterCleanupPolicies) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicyList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha1/ClusterCleanupPolicies", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCleanupPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCleanupPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1alpha1ClusterCleanupPolicies) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha1/ClusterCleanupPolicies", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCleanupPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCleanupPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1alpha1ClusterCleanupPolicies) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha1/ClusterCleanupPolicies", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCleanupPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCleanupPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1ClusterCleanupPolicies) UpdateStatus(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha1/ClusterCleanupPolicies", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCleanupPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCleanupPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1ClusterCleanupPolicies) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha1/ClusterCleanupPolicies", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha1"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterCleanupPolicies"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterCleanupPolicy"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedKyvernoV1alpha2AdmissionReports wrapper
type wrappedKyvernoV1alpha2AdmissionReports struct {
	inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.AdmissionReportInterface
}

func newKyvernoV1alpha2AdmissionReports(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.AdmissionReportInterface) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.AdmissionReportInterface {
	return &wrappedKyvernoV1alpha2AdmissionReports{inner}
}
func (c *wrappedKyvernoV1alpha2AdmissionReports) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.AdmissionReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.AdmissionReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/AdmissionReports", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "AdmissionReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "AdmissionReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2AdmissionReports) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/AdmissionReports", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "AdmissionReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "AdmissionReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2AdmissionReports) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/AdmissionReports", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "AdmissionReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "AdmissionReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2AdmissionReports) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.AdmissionReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/AdmissionReports", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "AdmissionReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "AdmissionReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2AdmissionReports) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.AdmissionReportList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/AdmissionReports", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "AdmissionReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "AdmissionReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1alpha2AdmissionReports) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.AdmissionReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/AdmissionReports", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "AdmissionReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "AdmissionReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1alpha2AdmissionReports) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.AdmissionReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.AdmissionReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/AdmissionReports", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "AdmissionReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "AdmissionReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2AdmissionReports) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/AdmissionReports", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "AdmissionReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "AdmissionReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedKyvernoV1alpha2BackgroundScanReports wrapper
type wrappedKyvernoV1alpha2BackgroundScanReports struct {
	inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.BackgroundScanReportInterface
}

func newKyvernoV1alpha2BackgroundScanReports(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.BackgroundScanReportInterface) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.BackgroundScanReportInterface {
	return &wrappedKyvernoV1alpha2BackgroundScanReports{inner}
}
func (c *wrappedKyvernoV1alpha2BackgroundScanReports) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.BackgroundScanReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.BackgroundScanReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/BackgroundScanReports", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "BackgroundScanReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "BackgroundScanReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2BackgroundScanReports) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/BackgroundScanReports", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "BackgroundScanReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "BackgroundScanReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2BackgroundScanReports) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/BackgroundScanReports", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "BackgroundScanReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "BackgroundScanReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2BackgroundScanReports) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.BackgroundScanReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/BackgroundScanReports", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "BackgroundScanReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "BackgroundScanReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2BackgroundScanReports) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.BackgroundScanReportList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/BackgroundScanReports", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "BackgroundScanReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "BackgroundScanReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1alpha2BackgroundScanReports) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.BackgroundScanReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/BackgroundScanReports", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "BackgroundScanReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "BackgroundScanReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1alpha2BackgroundScanReports) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.BackgroundScanReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.BackgroundScanReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/BackgroundScanReports", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "BackgroundScanReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "BackgroundScanReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2BackgroundScanReports) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/BackgroundScanReports", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "BackgroundScanReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "BackgroundScanReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedKyvernoV1alpha2ClusterAdmissionReports wrapper
type wrappedKyvernoV1alpha2ClusterAdmissionReports struct {
	inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterAdmissionReportInterface
}

func newKyvernoV1alpha2ClusterAdmissionReports(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterAdmissionReportInterface) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterAdmissionReportInterface {
	return &wrappedKyvernoV1alpha2ClusterAdmissionReports{inner}
}
func (c *wrappedKyvernoV1alpha2ClusterAdmissionReports) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterAdmissionReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterAdmissionReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/ClusterAdmissionReports", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterAdmissionReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterAdmissionReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2ClusterAdmissionReports) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/ClusterAdmissionReports", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterAdmissionReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterAdmissionReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2ClusterAdmissionReports) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/ClusterAdmissionReports", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterAdmissionReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterAdmissionReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2ClusterAdmissionReports) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterAdmissionReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/ClusterAdmissionReports", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterAdmissionReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterAdmissionReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2ClusterAdmissionReports) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterAdmissionReportList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/ClusterAdmissionReports", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterAdmissionReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterAdmissionReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1alpha2ClusterAdmissionReports) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterAdmissionReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/ClusterAdmissionReports", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterAdmissionReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterAdmissionReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1alpha2ClusterAdmissionReports) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterAdmissionReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterAdmissionReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/ClusterAdmissionReports", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterAdmissionReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterAdmissionReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2ClusterAdmissionReports) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/ClusterAdmissionReports", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterAdmissionReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterAdmissionReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedKyvernoV1alpha2ClusterBackgroundScanReports wrapper
type wrappedKyvernoV1alpha2ClusterBackgroundScanReports struct {
	inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterBackgroundScanReportInterface
}

func newKyvernoV1alpha2ClusterBackgroundScanReports(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterBackgroundScanReportInterface) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterBackgroundScanReportInterface {
	return &wrappedKyvernoV1alpha2ClusterBackgroundScanReports{inner}
}
func (c *wrappedKyvernoV1alpha2ClusterBackgroundScanReports) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterBackgroundScanReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterBackgroundScanReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/ClusterBackgroundScanReports", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterBackgroundScanReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterBackgroundScanReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2ClusterBackgroundScanReports) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/ClusterBackgroundScanReports", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterBackgroundScanReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterBackgroundScanReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2ClusterBackgroundScanReports) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/ClusterBackgroundScanReports", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterBackgroundScanReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterBackgroundScanReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2ClusterBackgroundScanReports) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterBackgroundScanReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/ClusterBackgroundScanReports", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterBackgroundScanReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterBackgroundScanReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2ClusterBackgroundScanReports) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterBackgroundScanReportList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/ClusterBackgroundScanReports", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterBackgroundScanReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterBackgroundScanReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1alpha2ClusterBackgroundScanReports) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterBackgroundScanReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/ClusterBackgroundScanReports", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterBackgroundScanReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterBackgroundScanReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1alpha2ClusterBackgroundScanReports) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterBackgroundScanReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterBackgroundScanReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/ClusterBackgroundScanReports", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterBackgroundScanReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterBackgroundScanReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2ClusterBackgroundScanReports) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1alpha2/ClusterBackgroundScanReports", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterBackgroundScanReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterBackgroundScanReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedKyvernoV1beta1UpdateRequests wrapper
type wrappedKyvernoV1beta1UpdateRequests struct {
	inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.UpdateRequestInterface
}

func newKyvernoV1beta1UpdateRequests(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.UpdateRequestInterface) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.UpdateRequestInterface {
	return &wrappedKyvernoV1beta1UpdateRequests{inner}
}
func (c *wrappedKyvernoV1beta1UpdateRequests) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1beta1/UpdateRequests", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "UpdateRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "UpdateRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1beta1UpdateRequests) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1beta1/UpdateRequests", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "UpdateRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "UpdateRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1beta1UpdateRequests) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1beta1/UpdateRequests", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "UpdateRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "UpdateRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1beta1UpdateRequests) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1beta1/UpdateRequests", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "UpdateRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "UpdateRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1beta1UpdateRequests) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequestList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1beta1/UpdateRequests", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "UpdateRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "UpdateRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1beta1UpdateRequests) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1beta1/UpdateRequests", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "UpdateRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "UpdateRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1beta1UpdateRequests) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1beta1/UpdateRequests", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "UpdateRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "UpdateRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1beta1UpdateRequests) UpdateStatus(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1beta1/UpdateRequests", "UpdateStatus", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "UpdateRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "UpdateRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1beta1UpdateRequests) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "KyvernoV1beta1/UpdateRequests", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "KyvernoV1beta1"),
		go_opentelemetry_io_otel_attribute.String("resource", "UpdateRequests"),
		go_opentelemetry_io_otel_attribute.String("kind", "UpdateRequest"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedWgpolicyk8sV1alpha2ClusterPolicyReports wrapper
type wrappedWgpolicyk8sV1alpha2ClusterPolicyReports struct {
	inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.ClusterPolicyReportInterface
}

func newWgpolicyk8sV1alpha2ClusterPolicyReports(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.ClusterPolicyReportInterface) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.ClusterPolicyReportInterface {
	return &wrappedWgpolicyk8sV1alpha2ClusterPolicyReports{inner}
}
func (c *wrappedWgpolicyk8sV1alpha2ClusterPolicyReports) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_policyreport_v1alpha2.ClusterPolicyReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.ClusterPolicyReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "Wgpolicyk8sV1alpha2/ClusterPolicyReports", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "Wgpolicyk8sV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterPolicyReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterPolicyReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2ClusterPolicyReports) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "Wgpolicyk8sV1alpha2/ClusterPolicyReports", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "Wgpolicyk8sV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterPolicyReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterPolicyReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2ClusterPolicyReports) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "Wgpolicyk8sV1alpha2/ClusterPolicyReports", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "Wgpolicyk8sV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterPolicyReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterPolicyReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2ClusterPolicyReports) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.ClusterPolicyReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "Wgpolicyk8sV1alpha2/ClusterPolicyReports", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "Wgpolicyk8sV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterPolicyReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterPolicyReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2ClusterPolicyReports) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.ClusterPolicyReportList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "Wgpolicyk8sV1alpha2/ClusterPolicyReports", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "Wgpolicyk8sV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterPolicyReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterPolicyReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedWgpolicyk8sV1alpha2ClusterPolicyReports) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.ClusterPolicyReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "Wgpolicyk8sV1alpha2/ClusterPolicyReports", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "Wgpolicyk8sV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterPolicyReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterPolicyReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedWgpolicyk8sV1alpha2ClusterPolicyReports) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_policyreport_v1alpha2.ClusterPolicyReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.ClusterPolicyReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "Wgpolicyk8sV1alpha2/ClusterPolicyReports", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "Wgpolicyk8sV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterPolicyReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterPolicyReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2ClusterPolicyReports) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "Wgpolicyk8sV1alpha2/ClusterPolicyReports", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "Wgpolicyk8sV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "ClusterPolicyReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "ClusterPolicyReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}

// wrappedWgpolicyk8sV1alpha2PolicyReports wrapper
type wrappedWgpolicyk8sV1alpha2PolicyReports struct {
	inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.PolicyReportInterface
}

func newWgpolicyk8sV1alpha2PolicyReports(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.PolicyReportInterface) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.PolicyReportInterface {
	return &wrappedWgpolicyk8sV1alpha2PolicyReports{inner}
}
func (c *wrappedWgpolicyk8sV1alpha2PolicyReports) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_policyreport_v1alpha2.PolicyReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.PolicyReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "Wgpolicyk8sV1alpha2/PolicyReports", "Create", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "Wgpolicyk8sV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PolicyReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "PolicyReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2PolicyReports) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "Wgpolicyk8sV1alpha2/PolicyReports", "Delete", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "Wgpolicyk8sV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PolicyReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "PolicyReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2PolicyReports) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "Wgpolicyk8sV1alpha2/PolicyReports", "DeleteCollection", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "Wgpolicyk8sV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PolicyReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "PolicyReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2PolicyReports) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.PolicyReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "Wgpolicyk8sV1alpha2/PolicyReports", "Get", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "Wgpolicyk8sV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PolicyReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "PolicyReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2PolicyReports) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.PolicyReportList, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "Wgpolicyk8sV1alpha2/PolicyReports", "List", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "Wgpolicyk8sV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PolicyReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "PolicyReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.List(arg0, arg1)
}
func (c *wrappedWgpolicyk8sV1alpha2PolicyReports) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.PolicyReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "Wgpolicyk8sV1alpha2/PolicyReports", "Patch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "Wgpolicyk8sV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PolicyReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "PolicyReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedWgpolicyk8sV1alpha2PolicyReports) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_policyreport_v1alpha2.PolicyReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.PolicyReport, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "Wgpolicyk8sV1alpha2/PolicyReports", "Update", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "Wgpolicyk8sV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PolicyReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "PolicyReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2PolicyReports) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	ctx, span := github_com_kyverno_kyverno_pkg_tracing.StartSpan(arg0, "Wgpolicyk8sV1alpha2/PolicyReports", "Watch", []go_opentelemetry_io_otel_attribute.KeyValue{
		go_opentelemetry_io_otel_attribute.String("client", "Wgpolicyk8sV1alpha2"),
		go_opentelemetry_io_otel_attribute.String("resource", "PolicyReports"),
		go_opentelemetry_io_otel_attribute.String("kind", "PolicyReport"),
	})
	defer span.End()
	arg0 = ctx
	return c.inner.Watch(arg0, arg1)
}
