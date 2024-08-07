package aggregate_test

import (
	"context"
	"testing"
	"time"

	"github.com/kyverno/kyverno/api/kyverno"
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/controllers/report/aggregate"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metafake "k8s.io/client-go/metadata/fake"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
	k8stesting "k8s.io/client-go/testing"
)

func newFakeMetaClient() (metadatainformers.SharedInformerFactory, metafake.MetadataClient) {
	s := metafake.NewTestScheme()
	metav1.AddMetaToScheme(s)

	client := metafake.NewSimpleMetadataClient(s)

	return metadatainformers.NewSharedInformerFactory(client, 1*time.Minute), client.Resource(v1alpha2.SchemeGroupVersion.WithResource("policyreports")).Namespace("default").(metafake.MetadataClient)
}

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
)

func TestController(t *testing.T) {
	metaFactory, metaClient := newFakeMetaClient()
	client := versionedfake.NewSimpleClientset()
	kyvernoFactory := kyvernoinformer.NewSharedInformerFactory(client, 1*time.Second)

	polInformer := kyvernoFactory.Kyverno().V1().Policies()
	cpolInformer := kyvernoFactory.Kyverno().V1().ClusterPolicies()

	client.Wgpolicyk8sV1alpha2().PolicyReports("default").Create(context.TODO(), kyvernoPolr, metav1.CreateOptions{})
	client.Wgpolicyk8sV1alpha2().PolicyReports("default").Create(context.TODO(), notKyvernoPolr, metav1.CreateOptions{})

	metaClient.CreateFake(&metav1.PartialObjectMetadata{ObjectMeta: kyvernoPolr.ObjectMeta}, metav1.CreateOptions{})
	metaClient.CreateFake(&metav1.PartialObjectMetadata{ObjectMeta: notKyvernoPolr.ObjectMeta}, metav1.CreateOptions{})

	controller := aggregate.NewController(client, nil, metaFactory, polInformer, cpolInformer, nil)

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
