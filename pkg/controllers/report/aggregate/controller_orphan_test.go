package aggregate

import (
	"context"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	policyreportv1alpha2 "github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/openreports"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	orfake "github.com/openreports/reports-api/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
)

func TestReconcileOrphanPolicyReport(t *testing.T) {
	for _, tc := range []struct {
		name         string
		resourceUID  types.UID
		scopeUID     types.UID
		owned        bool
		wantDeleted  bool
		wantRepaired bool
	}{
		{name: "live resource", resourceUID: "old-uid", scopeUID: "old-uid", wantRepaired: true},
		{name: "resource recreated", resourceUID: "new-uid", scopeUID: "old-uid", wantDeleted: true},
		{name: "resource deleted", scopeUID: "old-uid", wantDeleted: true},
		{name: "invalid scope", resourceUID: "old-uid", scopeUID: "other-uid"},
		{name: "already owned", scopeUID: "old-uid", owned: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			scope := &corev1.ObjectReference{APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "test-pod", UID: tc.scopeUID}
			report := reportutils.NewPolicyReport("default", "old-uid", scope, false)
			if tc.owned {
				report.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "v1", Kind: "Pod", Name: "test-pod", UID: "old-uid"}})
			}
			client := versionedfake.NewSimpleClientset()
			_, err := client.Wgpolicyk8sV1alpha2().PolicyReports("default").Create(ctx, report.(*openreports.WgpolicyReportAdapter).PolicyReport, metav1.CreateOptions{})
			require.NoError(t, err)

			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))
			var resources []runtime.Object
			if tc.resourceUID != "" {
				resources = append(resources, &corev1.Pod{
					TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
					ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default", UID: tc.resourceUID},
				})
			}
			dynamicClient, err := dclient.NewFakeClient(scheme, map[schema.GroupVersionResource]string{}, resources...)
			require.NoError(t, err)
			dynamicClient.SetDiscovery(dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{{Version: "v1", Resource: "pods"}}))

			controller := controller{client: client, dclient: dynamicClient}
			deleted, err := controller.reconcileOrphanReport(ctx, report)
			require.NoError(t, err)
			require.Equal(t, tc.wantDeleted, deleted)
			if tc.wantRepaired {
				require.Equal(t, []metav1.OwnerReference{{APIVersion: "v1", Kind: "Pod", Name: "test-pod", UID: "old-uid"}}, report.GetOwnerReferences())
			} else if !tc.owned {
				require.Empty(t, report.GetOwnerReferences())
			}
			_, err = client.Wgpolicyk8sV1alpha2().PolicyReports("default").Get(ctx, report.GetName(), metav1.GetOptions{})
			require.Equal(t, tc.wantDeleted, apierrors.IsNotFound(err))
			if !tc.wantDeleted {
				require.NoError(t, err)
			}
		})
	}
}

func TestReconcileOrphanClusterReport(t *testing.T) {
	for _, tc := range []struct {
		name        string
		resourceUID types.UID
		wantDeleted bool
	}{
		{name: "live namespace", resourceUID: "namespace-uid"},
		{name: "recreated namespace", resourceUID: "new-uid", wantDeleted: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			scope := &corev1.ObjectReference{APIVersion: "v1", Kind: "Namespace", Name: "default", UID: "namespace-uid"}
			report := reportutils.NewPolicyReport("", "namespace-uid", scope, true)
			orClient := orfake.NewSimpleClientset()
			_, err := orClient.OpenreportsV1alpha1().ClusterReports().Create(ctx, report.(*openreports.ClusterReportAdapter).ClusterReport, metav1.CreateOptions{})
			require.NoError(t, err)

			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))
			dynamicClient, err := dclient.NewFakeClient(scheme, map[schema.GroupVersionResource]string{}, &corev1.Namespace{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
				ObjectMeta: metav1.ObjectMeta{Name: "default", UID: tc.resourceUID},
			})
			require.NoError(t, err)
			dynamicClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))

			controller := controller{client: versionedfake.NewSimpleClientset(), orClient: orClient.OpenreportsV1alpha1(), dclient: dynamicClient}
			deleted, err := controller.reconcileOrphanReport(ctx, report)
			require.NoError(t, err)
			require.Equal(t, tc.wantDeleted, deleted)
			if !tc.wantDeleted {
				require.Equal(t, []metav1.OwnerReference{{APIVersion: "v1", Kind: "Namespace", Name: "default", UID: "namespace-uid"}}, report.GetOwnerReferences())
			}
			_, err = orClient.OpenreportsV1alpha1().ClusterReports().Get(ctx, report.GetName(), metav1.GetOptions{})
			require.Equal(t, tc.wantDeleted, apierrors.IsNotFound(err))
		})
	}
}

