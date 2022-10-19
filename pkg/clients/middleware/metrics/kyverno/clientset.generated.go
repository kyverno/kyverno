package client

import (
	context "context"

	github_com_kyverno_kyverno_api_kyverno_v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	github_com_kyverno_kyverno_api_kyverno_v1alpha1 "github.com/kyverno/kyverno/api/kyverno/v1alpha1"
	github_com_kyverno_kyverno_api_kyverno_v1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	github_com_kyverno_kyverno_api_kyverno_v1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	github_com_kyverno_kyverno_api_kyverno_v2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	github_com_kyverno_kyverno_api_policyreport_v1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha1"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha2"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1beta1"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v2beta1"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha2"
	github_com_kyverno_kyverno_pkg_metrics "github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_apimachinery_pkg_apis_meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_io_apimachinery_pkg_types "k8s.io/apimachinery/pkg/types"
	k8s_io_apimachinery_pkg_watch "k8s.io/apimachinery/pkg/watch"
	k8s_io_client_go_discovery "k8s.io/client-go/discovery"
	k8s_io_client_go_rest "k8s.io/client-go/rest"
)

// Wrap
func Wrap(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned.Interface, m github_com_kyverno_kyverno_pkg_metrics.MetricsConfigManager, t github_com_kyverno_kyverno_pkg_metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned.Interface {
	return &clientset{
		inner:               inner,
		kyvernov1:           newKyvernoV1(inner.KyvernoV1(), m, t),
		kyvernov1alpha1:     newKyvernoV1alpha1(inner.KyvernoV1alpha1(), m, t),
		kyvernov1alpha2:     newKyvernoV1alpha2(inner.KyvernoV1alpha2(), m, t),
		kyvernov1beta1:      newKyvernoV1beta1(inner.KyvernoV1beta1(), m, t),
		kyvernov2beta1:      newKyvernoV2beta1(inner.KyvernoV2beta1(), m, t),
		wgpolicyk8sv1alpha2: newWgpolicyk8sV1alpha2(inner.Wgpolicyk8sV1alpha2(), m, t),
	}
}

// clientset wrapper
type clientset struct {
	inner               github_com_kyverno_kyverno_pkg_client_clientset_versioned.Interface
	kyvernov1           github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.KyvernoV1Interface
	kyvernov1alpha1     github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.KyvernoV1alpha1Interface
	kyvernov1alpha2     github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface
	kyvernov1beta1      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface
	kyvernov2beta1      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.KyvernoV2beta1Interface
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
func (c *clientset) KyvernoV2beta1() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.KyvernoV2beta1Interface {
	return c.kyvernov2beta1
}
func (c *clientset) Wgpolicyk8sV1alpha2() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface {
	return c.wgpolicyk8sv1alpha2
}

// wrappedKyvernoV1 wrapper
type wrappedKyvernoV1 struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.KyvernoV1Interface
	metrics    github_com_kyverno_kyverno_pkg_metrics.MetricsConfigManager
	clientType github_com_kyverno_kyverno_pkg_metrics.ClientType
}

func newKyvernoV1(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.KyvernoV1Interface, metrics github_com_kyverno_kyverno_pkg_metrics.MetricsConfigManager, t github_com_kyverno_kyverno_pkg_metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.KyvernoV1Interface {
	return &wrappedKyvernoV1{inner, metrics, t}
}
func (c *wrappedKyvernoV1) ClusterPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.ClusterPolicyInterface {
	recorder := github_com_kyverno_kyverno_pkg_metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterPolicy", c.clientType)
	return newKyvernoV1ClusterPolicies(c.inner.ClusterPolicies(), recorder)
}
func (c *wrappedKyvernoV1) GenerateRequests(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.GenerateRequestInterface {
	recorder := github_com_kyverno_kyverno_pkg_metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "GenerateRequest", c.clientType)
	return newKyvernoV1GenerateRequests(c.inner.GenerateRequests(namespace), recorder)
}
func (c *wrappedKyvernoV1) Policies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.PolicyInterface {
	recorder := github_com_kyverno_kyverno_pkg_metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Policy", c.clientType)
	return newKyvernoV1Policies(c.inner.Policies(namespace), recorder)
}

// RESTClient is NOT instrumented
func (c *wrappedKyvernoV1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedKyvernoV1alpha1 wrapper
type wrappedKyvernoV1alpha1 struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.KyvernoV1alpha1Interface
	metrics    github_com_kyverno_kyverno_pkg_metrics.MetricsConfigManager
	clientType github_com_kyverno_kyverno_pkg_metrics.ClientType
}

func newKyvernoV1alpha1(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.KyvernoV1alpha1Interface, metrics github_com_kyverno_kyverno_pkg_metrics.MetricsConfigManager, t github_com_kyverno_kyverno_pkg_metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.KyvernoV1alpha1Interface {
	return &wrappedKyvernoV1alpha1{inner, metrics, t}
}
func (c *wrappedKyvernoV1alpha1) CleanupPolicies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.CleanupPolicyInterface {
	recorder := github_com_kyverno_kyverno_pkg_metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "CleanupPolicy", c.clientType)
	return newKyvernoV1alpha1CleanupPolicies(c.inner.CleanupPolicies(namespace), recorder)
}
func (c *wrappedKyvernoV1alpha1) ClusterCleanupPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.ClusterCleanupPolicyInterface {
	recorder := github_com_kyverno_kyverno_pkg_metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterCleanupPolicy", c.clientType)
	return newKyvernoV1alpha1ClusterCleanupPolicies(c.inner.ClusterCleanupPolicies(), recorder)
}

// RESTClient is NOT instrumented
func (c *wrappedKyvernoV1alpha1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedKyvernoV1alpha2 wrapper
type wrappedKyvernoV1alpha2 struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface
	metrics    github_com_kyverno_kyverno_pkg_metrics.MetricsConfigManager
	clientType github_com_kyverno_kyverno_pkg_metrics.ClientType
}

func newKyvernoV1alpha2(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface, metrics github_com_kyverno_kyverno_pkg_metrics.MetricsConfigManager, t github_com_kyverno_kyverno_pkg_metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface {
	return &wrappedKyvernoV1alpha2{inner, metrics, t}
}
func (c *wrappedKyvernoV1alpha2) AdmissionReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.AdmissionReportInterface {
	recorder := github_com_kyverno_kyverno_pkg_metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "AdmissionReport", c.clientType)
	return newKyvernoV1alpha2AdmissionReports(c.inner.AdmissionReports(namespace), recorder)
}
func (c *wrappedKyvernoV1alpha2) BackgroundScanReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.BackgroundScanReportInterface {
	recorder := github_com_kyverno_kyverno_pkg_metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "BackgroundScanReport", c.clientType)
	return newKyvernoV1alpha2BackgroundScanReports(c.inner.BackgroundScanReports(namespace), recorder)
}
func (c *wrappedKyvernoV1alpha2) ClusterAdmissionReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterAdmissionReportInterface {
	recorder := github_com_kyverno_kyverno_pkg_metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterAdmissionReport", c.clientType)
	return newKyvernoV1alpha2ClusterAdmissionReports(c.inner.ClusterAdmissionReports(), recorder)
}
func (c *wrappedKyvernoV1alpha2) ClusterBackgroundScanReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterBackgroundScanReportInterface {
	recorder := github_com_kyverno_kyverno_pkg_metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterBackgroundScanReport", c.clientType)
	return newKyvernoV1alpha2ClusterBackgroundScanReports(c.inner.ClusterBackgroundScanReports(), recorder)
}

// RESTClient is NOT instrumented
func (c *wrappedKyvernoV1alpha2) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedKyvernoV1beta1 wrapper
type wrappedKyvernoV1beta1 struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface
	metrics    github_com_kyverno_kyverno_pkg_metrics.MetricsConfigManager
	clientType github_com_kyverno_kyverno_pkg_metrics.ClientType
}

