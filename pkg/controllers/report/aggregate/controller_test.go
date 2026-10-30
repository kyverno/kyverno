package aggregate_test

import (
	"context"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	metaclient "github.com/kyverno/kyverno/pkg/clients/metadata"
	"github.com/kyverno/kyverno/pkg/controllers/report/aggregate"
	"github.com/kyverno/kyverno/pkg/openreports"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	orfake "github.com/openreports/reports-api/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	metafake "k8s.io/client-go/metadata/fake"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
	k8stesting "k8s.io/client-go/testing"
)

var (
	kyvernoPolr = &v1alpha2.PolicyReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kyverno-polr",
			Namespace: "default",
			Labels: map[string]string{
				kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp,
			},
		},
	}
	notKyvernoPolr = &v1alpha2.PolicyReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "not-kyverno-polr",
			Namespace: "default",
		},
	}
	pod = &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			UID:       "pod-id-12345",
			Labels: map[string]string{
				"not-app": "something",
			},
		},
	}
	vpol = &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "require-app-label",
		},
	}
	ephr = &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ephemeralreport-pod",
			Labels: map[string]string{
				"app.kubernetes.io/managed-by":        "kyverno",
				"audit.kyverno.io/resource.kind":      "Pod",
				"audit.kyverno.io/resource.name":      "test-pod",
				"audit.kyverno.io/resource.namespace": "default",
				"audit.kyverno.io/resource.version":   "v1",
				"audit.kyverno.io/source":             "background-scan",
				"vpol.kyverno.io/require-app-label":   "test-hash",
				"audit.kyverno.io/resource.uid":       "pod-id-12345",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "test-pod",
					UID:        "pod-id-12345",
				},
			},
		},
		Spec: reportsv1.EphemeralReportSpec{
			Results: []openreportsv1alpha1.ReportResult{
				{
					Description: "validation error: Pods must have a 'app' label",
					Policy:      "require-app-label",
					Result:      openreportsv1alpha1.Result(openreports.StatusFail),
					Scored:      true,
					Source:      reportutils.SourceValidatingPolicy,
					Properties: map[string]string{
						"process": "background scan",
					},
				},
			},
			Summary: openreportsv1alpha1.ReportSummary{
				Fail: 1,
			},
		},
	}
)

func newFakeMetaClient() (metadatainformers.SharedInformerFactory, metafake.MetadataClient, metafake.MetadataClient, metaclient.UpstreamInterface) {
	s := metafake.NewTestScheme()
	metav1.AddMetaToScheme(s)

	client := metafake.NewSimpleMetadataClient(s)

	return metadatainformers.NewSharedInformerFactory(client, 1*time.Minute),
		client.Resource(v1alpha2.SchemeGroupVersion.WithResource("policyreports")).Namespace("default").(metafake.MetadataClient),
		client.Resource(reportsv1.SchemeGroupVersion.WithResource("ephemeralreports")).Namespace("default").(metafake.MetadataClient),
		client
}

func TestController(t *testing.T) {
	metaFactory, metaClient, _, upstream := newFakeMetaClient()
	client := versionedfake.NewSimpleClientset()
	kyvernoFactory := kyvernoinformer.NewSharedInformerFactory(client, 1*time.Second)

	vpolInformer := kyvernoFactory.Policies().V1beta1().ValidatingPolicies()

	client.Wgpolicyk8sV1alpha2().PolicyReports("default").Create(context.TODO(), kyvernoPolr, metav1.CreateOptions{})
	client.Wgpolicyk8sV1alpha2().PolicyReports("default").Create(context.TODO(), notKyvernoPolr, metav1.CreateOptions{})

	metaClient.CreateFake(&metav1.PartialObjectMetadata{ObjectMeta: kyvernoPolr.ObjectMeta}, metav1.CreateOptions{})
	metaClient.CreateFake(&metav1.PartialObjectMetadata{ObjectMeta: notKyvernoPolr.ObjectMeta}, metav1.CreateOptions{})

	controller := aggregate.NewController(client, nil, nil, metaFactory, vpolInformer, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, upstream)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		controller.Run(ctx, 1)
	}()

	stop := make(chan struct{})
	defer close(stop)

	metaFactory.Start(stop)
	kyvernoFactory.Start(stop)

	metaFactory.WaitForCacheSync(stop)
	kyvernoFactory.WaitForCacheSync(stop)

	_, err := client.PoliciesV1beta1().ValidatingPolicies().Create(context.TODO(), &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kyverno-pol",
		},
	}, metav1.CreateOptions{})

	assert.Nil(t, err)

	metaClient.CreateFake(&metav1.PartialObjectMetadata{ObjectMeta: ephr.ObjectMeta}, metav1.CreateOptions{})
	client.ReportsV1().EphemeralReports("default").Create(context.TODO(), ephr, metav1.CreateOptions{})

	// This delay is necessary because the controller processes the queue if a delay of 10 seconds
	// because the controller runs in a goroutine it needs to wait a bit longer to give the controller time to process the queue
	time.Sleep(13 * time.Second)

	list, _ := client.Wgpolicyk8sV1alpha2().PolicyReports("default").List(context.TODO(), metav1.ListOptions{})

	assert.Len(t, list.Items, 1)
	assert.Equal(t, notKyvernoPolr.Name, list.Items[0].Name)

	for _, a := range client.Fake.Actions() {
		if action, ok := a.(k8stesting.GetAction); ok {
			assert.False(t, action.GetName() == notKyvernoPolr.Name, "PolicyReports not managed by kyverno should not be requested")
		}
	}
}

