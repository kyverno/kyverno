package client

import (
	context "context"

	github_com_kyverno_kyverno_api_kyverno_v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	github_com_kyverno_kyverno_api_kyverno_v1alpha1 "github.com/kyverno/kyverno/api/kyverno/v1alpha1"
	github_com_kyverno_kyverno_api_kyverno_v1alpha2 "github.com/kyverno/kyverno/api/kyverno/v1alpha2"
	github_com_kyverno_kyverno_api_kyverno_v1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	github_com_kyverno_kyverno_api_policyreport_v1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	versioned "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha1"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha2"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1beta1"
	github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/policyreport/v1alpha2"
	metrics "github.com/kyverno/kyverno/pkg/metrics"
	k8s_io_apimachinery_pkg_apis_meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_io_apimachinery_pkg_types "k8s.io/apimachinery/pkg/types"
	types "k8s.io/apimachinery/pkg/types"
	k8s_io_apimachinery_pkg_watch "k8s.io/apimachinery/pkg/watch"
	watch "k8s.io/apimachinery/pkg/watch"
	discovery "k8s.io/client-go/discovery"
	restclient "k8s.io/client-go/rest"
)

type clientset struct {
	inner               versioned.Interface
	kyvernov1           github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.KyvernoV1Interface
	kyvernov1alpha1     github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.KyvernoV1alpha1Interface
	kyvernov1alpha2     github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface
	kyvernov1beta1      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface
	wgpolicyk8sv1alpha2 github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface
}

func (c *clientset) Discovery() discovery.DiscoveryInterface {
	return c.inner.Discovery()
}

func Wrap(inner versioned.Interface, m metrics.MetricsConfigManager, t metrics.ClientType) versioned.Interface {
	return &clientset{
		inner:               inner,
		kyvernov1:           wrapKyvernoV1Interface(inner.KyvernoV1(), m, t),
		kyvernov1alpha1:     wrapKyvernoV1alpha1Interface(inner.KyvernoV1alpha1(), m, t),
		kyvernov1alpha2:     wrapKyvernoV1alpha2Interface(inner.KyvernoV1alpha2(), m, t),
		kyvernov1beta1:      wrapKyvernoV1beta1Interface(inner.KyvernoV1beta1(), m, t),
		wgpolicyk8sv1alpha2: wrapWgpolicyk8sV1alpha2Interface(inner.Wgpolicyk8sV1alpha2(), m, t),
	}
}

func NewForConfig(c *restclient.Config, m metrics.MetricsConfigManager, t metrics.ClientType) (versioned.Interface, error) {
	inner, err := versioned.NewForConfig(c)
	if err != nil {
		return nil, err
	}
	return Wrap(inner, m, t), nil
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

type wrappedKyvernoV1Interface struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.KyvernoV1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func wrapKyvernoV1Interface(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.KyvernoV1Interface, metrics metrics.MetricsConfigManager, t metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.KyvernoV1Interface {
	return &wrappedKyvernoV1Interface{inner, metrics, t}
}

type wrappedKyvernoV1InterfaceClusterPolicyInterface struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.ClusterPolicyInterface
	recorder metrics.Recorder
}

func wrapKyvernoV1InterfaceClusterPolicyInterface(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.ClusterPolicyInterface, recorder metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.ClusterPolicyInterface {
	return &wrappedKyvernoV1InterfaceClusterPolicyInterface{inner, recorder}
}
func (c *wrappedKyvernoV1InterfaceClusterPolicyInterface) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, arg2 metav1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1InterfaceClusterPolicyInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1InterfaceClusterPolicyInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1InterfaceClusterPolicyInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1InterfaceClusterPolicyInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicyList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1InterfaceClusterPolicyInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1InterfaceClusterPolicyInterface) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1InterfaceClusterPolicyInterface) UpdateStatus(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, arg2 metav1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.ClusterPolicy, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1InterfaceClusterPolicyInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedKyvernoV1Interface) ClusterPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.ClusterPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterPolicy", c.clientType)
	return wrapKyvernoV1InterfaceClusterPolicyInterface(c.inner.ClusterPolicies(), recorder)
}

type wrappedKyvernoV1InterfaceGenerateRequestInterface struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.GenerateRequestInterface
	recorder metrics.Recorder
}