func newKyvernoV1beta1(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface, metrics github_com_kyverno_kyverno_pkg_metrics.MetricsConfigManager, t github_com_kyverno_kyverno_pkg_metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface {
	return &wrappedKyvernoV1beta1{inner, metrics, t}
}
func (c *wrappedKyvernoV1beta1) UpdateRequests(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.UpdateRequestInterface {
	recorder := github_com_kyverno_kyverno_pkg_metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "UpdateRequest", c.clientType)
	return newKyvernoV1beta1UpdateRequests(c.inner.UpdateRequests(namespace), recorder)
}

// RESTClient is NOT instrumented
func (c *wrappedKyvernoV1beta1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedKyvernoV2beta1 wrapper
type wrappedKyvernoV2beta1 struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.KyvernoV2beta1Interface
	metrics    github_com_kyverno_kyverno_pkg_metrics.MetricsConfigManager
	clientType github_com_kyverno_kyverno_pkg_metrics.ClientType
}

func newKyvernoV2beta1(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.KyvernoV2beta1Interface, metrics github_com_kyverno_kyverno_pkg_metrics.MetricsConfigManager, t github_com_kyverno_kyverno_pkg_metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.KyvernoV2beta1Interface {
	return &wrappedKyvernoV2beta1{inner, metrics, t}
}
func (c *wrappedKyvernoV2beta1) ClusterPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.ClusterPolicyInterface {
	recorder := github_com_kyverno_kyverno_pkg_metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterPolicy", c.clientType)
	return newKyvernoV2beta1ClusterPolicies(c.inner.ClusterPolicies(), recorder)
}
func (c *wrappedKyvernoV2beta1) Policies(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.PolicyInterface {
	recorder := github_com_kyverno_kyverno_pkg_metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "Policy", c.clientType)
	return newKyvernoV2beta1Policies(c.inner.Policies(namespace), recorder)
}
func (c *wrappedKyvernoV2beta1) PolicyExceptions(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.PolicyExceptionInterface {
	recorder := github_com_kyverno_kyverno_pkg_metrics.ClusteredClientQueryRecorder(c.metrics, "PolicyException", c.clientType)
	return newKyvernoV2beta1PolicyExceptions(c.inner.PolicyExceptions(namespace), recorder)
}

// RESTClient is NOT instrumented
func (c *wrappedKyvernoV2beta1) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedWgpolicyk8sV1alpha2 wrapper
type wrappedWgpolicyk8sV1alpha2 struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface
	metrics    github_com_kyverno_kyverno_pkg_metrics.MetricsConfigManager
	clientType github_com_kyverno_kyverno_pkg_metrics.ClientType
}

func newWgpolicyk8sV1alpha2(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface, metrics github_com_kyverno_kyverno_pkg_metrics.MetricsConfigManager, t github_com_kyverno_kyverno_pkg_metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface {
	return &wrappedWgpolicyk8sV1alpha2{inner, metrics, t}
}
func (c *wrappedWgpolicyk8sV1alpha2) ClusterPolicyReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.ClusterPolicyReportInterface {
	recorder := github_com_kyverno_kyverno_pkg_metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterPolicyReport", c.clientType)
	return newWgpolicyk8sV1alpha2ClusterPolicyReports(c.inner.ClusterPolicyReports(), recorder)
}
func (c *wrappedWgpolicyk8sV1alpha2) PolicyReports(namespace string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.PolicyReportInterface {
	recorder := github_com_kyverno_kyverno_pkg_metrics.NamespacedClientQueryRecorder(c.metrics, namespace, "PolicyReport", c.clientType)
	return newWgpolicyk8sV1alpha2PolicyReports(c.inner.PolicyReports(namespace), recorder)
}

// RESTClient is NOT instrumented
func (c *wrappedWgpolicyk8sV1alpha2) RESTClient() k8s_io_client_go_rest.Interface {
	return c.inner.RESTClient()
}

// wrappedKyvernoV1ClusterPolicies wrapper
type wrappedKyvernoV1ClusterPolicies struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.ClusterPolicyInterface
	recorder github_com_kyverno_kyverno_pkg_metrics.Recorder
}

func newKyvernoV1ClusterPolicies(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.ClusterPolicyInterface, recorder github_com_kyverno_kyverno_pkg_metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.ClusterPolicyInterface {
	return &wrappedKyvernoV1ClusterPolicies{inner, recorder}
}
func (c *wrappedKyvernoV1ClusterPolicies) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1ClusterPolicies) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1ClusterPolicies) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1ClusterPolicies) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1ClusterPolicies) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicyList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1ClusterPolicies) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1ClusterPolicies) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1ClusterPolicies) UpdateStatus(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1ClusterPolicies) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

