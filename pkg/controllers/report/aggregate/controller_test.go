package aggregate

import (
	"context"
	"testing"
	"time"

	"github.com/kyverno/kyverno/api/kyverno"
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"

	//"github.com/kyverno/kyverno/pkg/controllers/report/aggregate"
	"github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	metafake "k8s.io/client-go/metadata/fake"
	metadatainformers "k8s.io/client-go/metadata/metadatainformer"
	k8stesting "k8s.io/client-go/testing"
)

type dummyReport struct {
	results []v1alpha2.PolicyReportResult
}

// GetCreationTimestamp implements v1.ReportInterface.
func (d *dummyReport) GetCreationTimestamp() metav1.Time {
	panic("unimplemented")
}

// GetDeletionGracePeriodSeconds implements v1.ReportInterface.
func (d *dummyReport) GetDeletionGracePeriodSeconds() *int64 {
	panic("unimplemented")
}

// GetDeletionTimestamp implements v1.ReportInterface.
func (d *dummyReport) GetDeletionTimestamp() *metav1.Time {
	panic("unimplemented")
}

// GetFinalizers implements v1.ReportInterface.
func (d *dummyReport) GetFinalizers() []string {
	panic("unimplemented")
}

// GetGenerateName implements v1.ReportInterface.
func (d *dummyReport) GetGenerateName() string {
	panic("unimplemented")
}

// GetGeneration implements v1.ReportInterface.
func (d *dummyReport) GetGeneration() int64 {
	panic("unimplemented")
}

// GetLabels implements v1.ReportInterface.
func (d *dummyReport) GetLabels() map[string]string {
	panic("unimplemented")
}

// GetManagedFields implements v1.ReportInterface.
func (d *dummyReport) GetManagedFields() []metav1.ManagedFieldsEntry {
	panic("unimplemented")
}

// GetName implements v1.ReportInterface.
func (d *dummyReport) GetName() string {
	panic("unimplemented")
}

// GetNamespace implements v1.ReportInterface.
func (d *dummyReport) GetNamespace() string {
	panic("unimplemented")
}

// GetOwnerReferences implements v1.ReportInterface.
func (d *dummyReport) GetOwnerReferences() []metav1.OwnerReference {
	panic("unimplemented")
}

// GetResourceVersion implements v1.ReportInterface.
func (d *dummyReport) GetResourceVersion() string {
	panic("unimplemented")
}

// GetSelfLink implements v1.ReportInterface.
func (d *dummyReport) GetSelfLink() string {
	panic("unimplemented")
}

// GetUID implements v1.ReportInterface.
func (d *dummyReport) GetUID() types.UID {
	panic("unimplemented")
}

// SetAnnotations implements v1.ReportInterface.
func (d *dummyReport) SetAnnotations(annotations map[string]string) {
	panic("unimplemented")
}

// SetCreationTimestamp implements v1.ReportInterface.
func (d *dummyReport) SetCreationTimestamp(timestamp metav1.Time) {
	panic("unimplemented")
}

// SetDeletionGracePeriodSeconds implements v1.ReportInterface.
func (d *dummyReport) SetDeletionGracePeriodSeconds(*int64) {
	panic("unimplemented")
}

// SetDeletionTimestamp implements v1.ReportInterface.
func (d *dummyReport) SetDeletionTimestamp(timestamp *metav1.Time) {
	panic("unimplemented")
}

// SetFinalizers implements v1.ReportInterface.
func (d *dummyReport) SetFinalizers(finalizers []string) {
	panic("unimplemented")
}

// SetGenerateName implements v1.ReportInterface.
func (d *dummyReport) SetGenerateName(name string) {
	panic("unimplemented")
}

// SetGeneration implements v1.ReportInterface.
func (d *dummyReport) SetGeneration(generation int64) {
	panic("unimplemented")
}

// SetLabels implements v1.ReportInterface.
func (d *dummyReport) SetLabels(labels map[string]string) {
	panic("unimplemented")
}

// SetManagedFields implements v1.ReportInterface.
func (d *dummyReport) SetManagedFields(managedFields []metav1.ManagedFieldsEntry) {
	panic("unimplemented")
}