func wrapKyvernoV1InterfaceGenerateRequestInterface(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.GenerateRequestInterface, recorder metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.GenerateRequestInterface {
	return &wrappedKyvernoV1InterfaceGenerateRequestInterface{inner, recorder}
}
func (c *wrappedKyvernoV1InterfaceGenerateRequestInterface) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1InterfaceGenerateRequestInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1InterfaceGenerateRequestInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1InterfaceGenerateRequestInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1InterfaceGenerateRequestInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequestList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1InterfaceGenerateRequestInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1InterfaceGenerateRequestInterface) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1InterfaceGenerateRequestInterface) UpdateStatus(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.GenerateRequest, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1InterfaceGenerateRequestInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedKyvernoV1Interface) GenerateRequests(arg0 string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.GenerateRequestInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "GenerateRequest", c.clientType)
	return wrapKyvernoV1InterfaceGenerateRequestInterface(c.inner.GenerateRequests(arg0), recorder)
}

type wrappedKyvernoV1InterfacePolicyInterface struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.PolicyInterface
	recorder metrics.Recorder
}

func wrapKyvernoV1InterfacePolicyInterface(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.PolicyInterface, recorder metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.PolicyInterface {
	return &wrappedKyvernoV1InterfacePolicyInterface{inner, recorder}
}
func (c *wrappedKyvernoV1InterfacePolicyInterface) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.Policy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.Policy, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1InterfacePolicyInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1InterfacePolicyInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1InterfacePolicyInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.Policy, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1InterfacePolicyInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.PolicyList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1InterfacePolicyInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1.Policy, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1InterfacePolicyInterface) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.Policy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.Policy, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1InterfacePolicyInterface) UpdateStatus(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1.Policy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1.Policy, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1InterfacePolicyInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedKyvernoV1Interface) Policies(arg0 string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1.PolicyInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "Policy", c.clientType)
	return wrapKyvernoV1InterfacePolicyInterface(c.inner.Policies(arg0), recorder)
}
func (c *wrappedKyvernoV1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedKyvernoV1alpha1Interface struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.KyvernoV1alpha1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func wrapKyvernoV1alpha1Interface(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.KyvernoV1alpha1Interface, metrics metrics.MetricsConfigManager, t metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.KyvernoV1alpha1Interface {
	return &wrappedKyvernoV1alpha1Interface{inner, metrics, t}
}

type wrappedKyvernoV1alpha1InterfaceCleanupPolicyInterface struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.CleanupPolicyInterface
	recorder metrics.Recorder
}

func wrapKyvernoV1alpha1InterfaceCleanupPolicyInterface(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.CleanupPolicyInterface, recorder metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.CleanupPolicyInterface {
	return &wrappedKyvernoV1alpha1InterfaceCleanupPolicyInterface{inner, recorder}
}
func (c *wrappedKyvernoV1alpha1InterfaceCleanupPolicyInterface) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1InterfaceCleanupPolicyInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1InterfaceCleanupPolicyInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1InterfaceCleanupPolicyInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1InterfaceCleanupPolicyInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicyList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1alpha1InterfaceCleanupPolicyInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 metav1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1alpha1InterfaceCleanupPolicyInterface) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1InterfaceCleanupPolicyInterface) UpdateStatus(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.CleanupPolicy, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1InterfaceCleanupPolicyInterface) Watch(arg0 context.Context, arg1 metav1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedKyvernoV1alpha1Interface) CleanupPolicies(arg0 string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.CleanupPolicyInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "CleanupPolicy", c.clientType)
	return wrapKyvernoV1alpha1InterfaceCleanupPolicyInterface(c.inner.CleanupPolicies(arg0), recorder)
}

type wrappedKyvernoV1alpha1InterfaceClusterCleanupPolicyInterface struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.ClusterCleanupPolicyInterface
	recorder metrics.Recorder
}

