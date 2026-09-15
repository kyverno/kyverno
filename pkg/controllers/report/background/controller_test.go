package background

import (
	"context"
	"testing"
	"time"

	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	metafake "k8s.io/client-go/metadata/fake"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
)

func newFakeMetaClient() (metadatainformers.SharedInformerFactory, metafake.MetadataClient, metafake.MetadataClient) {
	s := metafake.NewTestScheme()
	metav1.AddMetaToScheme(s)

	client := metafake.NewSimpleMetadataClient(s)

	return metadatainformers.NewSharedInformerFactory(client, 1*time.Minute),
		client.Resource(reportsv1.SchemeGroupVersion.WithResource("ephemeralreports")).Namespace("default").(metafake.MetadataClient),
		client.Resource(reportsv1.SchemeGroupVersion.WithResource("clusterephemeralreports")).(metafake.MetadataClient)
}

func Test_getMeta_and_getReport(t *testing.T) {
	metaFactory, ephrMetaClient, cephrMetaClient := newFakeMetaClient()
	client := versionedfake.NewSimpleClientset()

	ephrInformer := metaFactory.ForResource(reportsv1.SchemeGroupVersion.WithResource("ephemeralreports"))
	cephrInformer := metaFactory.ForResource(reportsv1.SchemeGroupVersion.WithResource("clusterephemeralreports"))

	c := &controller{
		kyvernoClient:  client,
		bgscanrLister:  ephrInformer.Lister(),
		cbgscanrLister: cephrInformer.Lister(),
	}

	stop := make(chan struct{})
	defer close(stop)

	metaFactory.Start(stop)
	metaFactory.WaitForCacheSync(stop)

	targetUID := types.UID("pod-uid-12345")
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}

	// Create ephemeral report with generateName (name has random suffix)
	ephrReport := reportutils.NewBackgroundScanReport("default", string(targetUID), gvk, "test-pod", targetUID)
	ephrReport.SetName("pod-uid-12345-abcde")
	reportutils.SetResourceVersionLabels(ephrReport, nil)

	// Add metadata to fake metadata client for informer lister
	metaObj := &metav1.PartialObjectMetadata{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-uid-12345-abcde",
			Namespace: "default",
			Labels:    ephrReport.GetLabels(),
		},
	}
	_, err := ephrMetaClient.CreateFake(metaObj, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Add actual object to versioned fake client
	_, err = client.ReportsV1().EphemeralReports("default").Create(context.Background(), ephrReport.(*reportsv1.EphemeralReport), metav1.CreateOptions{})
	assert.NoError(t, err)

	// Wait for informer cache to sync
	assert.Eventually(t, func() bool {
		meta, err := c.getMeta("default", string(targetUID))
		return err == nil && meta != nil && meta.GetName() == "pod-uid-12345-abcde"
	}, 3*time.Second, 50*time.Millisecond)

	// Test getMeta
	meta, err := c.getMeta("default", string(targetUID))
	assert.NoError(t, err)
	assert.NotNil(t, meta)
	assert.Equal(t, "pod-uid-12345-abcde", meta.GetName())

	// Test getReport
	report, err := c.getReport(context.Background(), "default", string(targetUID))
	assert.NoError(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, "pod-uid-12345-abcde", report.GetName())

	// Test cluster-scoped report
	clusterTargetUID := types.UID("node-uid-67890")
	clusterGVK := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Node"}
	cEphrReport := reportutils.NewBackgroundScanReport("", string(clusterTargetUID), clusterGVK, "test-node", clusterTargetUID)
	cEphrReport.SetName("node-uid-67890-fghij")

	cMetaObj := &metav1.PartialObjectMetadata{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-uid-67890-fghij",
			Labels: cEphrReport.GetLabels(),
		},
	}
	_, err = cephrMetaClient.CreateFake(cMetaObj, metav1.CreateOptions{})
	assert.NoError(t, err)

	_, err = client.ReportsV1().ClusterEphemeralReports().Create(context.Background(), cEphrReport.(*reportsv1.ClusterEphemeralReport), metav1.CreateOptions{})
	assert.NoError(t, err)

	assert.Eventually(t, func() bool {
		meta, err := c.getMeta("", string(clusterTargetUID))
		return err == nil && meta != nil && meta.GetName() == "node-uid-67890-fghij"
	}, 3*time.Second, 50*time.Millisecond)

	cMeta, err := c.getMeta("", string(clusterTargetUID))
	assert.NoError(t, err)
	assert.NotNil(t, cMeta)
	assert.Equal(t, "node-uid-67890-fghij", cMeta.GetName())

	cReport, err := c.getReport(context.Background(), "", string(clusterTargetUID))
	assert.NoError(t, err)
	assert.NotNil(t, cReport)
	assert.Equal(t, "node-uid-67890-fghij", cReport.GetName())
}

func Test_getMeta_NotFound(t *testing.T) {
	metaFactory, _, _ := newFakeMetaClient()
	client := versionedfake.NewSimpleClientset()

	ephrInformer := metaFactory.ForResource(reportsv1.SchemeGroupVersion.WithResource("ephemeralreports"))
	cephrInformer := metaFactory.ForResource(reportsv1.SchemeGroupVersion.WithResource("clusterephemeralreports"))

	c := &controller{
		kyvernoClient:  client,
		bgscanrLister:  ephrInformer.Lister(),
		cbgscanrLister: cephrInformer.Lister(),
	}

	stop := make(chan struct{})
	defer close(stop)

	metaFactory.Start(stop)
	metaFactory.WaitForCacheSync(stop)

	_, err := c.getMeta("default", "non-existent-uid")
	assert.Error(t, err)

	_, err = c.getReport(context.Background(), "default", "non-existent-uid")
	assert.Error(t, err)
}