// SetName implements v1.ReportInterface.
func (d *dummyReport) SetName(name string) {
	panic("unimplemented")
}

// SetNamespace implements v1.ReportInterface.
func (d *dummyReport) SetNamespace(namespace string) {
	panic("unimplemented")
}

// SetOwnerReferences implements v1.ReportInterface.
func (d *dummyReport) SetOwnerReferences([]metav1.OwnerReference) {
	panic("unimplemented")
}

// SetResourceVersion implements v1.ReportInterface.
func (d *dummyReport) SetResourceVersion(version string) {
	panic("unimplemented")
}

// SetResults implements v1.ReportInterface.
func (d *dummyReport) SetResults([]v1alpha2.PolicyReportResult) {
	panic("unimplemented")
}

// SetSelfLink implements v1.ReportInterface.
func (d *dummyReport) SetSelfLink(selfLink string) {
	panic("unimplemented")
}

// SetSummary implements v1.ReportInterface.
func (d *dummyReport) SetSummary(v1alpha2.PolicyReportSummary) {
	panic("unimplemented")
}

// SetUID implements v1.ReportInterface.
func (d *dummyReport) SetUID(uid types.UID) {
	panic("unimplemented")
}

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

	controller := NewController(client, nil, metaFactory, polInformer, cpolInformer, nil, nil, nil)

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

func (d *dummyReport) GetResults() []v1alpha2.PolicyReportResult {
	return d.results
}

func createTimestamp(t time.Time) metav1.Timestamp {
	return metav1.Timestamp{
		Seconds: t.Unix(),
		Nanos:   int32(t.Nanosecond()),
	}
}

func (d *dummyReport) GetAnnotations() map[string]string {
	return map[string]string{}
}

func TestMergeReportsDeduplication(t *testing.T) {
	uid := types.UID("dummy-uid")
	time.Now()

	baseTime := time.Unix(100, 500000000)
	result1 := v1alpha2.PolicyReportResult{
		Policy:  "test-policy",
		Rule:    "test-rule",
		Message: "failure occurred",
		Result:  "fail",
		Source:  report.SourceValidatingPolicy, // e.g., "validating-policy"
		// Use metav1.NewTime without dereferencing.
		Timestamp: createTimestamp(baseTime),
		Resources: []corev1.ObjectReference{{
			Namespace: "default",
			Name:      "test-resource",
		}},
	}
	laterTime := baseTime.Add(10 * time.Second)
	result2 := v1alpha2.PolicyReportResult{
		Policy:  "test-policy",
		Rule:    "test-rule",
		Message: "failure occurred",
		Result:  "fail",
		Source:  report.SourceValidatingAdmissionPolicy, // e.g., "validating-admission-policy"
		// Use metav1.NewTime without dereferencing.
		Timestamp: createTimestamp(laterTime), // Later timestamp
		Resources: []corev1.ObjectReference{{
			Namespace: "default",
			Name:      "test-resource",
		}},
	}

	// Create a dummy report that returns both results.
	dReport := &dummyReport{
		results: []v1alpha2.PolicyReportResult{result1, result2},
	}

	// Setup maps for policy types.
	vpolSet := sets.New[string]()
	vapSet := sets.New[string]()
	ivpolSet := sets.New[string]()
	vpolSet.Insert("test-policy")
	vapSet.Insert("test-policy")
	// Use the unexported type policyMapEntry from the aggregate package.
	polMap := make(map[string]policyMapEntry)

	myMaps := maps{
		vap:   vapSet,
		vpol:  vpolSet,
		ivpol: ivpolSet,
		pol:   polMap,
	}

	// Prepare an empty accumulator.
	accumulator := make(map[string]v1alpha2.PolicyReportResult)

	// Call mergeReports. (Since we're in package aggregate, we can call it directly.)
	mergeReports(myMaps, accumulator, uid, dReport)

	// Expect only one deduplicated result in the accumulator.
	assert.Equal(t, 1, len(accumulator), "Expected one deduplicated result")

	// Since result2 has the later timestamp, it should be kept.
	for _, res := range accumulator {
		assert.Equal(t, report.SourceValidatingAdmissionPolicy, res.Source, "Expected the result with the later timestamp to be kept")
	}
}
