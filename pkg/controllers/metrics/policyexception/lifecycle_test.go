package policyexception

import (
	"context"
	"errors"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	kyvernov2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

type recordingMetrics struct {
	callback     metric.Callback
	observations []info
}

func (m *recordingMetrics) RecordPolicyExceptionInfo(_ context.Context, _ metric.Observer, group, namespace, name, kind, policy string) {
	m.observations = append(m.observations, info{group, namespace, name, kind, policy})
}

func (m *recordingMetrics) RegisterCallback(callback metric.Callback) (metric.Registration, error) {
	m.callback = callback
	return nil, nil
}

type testManager struct {
	metrics.MetricsConfigManager
	exceptions metrics.PolicyExceptionMetrics
}

func (m testManager) PolicyExceptionMetrics() metrics.PolicyExceptionMetrics { return m.exceptions }

func TestConstructorAndCacheLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	legacy := &kyvernov2.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team", Name: "same"},
		Spec: kyvernov2.PolicyExceptionSpec{Exceptions: []kyvernov2.Exception{
			{PolicyName: "team/old"}, {PolicyName: "team/old"},
		}},
	}
	cel := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team", Name: "same"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			ExpiresAt: &metav1.Time{Time: time.Unix(1, 0)},
			PolicyRefs: []policiesv1beta1.PolicyRef{
				{Kind: "ValidatingPolicy", Name: "missing"},
				{Kind: "ValidatingPolicy", Name: "missing"},
			},
		},
	}
	client := fake.NewSimpleClientset(legacy, cel)
	factory := kyvernoinformers.NewSharedInformerFactory(client, 0)
	legacyInformer := factory.Kyverno().V2().PolicyExceptions()
	celInformer := factory.Policies().V1beta1().PolicyExceptions()
	recording := &recordingMetrics{}
	previous := metrics.GetManager()
	metrics.SetManager(testManager{exceptions: recording})
	defer metrics.SetManager(previous)

	NewController(legacyInformer, celInformer)
	require.NotNil(t, recording.callback)
	// A scrape before synchronization must not create or read informer caches.
	require.NoError(t, recording.callback(ctx, nil))
	require.Empty(t, recording.observations)
	require.Empty(t, client.Actions())

	factory.Start(ctx.Done())
	synced := factory.WaitForCacheSync(ctx.Done())
	// Calling PolicyExceptions() alone does not instantiate an informer.
	// This assertion catches constructors which defer that work until a scrape.
	require.Len(t, synced, 2)
	for _, ok := range synced {
		require.True(t, ok)
	}
	cancel()
	factory.Shutdown()
	client.ClearActions()

	collect := func(want ...info) {
		t.Helper()
		recording.observations = nil
		require.NoError(t, recording.callback(context.Background(), nil))
		require.ElementsMatch(t, want, recording.observations)
		require.Empty(t, client.Actions(), "scraping must not query the API server")
	}
	legacyInfo := info{legacyAPIGroup, "team", "same", "Policy", "team/old"}
	celInfo := info{celAPIGroup, "team", "same", "ValidatingPolicy", "missing"}
	collect(legacyInfo, celInfo)

	second := legacy.DeepCopy()
	second.Namespace = "other"
	second.Spec.Exceptions = nil
	require.NoError(t, legacyInformer.Informer().GetIndexer().Add(second))
	empty := info{legacyAPIGroup, "other", "same", "", ""}
	collect(legacyInfo, celInfo, empty)

	updated := legacy.DeepCopy()
	updated.Spec.Exceptions = []kyvernov2.Exception{{PolicyName: "new-cluster-policy"}}
	require.NoError(t, legacyInformer.Informer().GetIndexer().Update(updated))
	legacyInfo.policyKind, legacyInfo.policyName = "ClusterPolicy", "new-cluster-policy"
	updatedCEL := cel.DeepCopy()
	updatedCEL.Spec.PolicyRefs = nil
	require.NoError(t, celInformer.Informer().GetIndexer().Update(updatedCEL))
	celInfo.policyKind, celInfo.policyName = "", ""
	collect(legacyInfo, celInfo, empty)

	require.NoError(t, legacyInformer.Informer().GetIndexer().Delete(updated))
	collect(celInfo, empty)
	require.NoError(t, celInformer.Informer().GetIndexer().Delete(updatedCEL))
	require.NoError(t, legacyInformer.Informer().GetIndexer().Delete(second))
	collect()
}

type failingLegacyLister struct {
	kyvernov2listers.PolicyExceptionLister
}

func (failingLegacyLister) List(labels.Selector) ([]*kyvernov2.PolicyException, error) {
	return nil, errors.New("legacy list failed")
}

type failingCELLister struct {
	policiesv1beta1listers.PolicyExceptionLister
}

func (failingCELLister) List(labels.Selector) ([]*policiesv1beta1.PolicyException, error) {
	return nil, errors.New("CEL list failed")
}

func TestSourcesAreIndependent(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"legacy", "cel"} {
		t.Run(source, func(t *testing.T) {
			t.Parallel()
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			recording := &recordingMetrics{}
			c := controller{metrics: recording, legacySynced: func() bool { return true }, celSynced: func() bool { return true }}
			var want info
			if source == "legacy" {
				require.NoError(t, indexer.Add(&policiesv1beta1.PolicyException{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cel"}}))
				c.legacyLister = failingLegacyLister{}
				c.celLister = policiesv1beta1listers.NewPolicyExceptionLister(indexer)
				want = info{celAPIGroup, "ns", "cel", "", ""}
			} else {
				require.NoError(t, indexer.Add(&kyvernov2.PolicyException{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "legacy"}}))
				c.legacyLister = kyvernov2listers.NewPolicyExceptionLister(indexer)
				c.celLister = failingCELLister{}
				want = info{legacyAPIGroup, "ns", "legacy", "", ""}
			}
			require.NoError(t, c.report(context.Background(), nil))
			require.Equal(t, []info{want}, recording.observations)
			recording.observations = nil
			if source == "legacy" {
				c.legacySynced = func() bool { return false }
			} else {
				c.celSynced = func() bool { return false }
			}
			require.NoError(t, c.report(context.Background(), nil))
			require.Equal(t, []info{want}, recording.observations)
		})
	}
}

func TestConstructorWithoutMetrics(t *testing.T) {
	previous := metrics.GetManager()
	metrics.SetManager(nil)
	defer metrics.SetManager(previous)
	factory := kyvernoinformers.NewSharedInformerFactory(fake.NewSimpleClientset(), 0)
	require.NotPanics(t, func() {
		NewController(factory.Kyverno().V2().PolicyExceptions(), factory.Policies().V1beta1().PolicyExceptions())
	})
}