func TestControllerWithOpenreports(t *testing.T) {
	metaFactory, _, metaClient, upstream := newFakeMetaClient()
	client := versionedfake.NewSimpleClientset()
	orClient := orfake.NewSimpleClientset()
	kyvernoFactory := kyvernoinformer.NewSharedInformerFactory(client, 1*time.Second)

	vpolInformer := kyvernoFactory.Policies().V1beta1().ValidatingPolicies()

	// create the ephemeral report that will trigger frontReconcile
	metaClient.CreateFake(&metav1.PartialObjectMetadata{ObjectMeta: ephr.ObjectMeta}, metav1.CreateOptions{})
	client.ReportsV1().EphemeralReports("default").Create(context.TODO(), ephr, metav1.CreateOptions{})

	s := runtime.NewScheme()
	metav1.AddMetaToScheme(s)
	dClient, _ := dclient.NewFakeClient(s, map[schema.GroupVersionResource]string{}, pod)
	dClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))

	controller := aggregate.NewController(client, orClient.OpenreportsV1alpha1(), dClient, metaFactory, vpolInformer, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, upstream)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		controller.Run(ctx, 1)
	}()

	stop := make(chan struct{})
	defer close(stop)

	metaFactory.Start(stop)
	kyvernoFactory.Start(stop)

	metaFactory.WaitForCacheSync(stop)
	kyvernoFactory.WaitForCacheSync(stop)

	// Create the policy for result merging to work
	_, err := client.PoliciesV1beta1().ValidatingPolicies().Create(context.TODO(), vpol, metav1.CreateOptions{})
	assert.Nil(t, err)

	time.Sleep(13 * time.Second)

	list, _ := orClient.OpenreportsV1alpha1().Reports("default").List(context.TODO(), metav1.ListOptions{})
	assert.Len(t, list.Items, 1)
}

// TestControllerWithOpenreports_LegacyResultsPruned proves a legacy kyverno.io
// result carried in an ephemeral report is pruned during aggregation, while a
// CEL (ValidatingPolicy) result in the same report is kept.
func TestControllerWithOpenreports_LegacyResultsPruned(t *testing.T) {
	metaFactory, _, metaClient, upstream := newFakeMetaClient()
	client := versionedfake.NewSimpleClientset()
	orClient := orfake.NewSimpleClientset()
	kyvernoFactory := kyvernoinformer.NewSharedInformerFactory(client, 1*time.Second)

	vpolInformer := kyvernoFactory.Policies().V1beta1().ValidatingPolicies()

	mixedEphr := ephr.DeepCopy()
	mixedEphr.Spec.Results = append(mixedEphr.Spec.Results, openreportsv1alpha1.ReportResult{
		Description: "legacy validation error",
		Policy:      "default/require-app-label",
		Rule:        "check-app-label",
		Result:      openreportsv1alpha1.Result(openreports.StatusFail),
		Scored:      true,
		Source:      reportutils.SourceKyverno,
	})

	metaClient.CreateFake(&metav1.PartialObjectMetadata{ObjectMeta: mixedEphr.ObjectMeta}, metav1.CreateOptions{})
	client.ReportsV1().EphemeralReports("default").Create(context.TODO(), mixedEphr, metav1.CreateOptions{})

	s := runtime.NewScheme()
	metav1.AddMetaToScheme(s)
	dClient, _ := dclient.NewFakeClient(s, map[schema.GroupVersionResource]string{}, pod)
	dClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))

	controller := aggregate.NewController(client, orClient.OpenreportsV1alpha1(), dClient, metaFactory, vpolInformer, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, upstream)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		controller.Run(ctx, 1)
	}()

	stop := make(chan struct{})
	defer close(stop)

	metaFactory.Start(stop)
	kyvernoFactory.Start(stop)

	metaFactory.WaitForCacheSync(stop)
	kyvernoFactory.WaitForCacheSync(stop)

	_, err := client.PoliciesV1beta1().ValidatingPolicies().Create(context.TODO(), vpol, metav1.CreateOptions{})
	assert.Nil(t, err)

	time.Sleep(13 * time.Second)

	list, _ := orClient.OpenreportsV1alpha1().Reports("default").List(context.TODO(), metav1.ListOptions{})
	if assert.Len(t, list.Items, 1) {
		assert.Len(t, list.Items[0].Results, 1)
		assert.Equal(t, reportutils.SourceValidatingPolicy, list.Items[0].Results[0].Source)
	}
}