func wrapKyvernoV1alpha1InterfaceClusterCleanupPolicyInterface(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.ClusterCleanupPolicyInterface, recorder metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.ClusterCleanupPolicyInterface {
	return &wrappedKyvernoV1alpha1InterfaceClusterCleanupPolicyInterface{inner, recorder}
}
func (c *wrappedKyvernoV1alpha1InterfaceClusterCleanupPolicyInterface) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1InterfaceClusterCleanupPolicyInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1InterfaceClusterCleanupPolicyInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1InterfaceClusterCleanupPolicyInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1InterfaceClusterCleanupPolicyInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicyList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1alpha1InterfaceClusterCleanupPolicyInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1alpha1InterfaceClusterCleanupPolicyInterface) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1InterfaceClusterCleanupPolicyInterface) UpdateStatus(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha1.ClusterCleanupPolicy, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha1InterfaceClusterCleanupPolicyInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedKyvernoV1alpha1Interface) ClusterCleanupPolicies() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha1.ClusterCleanupPolicyInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterCleanupPolicy", c.clientType)
	return wrapKyvernoV1alpha1InterfaceClusterCleanupPolicyInterface(c.inner.ClusterCleanupPolicies(), recorder)
}
func (c *wrappedKyvernoV1alpha1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedKyvernoV1alpha2Interface struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func wrapKyvernoV1alpha2Interface(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface, metrics metrics.MetricsConfigManager, t metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.KyvernoV1alpha2Interface {
	return &wrappedKyvernoV1alpha2Interface{inner, metrics, t}
}

type wrappedKyvernoV1alpha2InterfaceAdmissionReportInterface struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.AdmissionReportInterface
	recorder metrics.Recorder
}

func wrapKyvernoV1alpha2InterfaceAdmissionReportInterface(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.AdmissionReportInterface, recorder metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.AdmissionReportInterface {
	return &wrappedKyvernoV1alpha2InterfaceAdmissionReportInterface{inner, recorder}
}
func (c *wrappedKyvernoV1alpha2InterfaceAdmissionReportInterface) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.AdmissionReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.AdmissionReport, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2InterfaceAdmissionReportInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2InterfaceAdmissionReportInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2InterfaceAdmissionReportInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.AdmissionReport, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2InterfaceAdmissionReportInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.AdmissionReportList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1alpha2InterfaceAdmissionReportInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.AdmissionReport, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1alpha2InterfaceAdmissionReportInterface) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.AdmissionReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.AdmissionReport, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2InterfaceAdmissionReportInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedKyvernoV1alpha2Interface) AdmissionReports(arg0 string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.AdmissionReportInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "AdmissionReport", c.clientType)
	return wrapKyvernoV1alpha2InterfaceAdmissionReportInterface(c.inner.AdmissionReports(arg0), recorder)
}

type wrappedKyvernoV1alpha2InterfaceBackgroundScanReportInterface struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.BackgroundScanReportInterface
	recorder metrics.Recorder
}

func wrapKyvernoV1alpha2InterfaceBackgroundScanReportInterface(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.BackgroundScanReportInterface, recorder metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.BackgroundScanReportInterface {
	return &wrappedKyvernoV1alpha2InterfaceBackgroundScanReportInterface{inner, recorder}
}
func (c *wrappedKyvernoV1alpha2InterfaceBackgroundScanReportInterface) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.BackgroundScanReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.BackgroundScanReport, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2InterfaceBackgroundScanReportInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2InterfaceBackgroundScanReportInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2InterfaceBackgroundScanReportInterface) Get(arg0 context.Context, arg1 string, arg2 metav1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.BackgroundScanReport, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2InterfaceBackgroundScanReportInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.BackgroundScanReportList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1alpha2InterfaceBackgroundScanReportInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.BackgroundScanReport, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1alpha2InterfaceBackgroundScanReportInterface) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.BackgroundScanReport, arg2 metav1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.BackgroundScanReport, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2InterfaceBackgroundScanReportInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedKyvernoV1alpha2Interface) BackgroundScanReports(arg0 string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.BackgroundScanReportInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "BackgroundScanReport", c.clientType)
	return wrapKyvernoV1alpha2InterfaceBackgroundScanReportInterface(c.inner.BackgroundScanReports(arg0), recorder)
}

type wrappedKyvernoV1alpha2InterfaceClusterAdmissionReportInterface struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterAdmissionReportInterface
	recorder metrics.Recorder
}

func wrapKyvernoV1alpha2InterfaceClusterAdmissionReportInterface(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterAdmissionReportInterface, recorder metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterAdmissionReportInterface {
	return &wrappedKyvernoV1alpha2InterfaceClusterAdmissionReportInterface{inner, recorder}
}
func (c *wrappedKyvernoV1alpha2InterfaceClusterAdmissionReportInterface) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterAdmissionReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterAdmissionReport, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2InterfaceClusterAdmissionReportInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2InterfaceClusterAdmissionReportInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2InterfaceClusterAdmissionReportInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterAdmissionReport, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2InterfaceClusterAdmissionReportInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterAdmissionReportList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1alpha2InterfaceClusterAdmissionReportInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterAdmissionReport, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1alpha2InterfaceClusterAdmissionReportInterface) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterAdmissionReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterAdmissionReport, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2InterfaceClusterAdmissionReportInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedKyvernoV1alpha2Interface) ClusterAdmissionReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterAdmissionReportInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterAdmissionReport", c.clientType)
	return wrapKyvernoV1alpha2InterfaceClusterAdmissionReportInterface(c.inner.ClusterAdmissionReports(), recorder)
}