// wrappedKyvernoV1GenerateRequests wrapper
type wrappedKyvernoV1GenerateRequests struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.GenerateRequestInterface
	recorder github_com_kyverno_kyverno_pkg_metrics.Recorder
}

func newKyvernoV1GenerateRequests(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.GenerateRequestInterface, recorder github_com_kyverno_kyverno_pkg_metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.GenerateRequestInterface {
	return &wrappedKyvernoV1GenerateRequests{inner, recorder}
}
func (c *wrappedKyvernoV1GenerateRequests) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1GenerateRequests) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1GenerateRequests) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1GenerateRequests) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1GenerateRequests) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequestList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1GenerateRequests) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1GenerateRequests) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1GenerateRequests) UpdateStatus(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1GenerateRequests) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

// wrappedKyvernoV1Policies wrapper
type wrappedKyvernoV1Policies struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.PolicyInterface
	recorder github_com_kyverno_kyverno_pkg_metrics.Recorder
}

func newKyvernoV1Policies(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.PolicyInterface, recorder github_com_kyverno_kyverno_pkg_metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.PolicyInterface {
	return &wrappedKyvernoV1Policies{inner, recorder}
}
func (c *wrappedKyvernoV1Policies) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.Policy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.Policy, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1Policies) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1Policies) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1Policies) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.Policy, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1Policies) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.PolicyList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1Policies) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1.Policy, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1Policies) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.Policy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.Policy, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1Policies) UpdateStatus(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.Policy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.Policy, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1Policies) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

