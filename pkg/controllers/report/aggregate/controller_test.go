package aggregate_test

import (
	"context"
	"testing"
	"time"

	"github.com/kyverno/kyverno/api/kyverno"
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/openreports"
	openreportsv1alpha1 "openreports.io/apis/openreports.io/v1alpha1"
	orfake "openreports.io/pkg/client/clientset/versioned/fake"

	"github.com/kyverno/kyverno/pkg/controllers/report/aggregate"
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
	policy = &v1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "require-app-label",
		},
		Spec: v1.Spec{
			ValidationFailureAction: v1.Enforce,
			Rules: []v1.Rule{
				{
					Name: "check-app-label",
					MatchResources: v1.MatchResources{
						ResourceDescription: v1.ResourceDescription{
							Kinds: []string{"Pod"},
						},
					},
				},
			},
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
				"pol.kyverno.io/require-app-label":    "test-hash",
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
					Policy:      "default/require-app-label",
					Rule:        "check-app-label",
					Result:      openreportsv1alpha1.Result(openreports.StatusFail),
					Scored:      true,
					Source:      "kyverno",
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
	rep = &openreportsv1alpha1.Report{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-report-pod",
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "kyverno",
			},
		},
		Scope: &corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Pod",
			Name:       "test-pod",
			Namespace:  "default",
		},
		Results: []openreportsv1alpha1.ReportResult{
			{
				Description: "validation error: Pods must have a 'app' label",
				Policy:      "default/require-app-label",
				Rule:        "check-app-label",
				Result:      openreports.StatusFail,
				Scored:      true,
				Source:      "kyverno",
				Properties: map[string]string{
					"process": "background scan",
				},
			},
		},
		Summary: openreportsv1alpha1.ReportSummary{
			Fail: 1,
		},
	}
)

func newFakeMetaClient() (metadatainformers.SharedInformerFactory, metafake.MetadataClient, metafake.MetadataClient) {
	s := metafake.NewTestScheme()
	metav1.AddMetaToScheme(s)

	client := metafake.NewSimpleMetadataClient(s)

	return metadatainformers.NewSharedInformerFactory(client, 1*time.Minute),
		client.Resource(v1alpha2.SchemeGroupVersion.WithResource("policyreports")).Namespace("default").(metafake.MetadataClient),
		client.Resource(reportsv1.SchemeGroupVersion.WithResource("ephemeralreports")).Namespace("default").(metafake.MetadataClient)
}

func TestController(t *testing.T) {
	metaFactory, metaClient, _ := newFakeMetaClient()
	client := versionedfake.NewSimpleClientset()
	kyvernoFactory := kyvernoinformer.NewSharedInformerFactory(client, 1*time.Second)

	polInformer := kyvernoFactory.Kyverno().V1().Policies()
	cpolInformer := kyvernoFactory.Kyverno().V1().ClusterPolicies()

	client.Wgpolicyk8sV1alpha2().PolicyReports("default").Create(context.TODO(), kyvernoPolr, metav1.CreateOptions{})
	client.Wgpolicyk8sV1alpha2().PolicyReports("default").Create(context.TODO(), notKyvernoPolr, metav1.CreateOptions{})

	metaClient.CreateFake(&metav1.PartialObjectMetadata{ObjectMeta: kyvernoPolr.ObjectMeta}, metav1.CreateOptions{})
	metaClient.CreateFake(&metav1.PartialObjectMetadata{ObjectMeta: notKyvernoPolr.ObjectMeta}, metav1.CreateOptions{})

	controller := aggregate.NewController(client, nil, nil, metaFactory, polInformer, cpolInformer, nil, nil, nil, nil, nil, nil)

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

	_, err := client.KyvernoV1().ClusterPolicies().Create(context.TODO(), &v1.ClusterPolicy{
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
	metaFactory, _, metaClient := newFakeMetaClient()
	client := versionedfake.NewSimpleClientset()
	orClient := orfake.NewSimpleClientset()
	kyvernoFactory := kyvernoinformer.NewSharedInformerFactory(client, 1*time.Second)

	polInformer := kyvernoFactory.Kyverno().V1().Policies()
	cpolInformer := kyvernoFactory.Kyverno().V1().ClusterPolicies()

	// create the ephemeral report that will trigger frontReconcile
	metaClient.CreateFake(&metav1.PartialObjectMetadata{ObjectMeta: ephr.ObjectMeta}, metav1.CreateOptions{})
	client.ReportsV1().EphemeralReports("default").Create(context.TODO(), ephr, metav1.CreateOptions{})

	s := runtime.NewScheme()
	metav1.AddMetaToScheme(s)
	dClient, _ := dclient.NewFakeClient(s, map[schema.GroupVersionResource]string{}, pod)
	dClient.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))

	controller := aggregate.NewController(client, orClient.OpenreportsV1alpha1(), dClient, metaFactory, polInformer, cpolInformer, nil, nil, nil, nil, nil, nil)

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
	_, err := client.KyvernoV1().Policies("default").Create(context.TODO(), policy, metav1.CreateOptions{})
	assert.Nil(t, err)

	time.Sleep(13 * time.Second)

	list, _ := orClient.OpenreportsV1alpha1().Reports("default").List(context.TODO(), metav1.ListOptions{})
	assert.Len(t, list.Items, 1)
}