type wrappedKyvernoV1alpha2InterfaceClusterBackgroundScanReportInterface struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterBackgroundScanReportInterface
	recorder metrics.Recorder
}

func wrapKyvernoV1alpha2InterfaceClusterBackgroundScanReportInterface(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterBackgroundScanReportInterface, recorder metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterBackgroundScanReportInterface {
	return &wrappedKyvernoV1alpha2InterfaceClusterBackgroundScanReportInterface{inner, recorder}
}
func (c *wrappedKyvernoV1alpha2InterfaceClusterBackgroundScanReportInterface) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterBackgroundScanReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterBackgroundScanReport, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2InterfaceClusterBackgroundScanReportInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2InterfaceClusterBackgroundScanReportInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2InterfaceClusterBackgroundScanReportInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterBackgroundScanReport, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2InterfaceClusterBackgroundScanReportInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterBackgroundScanReportList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1alpha2InterfaceClusterBackgroundScanReportInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterBackgroundScanReport, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1alpha2InterfaceClusterBackgroundScanReportInterface) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterBackgroundScanReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1alpha2.ClusterBackgroundScanReport, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1alpha2InterfaceClusterBackgroundScanReportInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedKyvernoV1alpha2Interface) ClusterBackgroundScanReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1alpha2.ClusterBackgroundScanReportInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterBackgroundScanReport", c.clientType)
	return wrapKyvernoV1alpha2InterfaceClusterBackgroundScanReportInterface(c.inner.ClusterBackgroundScanReports(), recorder)
}
func (c *wrappedKyvernoV1alpha2Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedKyvernoV1beta1Interface struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func wrapKyvernoV1beta1Interface(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface, metrics metrics.MetricsConfigManager, t metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.KyvernoV1beta1Interface {
	return &wrappedKyvernoV1beta1Interface{inner, metrics, t}
}

type wrappedKyvernoV1beta1InterfaceUpdateRequestInterface struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.UpdateRequestInterface
	recorder metrics.Recorder
}

func wrapKyvernoV1beta1InterfaceUpdateRequestInterface(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.UpdateRequestInterface, recorder metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.UpdateRequestInterface {
	return &wrappedKyvernoV1beta1InterfaceUpdateRequestInterface{inner, recorder}
}
func (c *wrappedKyvernoV1beta1InterfaceUpdateRequestInterface) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1beta1InterfaceUpdateRequestInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1beta1InterfaceUpdateRequestInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1beta1InterfaceUpdateRequestInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1beta1InterfaceUpdateRequestInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequestList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedKyvernoV1beta1InterfaceUpdateRequestInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedKyvernoV1beta1InterfaceUpdateRequestInterface) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1beta1InterfaceUpdateRequestInterface) UpdateStatus(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_kyverno_v1beta1.UpdateRequest, error) {
	defer c.recorder.Record("update_status")
	return c.inner.UpdateStatus(arg0, arg1, arg2)
}
func (c *wrappedKyvernoV1beta1InterfaceUpdateRequestInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedKyvernoV1beta1Interface) UpdateRequests(arg0 string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_kyverno_v1beta1.UpdateRequestInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "UpdateRequest", c.clientType)
	return wrapKyvernoV1beta1InterfaceUpdateRequestInterface(c.inner.UpdateRequests(arg0), recorder)
}
func (c *wrappedKyvernoV1beta1Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}

type wrappedWgpolicyk8sV1alpha2Interface struct {
	inner      github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface
	metrics    metrics.MetricsConfigManager
	clientType metrics.ClientType
}

func wrapWgpolicyk8sV1alpha2Interface(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface, metrics metrics.MetricsConfigManager, t metrics.ClientType) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.Wgpolicyk8sV1alpha2Interface {
	return &wrappedWgpolicyk8sV1alpha2Interface{inner, metrics, t}
}

type wrappedWgpolicyk8sV1alpha2InterfaceClusterPolicyReportInterface struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.ClusterPolicyReportInterface
	recorder metrics.Recorder
}