// wrappedKyvernoV1alpha1CleanupPolicies wrapper
type wrappedKyvernoV1alpha1CleanupPolicies struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.CleanupPolicyInterface
	recorder github_com_kyverno_kyverno_pkg_metrics.Recorder
}

func newKyvernoV1alpha1CleanupPolicies(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.CleanupPolicyInterface, recorder github_com_kyverno_kyverno_pkg_metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.CleanupPolicyInterface {
	return &wrappedKyvernoV1alpha1CleanupPolicies{inner, recorder}
}
func (c *wrappedKyvernoV1alpha1CleanupPolicies) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1CleanupPolicies) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1CleanupPolicies) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1CleanupPolicies) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1CleanupPolicies) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicyList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1alpha1CleanupPolicies) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1alpha1CleanupPolicies) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1CleanupPolicies) UpdateStatus(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1CleanupPolicies) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

// wrappedKyvernoV1alpha1ClusterCleanupPolicies wrapper
type wrappedKyvernoV1alpha1ClusterCleanupPolicies struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.ClusterCleanupPolicyInterface
	recorder github_com_kyverno_kyverno_pkg_metrics.Recorder
}

func newKyvernoV1alpha1ClusterCleanupPolicies(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.ClusterCleanupPolicyInterface, recorder github_com_kyverno_kyverno_pkg_metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.ClusterCleanupPolicyInterface {
	return &wrappedKyvernoV1alpha1ClusterCleanupPolicies{inner, recorder}
}
func (c *wrappedKyvernoV1alpha1ClusterCleanupPolicies) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1ClusterCleanupPolicies) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1ClusterCleanupPolicies) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1ClusterCleanupPolicies) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1ClusterCleanupPolicies) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicyList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1alpha1ClusterCleanupPolicies) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1alpha1ClusterCleanupPolicies) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1ClusterCleanupPolicies) UpdateStatus(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1ClusterCleanupPolicies) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

// wrappedKyvernoV1alpha2AdmissionReports wrapper
type wrappedKyvernoV1alpha2AdmissionReports struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.AdmissionReportInterface
	recorder github_com_kyverno_kyverno_pkg_metrics.Recorder
}

func newKyvernoV1alpha2AdmissionReports(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.AdmissionReportInterface, recorder github_com_kyverno_kyverno_pkg_metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.AdmissionReportInterface {
	return &wrappedKyvernoV1alpha2AdmissionReports{inner, recorder}
}
func (c *wrappedKyvernoV1alpha2AdmissionReports) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.AdmissionReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.AdmissionReport, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2AdmissionReports) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2AdmissionReports) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2AdmissionReports) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.AdmissionReport, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2AdmissionReports) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.AdmissionReportList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1alpha2AdmissionReports) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.AdmissionReport, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1alpha2AdmissionReports) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.AdmissionReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.AdmissionReport, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2AdmissionReports) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

// wrappedKyvernoV1alpha2BackgroundScanReports wrapper
type wrappedKyvernoV1alpha2BackgroundScanReports struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.BackgroundScanReportInterface
	recorder github_com_kyverno_kyverno_pkg_metrics.Recorder
}

func newKyvernoV1alpha2BackgroundScanReports(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.BackgroundScanReportInterface, recorder github_com_kyverno_kyverno_pkg_metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.BackgroundScanReportInterface {
	return &wrappedKyvernoV1alpha2BackgroundScanReports{inner, recorder}
}
func (c *wrappedKyvernoV1alpha2BackgroundScanReports) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.BackgroundScanReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.BackgroundScanReport, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2BackgroundScanReports) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2BackgroundScanReports) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2BackgroundScanReports) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.BackgroundScanReport, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2BackgroundScanReports) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.BackgroundScanReportList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1alpha2BackgroundScanReports) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.BackgroundScanReport, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1alpha2BackgroundScanReports) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.BackgroundScanReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.BackgroundScanReport, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2BackgroundScanReports) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

