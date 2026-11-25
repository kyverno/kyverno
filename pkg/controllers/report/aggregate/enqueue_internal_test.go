package aggregate

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type recordingQueue struct {
	workqueue.TypedRateLimitingInterface[any]
	added []string
}

func (q *recordingQueue) AddAfter(item any, _ time.Duration) {
	q.added = append(q.added, item.(string))
}

type failingLister struct {
	cache.GenericLister
}

func (failingLister) List(labels.Selector) ([]runtime.Object, error) {
	return nil, errors.New("list failed")
}

func newReportLister(t *testing.T, resource, namespace, name, uid string) cache.GenericLister {
	t.Helper()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	assert.NoError(t, indexer.Add(&metav1.PartialObjectMetadata{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			UID:       types.UID(uid),
			Labels:    map[string]string{kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp},
		},
	}))
	return cache.NewGenericLister(indexer, schema.GroupResource{Resource: resource})
}

func TestEnqueueReportsForPolicy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		policyCache map[string]sets.Set[string]
		polrFails   bool
		cpolrFails  bool
		want        []string
	}{
		{
			name:        "empty cache queues namespaced and cluster-scoped reports",
			policyCache: map[string]sets.Set[string]{},
			want:        []string{"default/polr-1", "cpolr-1"},
		},
		{
			name: "populated cache queues only reports that reference the policy",
			policyCache: map[string]sets.Set[string]{
				"polr-uid":  sets.New("pol-b"),
				"cpolr-uid": sets.New("pol-a"),
			},
			want: []string{"cpolr-1"},
		},
		{
			name:        "populated cache queues reports that aren't cached yet",
			policyCache: map[string]sets.Set[string]{"other-uid": sets.New("pol-b")},
			want:        []string{"default/polr-1", "cpolr-1"},
		},
		{
			name:        "empty cache with failing namespaced lister still queues cluster-scoped reports",
			policyCache: map[string]sets.Set[string]{},
			polrFails:   true,
			want:        []string{"cpolr-1"},
		},
		{
			name:        "empty cache with failing cluster-scoped lister still queues namespaced reports",
			policyCache: map[string]sets.Set[string]{},
			cpolrFails:  true,
			want:        []string{"default/polr-1"},
		},
		{
			name:        "populated cache with failing namespaced lister still queues cluster-scoped reports",
			policyCache: map[string]sets.Set[string]{"cpolr-uid": sets.New("pol-a")},
			polrFails:   true,
			want:        []string{"cpolr-1"},
		},
		{
			name:        "populated cache with failing cluster-scoped lister still queues namespaced reports",
			policyCache: map[string]sets.Set[string]{"polr-uid": sets.New("pol-a")},
			cpolrFails:  true,
			want:        []string{"default/polr-1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			q := &recordingQueue{}
			var polrLister cache.GenericLister = newReportLister(t, "policyreports", "default", "polr-1", "polr-uid")
			if tt.polrFails {
				polrLister = failingLister{}
			}
			var cpolrLister cache.GenericLister = newReportLister(t, "clusterpolicyreports", "", "cpolr-1", "cpolr-uid")
			if tt.cpolrFails {
				cpolrLister = failingLister{}
			}
			c := &controller{
				polrLister:              polrLister,
				cpolrLister:             cpolrLister,
				managedSelector:         labels.SelectorFromSet(labels.Set{kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp}),
				cacheMu:                 &sync.Mutex{},
				reportUUIDToPolicyCache: tt.policyCache,
				backQueue:               q,
			}
			c.enqueueReportsForPolicy(&metav1.ObjectMeta{Name: "pol-a"})
			assert.ElementsMatch(t, tt.want, q.added)
		})
	}
}