func wrapWgpolicyk8sV1alpha2InterfaceClusterPolicyReportInterface(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.ClusterPolicyReportInterface, recorder metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.ClusterPolicyReportInterface {
	return &wrappedWgpolicyk8sV1alpha2InterfaceClusterPolicyReportInterface{inner, recorder}
}
func (c *wrappedWgpolicyk8sV1alpha2InterfaceClusterPolicyReportInterface) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_policyreport_v1alpha2.ClusterPolicyReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.ClusterPolicyReport, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2InterfaceClusterPolicyReportInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2InterfaceClusterPolicyReportInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2InterfaceClusterPolicyReportInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.ClusterPolicyReport, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2InterfaceClusterPolicyReportInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.ClusterPolicyReportList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedWgpolicyk8sV1alpha2InterfaceClusterPolicyReportInterface) Patch(arg0 context.Context, arg1 string, arg2 types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.ClusterPolicyReport, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedWgpolicyk8sV1alpha2InterfaceClusterPolicyReportInterface) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_policyreport_v1alpha2.ClusterPolicyReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.UpdateOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.ClusterPolicyReport, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2InterfaceClusterPolicyReportInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedWgpolicyk8sV1alpha2Interface) ClusterPolicyReports() github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.ClusterPolicyReportInterface {
	recorder := metrics.ClusteredClientQueryRecorder(c.metrics, "ClusterPolicyReport", c.clientType)
	return wrapWgpolicyk8sV1alpha2InterfaceClusterPolicyReportInterface(c.inner.ClusterPolicyReports(), recorder)
}

type wrappedWgpolicyk8sV1alpha2InterfacePolicyReportInterface struct {
	inner    github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.PolicyReportInterface
	recorder metrics.Recorder
}

func wrapWgpolicyk8sV1alpha2InterfacePolicyReportInterface(inner github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.PolicyReportInterface, recorder metrics.Recorder) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.PolicyReportInterface {
	return &wrappedWgpolicyk8sV1alpha2InterfacePolicyReportInterface{inner, recorder}
}
func (c *wrappedWgpolicyk8sV1alpha2InterfacePolicyReportInterface) Create(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_policyreport_v1alpha2.PolicyReport, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.CreateOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.PolicyReport, error) {
	defer c.recorder.Record("create")
	return c.inner.Create(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2InterfacePolicyReportInterface) Delete(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions) error {
	defer c.recorder.Record("delete")
	return c.inner.Delete(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2InterfacePolicyReportInterface) DeleteCollection(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.DeleteOptions, arg2 metav1.ListOptions) error {
	defer c.recorder.Record("delete_collection")
	return c.inner.DeleteCollection(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2InterfacePolicyReportInterface) Get(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_apis_meta_v1.GetOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.PolicyReport, error) {
	defer c.recorder.Record("get")
	return c.inner.Get(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2InterfacePolicyReportInterface) List(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.PolicyReportList, error) {
	defer c.recorder.Record("list")
	return c.inner.List(arg0, arg1)
}
func (c *wrappedWgpolicyk8sV1alpha2InterfacePolicyReportInterface) Patch(arg0 context.Context, arg1 string, arg2 k8s_io_apimachinery_pkg_types.PatchType, arg3 []uint8, arg4 k8s_io_apimachinery_pkg_apis_meta_v1.PatchOptions, arg5 ...string) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.PolicyReport, error) {
	defer c.recorder.Record("patch")
	return c.inner.Patch(arg0, arg1, arg2, arg3, arg4, arg5...)
}
func (c *wrappedWgpolicyk8sV1alpha2InterfacePolicyReportInterface) Update(arg0 context.Context, arg1 *github_com_kyverno_kyverno_api_policyreport_v1alpha2.PolicyReport, arg2 metav1.UpdateOptions) (*github_com_kyverno_kyverno_api_policyreport_v1alpha2.PolicyReport, error) {
	defer c.recorder.Record("update")
	return c.inner.Update(arg0, arg1, arg2)
}
func (c *wrappedWgpolicyk8sV1alpha2InterfacePolicyReportInterface) Watch(arg0 context.Context, arg1 k8s_io_apimachinery_pkg_apis_meta_v1.ListOptions) (k8s_io_apimachinery_pkg_watch.Interface, error) {
	defer c.recorder.Record("watch")
	return c.inner.Watch(arg0, arg1)
}

func (c *wrappedWgpolicyk8sV1alpha2Interface) PolicyReports(arg0 string) github_com_kyverno_kyverno_pkg_client_clientset_versioned_typed_policyreport_v1alpha2.PolicyReportInterface {
	recorder := metrics.NamespacedClientQueryRecorder(c.metrics, arg0, "PolicyReport", c.clientType)
	return wrapWgpolicyk8sV1alpha2InterfacePolicyReportInterface(c.inner.PolicyReports(arg0), recorder)
}
func (c *wrappedWgpolicyk8sV1alpha2Interface) RESTClient() restclient.Interface {
	return c.inner.RESTClient()
}