// wrappedKyvernoV1alpha2ClusterAdmissionReports wrapper
type wrappedKyvernoV1alpha2ClusterAdmissionReports struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterAdmissionReportInterface
	recorder github_com_kyverno_kyverno_pkg_metrics.Recorder
}

func newKyvernoV1alpha2ClusterAdmissionReports(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterAdmissionReportInterface, recorder github_com_kyverno_kyverno_pkg_metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterAdmissionReportInterface {
	return &wrappedKyvernoV1alpha2ClusterAdmissionReports{inner, recorder}
}
func (c *wrappedKyvernoV1alpha2ClusterAdmissionReports) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterAdmissionReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterAdmissionReport, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2ClusterAdmissionReports) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2ClusterAdmissionReports) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2ClusterAdmissionReports) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterAdmissionReport, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2ClusterAdmissionReports) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterAdmissionReportList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1alpha2ClusterAdmissionReports) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterAdmissionReport, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1alpha2ClusterAdmissionReports) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterAdmissionReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterAdmissionReport, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2ClusterAdmissionReports) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

// wrappedKyvernoV1alpha2ClusterBackgroundScanReports wrapper
type wrappedKyvernoV1alpha2ClusterBackgroundScanReports struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterBackgroundScanReportInterface
	recorder github_com_kyverno_kyverno_pkg_metrics.Recorder
}

func newKyvernoV1alpha2ClusterBackgroundScanReports(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterBackgroundScanReportInterface, recorder github_com_kyverno_kyverno_pkg_metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterBackgroundScanReportInterface {
	return &wrappedKyvernoV1alpha2ClusterBackgroundScanReports{inner, recorder}
}
func (c *wrappedKyvernoV1alpha2ClusterBackgroundScanReports) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterBackgroundScanReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterBackgroundScanReport, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2ClusterBackgroundScanReports) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2ClusterBackgroundScanReports) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2ClusterBackgroundScanReports) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterBackgroundScanReport, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2ClusterBackgroundScanReports) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterBackgroundScanReportList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1alpha2ClusterBackgroundScanReports) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterBackgroundScanReport, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1alpha2ClusterBackgroundScanReports) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterBackgroundScanReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterBackgroundScanReport, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2ClusterBackgroundScanReports) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

// wrappedKyvernoV1beta1UpdateRequests wrapper
type wrappedKyvernoV1beta1UpdateRequests struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.UpdateRequestInterface
	recorder github_com_kyverno_kyverno_pkg_metrics.Recorder
}

func newKyvernoV1beta1UpdateRequests(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.UpdateRequestInterface, recorder github_com_kyverno_kyverno_pkg_metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.UpdateRequestInterface {
	return &wrappedKyvernoV1beta1UpdateRequests{inner, recorder}
}
func (c *wrappedKyvernoV1beta1UpdateRequests) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1beta1UpdateRequests) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1beta1UpdateRequests) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1beta1UpdateRequests) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1beta1UpdateRequests) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequestList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1beta1UpdateRequests) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1beta1UpdateRequests) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1beta1UpdateRequests) UpdateStatus(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1beta1UpdateRequests) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

// wrappedKyvernoV2beta1ClusterPolicies wrapper
type wrappedKyvernoV2beta1ClusterPolicies struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.ClusterPolicyInterface
	recorder github_com_kyverno_kyverno_pkg_metrics.Recorder
}

func newKyvernoV2beta1ClusterPolicies(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.ClusterPolicyInterface, recorder github_com_kyverno_kyverno_pkg_metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.ClusterPolicyInterface {
	return &wrappedKyvernoV2beta1ClusterPolicies{inner, recorder}
}
func (c *wrappedKyvernoV2beta1ClusterPolicies) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v2beta1.ClusterPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v2beta1.ClusterPolicy, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV2beta1ClusterPolicies) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV2beta1ClusterPolicies) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV2beta1ClusterPolicies) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v2beta1.ClusterPolicy, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV2beta1ClusterPolicies) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v2beta1.ClusterPolicyList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV2beta1ClusterPolicies) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v2beta1.ClusterPolicy, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV2beta1ClusterPolicies) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v2beta1.ClusterPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v2beta1.ClusterPolicy, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV2beta1ClusterPolicies) UpdateStatus(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v2beta1.ClusterPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v2beta1.ClusterPolicy, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV2beta1ClusterPolicies) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