func TestReconcileOrphanDiscoveryErrorPreservesReport(t *testing.T) {
	ctx := context.Background()
	scope := &corev1.ObjectReference{APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "test-pod", UID: "old-uid"}
	report := reportutils.NewPolicyReport("default", "old-uid", scope, false)
	client := versionedfake.NewSimpleClientset()
	_, err := client.Wgpolicyk8sV1alpha2().PolicyReports("default").Create(ctx, report.(*openreports.WgpolicyReportAdapter).PolicyReport, metav1.CreateOptions{})
	require.NoError(t, err)
	dynamicClient := dclient.NewEmptyFakeClient()
	dynamicClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))

	controller := controller{client: client, dclient: dynamicClient}
	deleted, err := controller.reconcileOrphanReport(ctx, report)
	require.Error(t, err)
	require.False(t, deleted)
	require.Empty(t, report.GetOwnerReferences())
	_, err = client.Wgpolicyk8sV1alpha2().PolicyReports("default").Get(ctx, report.GetName(), metav1.GetOptions{})
	require.NoError(t, err)
}

func TestReportScope(t *testing.T) {
	scope := &corev1.ObjectReference{UID: "uid"}
	for _, report := range []reportsv1.ReportInterface{
		openreports.NewWGCpolAdapter(&policyreportv1alpha2.ClusterPolicyReport{Scope: scope}),
		reportutils.NewPolicyReport("default", "uid", scope, false),
		reportutils.NewPolicyReport("default", "uid", scope, true),
		reportutils.NewPolicyReport("", "uid", scope, true),
	} {
		require.Equal(t, scope, reportScope(report))
	}
}

func TestBackReconcileKeepsOwnedReportWithoutEphemeralInputs(t *testing.T) {
	ctx := context.Background()
	policy := &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{Name: "require-app-label", Namespace: "default"},
		Spec:       kyvernov1.Spec{Rules: []kyvernov1.Rule{{Name: "check-app-label"}}},
	}
	policyIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	require.NoError(t, policyIndexer.Add(policy))
	clusterPolicyIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})

	scope := &corev1.ObjectReference{APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "test-pod", UID: "pod-uid"}
	report := reportutils.NewPolicyReport("default", "pod-uid", scope, false)
	report.SetResourceVersion("1")
	report.SetUID("report-uid")
	report.SetOwnerReferences([]metav1.OwnerReference{{APIVersion: "v1", Kind: "Pod", Name: "test-pod", UID: "pod-uid"}})
	reportutils.SetResults(report, openreportsv1alpha1.ReportResult{Source: "kyverno", Policy: "default/require-app-label", Rule: "check-app-label", Result: "pass"})
	client := versionedfake.NewSimpleClientset()
	_, err := client.Wgpolicyk8sV1alpha2().PolicyReports("default").Create(ctx, report.(*openreports.WgpolicyReportAdapter).PolicyReport, metav1.CreateOptions{})
	require.NoError(t, err)

	controller := controller{
		client:                  client,
		polLister:               kyvernov1listers.NewPolicyLister(policyIndexer),
		cpolLister:              kyvernov1listers.NewClusterPolicyLister(clusterPolicyIndexer),
		cacheMu:                 &sync.Mutex{},
		reportUUIDToPolicyCache: map[string]sets.Set[string]{},
	}
	require.NoError(t, controller.backReconcile(ctx, logr.Discard(), "", "default", "pod-uid"))
	retained, err := client.Wgpolicyk8sV1alpha2().PolicyReports("default").Get(ctx, "pod-uid", metav1.GetOptions{})
	require.NoError(t, err)
	require.Len(t, retained.Results, 1)
	require.Equal(t, "pass", string(retained.Results[0].Result))
}