// TestController_StartupSweepDeletesLegacyOnlyReport proves decision C2: on
// startup, with a legacy kyverno.io ClusterPolicy present, a permanent report
// carrying only legacy results and no ephemeral reports gets swept and deleted.
func TestController_StartupSweepDeletesLegacyOnlyReport(t *testing.T) {
	metaFactory, metaClient, _, upstream := newFakeMetaClient()
	client := versionedfake.NewSimpleClientset()
	kyvernoFactory := kyvernoinformer.NewSharedInformerFactory(client, 1*time.Second)

	vpolInformer := kyvernoFactory.Policies().V1beta1().ValidatingPolicies()

	legacyOnlyPolr := &v1alpha2.PolicyReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "legacy-only-polr",
			Namespace: "default",
			Labels: map[string]string{
				kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp,
			},
		},
		Results: []v1alpha2.PolicyReportResult{
			{Source: reportutils.SourceKyverno, Policy: "require-app-label", Rule: "check-app-label"},
		},
	}
	client.Wgpolicyk8sV1alpha2().PolicyReports("default").Create(context.TODO(), legacyOnlyPolr, metav1.CreateOptions{})
	metaClient.CreateFake(&metav1.PartialObjectMetadata{ObjectMeta: legacyOnlyPolr.ObjectMeta}, metav1.CreateOptions{})

	// seed a legacy ClusterPolicy in the metadata client so the startup sweep gate opens
	cpolGVR := schema.GroupVersionResource{Group: "kyverno.io", Version: "v1", Resource: "clusterpolicies"}
	_, err := upstream.Resource(cpolGVR).(metafake.MetadataClient).CreateFake(&metav1.PartialObjectMetadata{
		ObjectMeta: metav1.ObjectMeta{Name: "legacy-cpol"},
	}, metav1.CreateOptions{})
	assert.Nil(t, err)

	controller := aggregate.NewController(client, nil, nil, metaFactory, vpolInformer, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, upstream)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan struct{})
	defer close(stop)

	metaFactory.Start(stop)
	kyvernoFactory.Start(stop)

	metaFactory.WaitForCacheSync(stop)
	kyvernoFactory.WaitForCacheSync(stop)

	// production order: caches synced before Run starts the startup sweep
	go func() {
		controller.Run(ctx, 1)
	}()

	time.Sleep(13 * time.Second)

	list, _ := client.Wgpolicyk8sV1alpha2().PolicyReports("default").List(context.TODO(), metav1.ListOptions{})
	for _, item := range list.Items {
		assert.NotEqual(t, legacyOnlyPolr.Name, item.Name, "legacy-only report should have been swept and deleted")
	}
}

// TestController_StartupSweepSkippedWithoutLegacyPolicies proves the startup
// sweep stays off (decision C2) when no legacy kyverno.io policy exists, so a
// legacy-only report survives untouched.
func TestController_StartupSweepSkippedWithoutLegacyPolicies(t *testing.T) {
	metaFactory, metaClient, _, upstream := newFakeMetaClient()
	client := versionedfake.NewSimpleClientset()
	kyvernoFactory := kyvernoinformer.NewSharedInformerFactory(client, 1*time.Second)

	vpolInformer := kyvernoFactory.Policies().V1beta1().ValidatingPolicies()

	legacyOnlyPolr := &v1alpha2.PolicyReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "legacy-only-polr-2",
			Namespace: "default",
			Labels: map[string]string{
				kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp,
			},
		},
		Results: []v1alpha2.PolicyReportResult{
			{Source: reportutils.SourceKyverno, Policy: "require-app-label", Rule: "check-app-label"},
		},
	}
	client.Wgpolicyk8sV1alpha2().PolicyReports("default").Create(context.TODO(), legacyOnlyPolr, metav1.CreateOptions{})
	metaClient.CreateFake(&metav1.PartialObjectMetadata{ObjectMeta: legacyOnlyPolr.ObjectMeta}, metav1.CreateOptions{})

	controller := aggregate.NewController(client, nil, nil, metaFactory, vpolInformer, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, upstream)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan struct{})
	defer close(stop)

	metaFactory.Start(stop)
	kyvernoFactory.Start(stop)

	metaFactory.WaitForCacheSync(stop)
	kyvernoFactory.WaitForCacheSync(stop)

	go func() {
		controller.Run(ctx, 1)
	}()

	time.Sleep(13 * time.Second)

	list, _ := client.Wgpolicyk8sV1alpha2().PolicyReports("default").List(context.TODO(), metav1.ListOptions{})
	var found bool
	for _, item := range list.Items {
		if item.Name == legacyOnlyPolr.Name {
			found = true
		}
	}
	assert.True(t, found, "legacy-only report should survive when no legacy policies are present")

	for _, a := range client.Fake.Actions() {
		if action, ok := a.(k8stesting.GetAction); ok {
			assert.NotEqual(t, legacyOnlyPolr.Name, action.GetName(), "the startup sweep should not have touched the report")
		}
	}
}