// wrappedKyvernoV2beta1Policies wrapper
type wrappedKyvernoV2beta1Policies struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.PolicyInterface
	recorder github_com_kyverno_kyverno_pkg_metrics.Recorder
}

func newKyvernoV2beta1Policies(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.PolicyInterface, recorder github_com_kyverno_kyverno_pkg_metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.PolicyInterface {
	return &wrappedKyvernoV2beta1Policies{inner, recorder}
}
func (c *wrappedKyvernoV2beta1Policies) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v2beta1.Policy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v2beta1.Policy, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV2beta1Policies) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV2beta1Policies) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV2beta1Policies) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v2beta1.Policy, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV2beta1Policies) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v2beta1.PolicyList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV2beta1Policies) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v2beta1.Policy, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV2beta1Policies) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v2beta1.Policy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v2beta1.Policy, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV2beta1Policies) UpdateStatus(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v2beta1.Policy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v2beta1.Policy, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV2beta1Policies) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

// wrappedKyvernoV2beta1PolicyExceptions wrapper
type wrappedKyvernoV2beta1PolicyExceptions struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.PolicyExceptionInterface
	recorder github_com_kyverno_kyverno_pkg_metrics.Recorder
}

func newKyvernoV2beta1PolicyExceptions(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.PolicyExceptionInterface, recorder github_com_kyverno_kyverno_pkg_metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v2beta1.PolicyExceptionInterface {
	return &wrappedKyvernoV2beta1PolicyExceptions{inner, recorder}
}
func (c *wrappedKyvernoV2beta1PolicyExceptions) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v2beta1.PolicyException, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v2beta1.PolicyException, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV2beta1PolicyExceptions) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV2beta1PolicyExceptions) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV2beta1PolicyExceptions) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v2beta1.PolicyException, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV2beta1PolicyExceptions) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v2beta1.PolicyExceptionList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV2beta1PolicyExceptions) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v2beta1.PolicyException, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV2beta1PolicyExceptions) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v2beta1.PolicyException, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v2beta1.PolicyException, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV2beta1PolicyExceptions) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

// wrappedWgpolicyk8sV1alpha2ClusterPolicyReports wrapper
type wrappedWgpolicyk8sV1alpha2ClusterPolicyReports struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.ClusterPolicyReportInterface
	recorder github_com_kyverno_kyverno_pkg_metrics.Recorder
}

func newWgpolicyk8sV1alpha2ClusterPolicyReports(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.ClusterPolicyReportInterface, recorder github_com_kyverno_kyverno_pkg_metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.ClusterPolicyReportInterface {
	return &wrappedWgpolicyk8sV1alpha2ClusterPolicyReports{inner, recorder}
}
func (c *wrappedWgpolicyk8sV1alpha2ClusterPolicyReports) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_policyreport_v1alpha2.ClusterPolicyReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.ClusterPolicyReport, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2ClusterPolicyReports) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2ClusterPolicyReports) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2ClusterPolicyReports) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.ClusterPolicyReport, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2ClusterPolicyReports) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.ClusterPolicyReportList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedWgpolicyk8sV1alpha2ClusterPolicyReports) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.ClusterPolicyReport, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedWgpolicyk8sV1alpha2ClusterPolicyReports) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_policyreport_v1alpha2.ClusterPolicyReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.ClusterPolicyReport, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2ClusterPolicyReports) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

// wrappedWgpolicyk8sV1alpha2PolicyReports wrapper
type wrappedWgpolicyk8sV1alpha2PolicyReports struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.PolicyReportInterface
	recorder github_com_kyverno_kyverno_pkg_metrics.Recorder
}

func newWgpolicyk8sV1alpha2PolicyReports(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.PolicyReportInterface, recorder github_com_kyverno_kyverno_pkg_metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.PolicyReportInterface {
	return &wrappedWgpolicyk8sV1alpha2PolicyReports{inner, recorder}
}
func (c *wrappedWgpolicyk8sV1alpha2PolicyReports) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_policyreport_v1alpha2.PolicyReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.PolicyReport, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2PolicyReports) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2PolicyReports) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2PolicyReports) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.PolicyReport, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2PolicyReports) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.PolicyReportList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedWgpolicyk8sV1alpha2PolicyReports) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.PolicyReport, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedWgpolicyk8sV1alpha2PolicyReports) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_policyreport_v1alpha2.PolicyReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.PolicyReport, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2PolicyReports) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}
