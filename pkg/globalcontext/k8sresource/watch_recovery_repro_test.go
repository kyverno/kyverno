package k8sresource

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
)

type recoveryEventSink struct {
	calls atomic.Int32
	mu    sync.Mutex
	infos []event.Info
}

func (s *recoveryEventSink) Add(events ...event.Info) {
	s.mu.Lock()
	s.infos = append(s.infos, events...)
	s.mu.Unlock()
	s.calls.Add(int32(len(events)))
}

func (s *recoveryEventSink) containsMessage(fragment string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.infos {
		if strings.Contains(e.Message, fragment) {
			return true
		}
	}
	return false
}

// Exercise the production New/Get lifecycle with client-go's real informer.
// An expired watch forces a relist; exactly one relist fails, then the API
// fixture is healthy again. The same entry must eventually observe new data.
func TestEntry_PostSyncRelistRecovery(t *testing.T) {
	for _, tc := range []struct {
		name        string
		failInitial bool
		failRelist  bool
		projections bool
	}{
		{name: "healthy_relist_control", projections: true},
		{name: "transient_relist_with_projection", failRelist: true, projections: true},
		{name: "transient_relist_without_projection", failRelist: true},
		{name: "initial_list_failure_fails_fast", failInitial: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
			dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
				runtime.NewScheme(), map[schema.GroupVersionResource]string{gvr: "ConfigMapList"},
			)
			var lists, watches atomic.Int32
			dyn.PrependReactor("list", "configmaps", func(clienttesting.Action) (bool, runtime.Object, error) {
				n := lists.Add(1)
				if tc.failInitial && n == 1 {
					return true, nil, apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "missing")
				}
				if tc.failRelist && n == 2 {
					return true, nil, apierrors.NewServiceUnavailable("injected one-shot relist outage")
				}
				value, rv := "old", "1"
				if n > 1 {
					value, rv = "new", "2"
				}
				list := &unstructured.UnstructuredList{
					Items: []unstructured.Unstructured{{Object: map[string]interface{}{
						"apiVersion": "v1", "kind": "ConfigMap",
						"metadata": map[string]interface{}{"name": "source", "namespace": "default", "resourceVersion": rv},
						"data":     map[string]interface{}{"value": value},
					}}},
				}
				list.SetAPIVersion("v1")
				list.SetKind("ConfigMapList")
				list.SetResourceVersion(rv)
				return true, list, nil
			})
			firstWatch := watch.NewRaceFreeFake()
			defer firstWatch.Stop()
			dyn.PrependWatchReactor("configmaps", func(clienttesting.Action) (bool, watch.Interface, error) {
				if watches.Add(1) == 1 {
					return true, firstWatch, nil
				}
				return true, watch.NewRaceFreeFake(), nil
			})
			gce := &kyvernov2beta1.GlobalContextEntry{
				ObjectMeta: metav1.ObjectMeta{Name: "watch-recovery-repro"},
				Spec: kyvernov2beta1.GlobalContextEntrySpec{
					KubernetesResource: &kyvernov2beta1.KubernetesResource{Version: "v1", Resource: "configmaps", Namespace: "default"},
					Projections:        []kyvernov2beta1.GlobalContextEntryProjection{{Name: "values", JMESPath: "[].data.value"}},
				},
			}
			if !tc.projections {
				gce.Spec.Projections = nil
			}
			events := &recoveryEventSink{}
			started := time.Now()
			entry, err := New(ctx, gce, events, dyn, logr.Discard(), gvr, "default", jmespath.New(config.NewDefaultConfiguration(false)))
			if tc.failInitial {
				require.Error(t, err)
				require.ErrorIs(t, err, context.Canceled)
				require.Nil(t, entry)
				require.Less(t, time.Since(started), cacheSyncTimeout)
				require.Eventually(t, func() bool { return events.calls.Load() > 0 }, time.Second, 10*time.Millisecond)
				return
			}
			require.NoError(t, err)
			defer entry.Stop()
			if tc.projections {
				initial, err := entry.Get("values")
				require.NoError(t, err)
				require.Equal(t, []interface{}{"old"}, initial)
			}
			require.Equal(t, "old", currentConfigMapValue(t, entry))
			require.Eventually(t, func() bool { return watches.Load() == 1 }, 5*time.Second, 10*time.Millisecond)

			// New has returned and the entry is usable before the fault occurs.
			firstWatch.Error(&metav1.Status{Status: metav1.StatusFailure, Reason: metav1.StatusReasonExpired, Code: 410, Message: "injected expired resource version"})
			if tc.failRelist {
				require.Eventually(t, func() bool {
					return events.containsMessage("injected one-shot relist outage")
				}, 10*time.Second, 10*time.Millisecond,
					"the watch error callback must report the underlying relist failure")
			}
			recovered := func() bool {
				if tc.projections {
					projection, err := entry.Get("values")
					if err != nil {
						return false
					}
					values, ok := projection.([]interface{})
					if !ok || len(values) != 1 || values[0] != "new" {
						return false
					}
				}
				raw, err := entry.Get("")
				if err != nil {
					return false
				}
				objects, ok := raw.([]interface{})
				if !ok || len(objects) != 1 {
					return false
				}
				obj, ok := objects[0].(map[string]interface{})
				if !ok {
					return false
				}
				value, _, _ := unstructured.NestedString(obj, "data", "value")
				return value == "new" && watches.Load() >= 2
			}
			ok := assertRecovery(t, recovered)
			if tc.projections {
				projection, getErr := entry.Get("values")
				t.Logf("recovered=%v list_calls=%d watch_calls=%d error_events=%d projection=%v get_error=%v", ok, lists.Load(), watches.Load(), events.calls.Load(), projection, getErr)
			} else {
				t.Logf("recovered=%v list_calls=%d watch_calls=%d error_events=%d", ok, lists.Load(), watches.Load(), events.calls.Load())
			}
			raw, rawErr := entry.Get("")
			t.Logf("raw=%v raw_error=%v", raw, rawErr)
			if !ok {
				t.Fatal("same entry never recovered after the one-shot relist failure")
			}
		})
	}
}

func currentConfigMapValue(t *testing.T, entry interface{ Get(string) (any, error) }) string {
	t.Helper()
	raw, err := entry.Get("")
	require.NoError(t, err)
	objects, ok := raw.([]interface{})
	require.True(t, ok)
	require.Len(t, objects, 1)
	obj, ok := objects[0].(map[string]interface{})
	require.True(t, ok)
	value, found, err := unstructured.NestedString(obj, "data", "value")
	require.NoError(t, err)
	require.True(t, found)
	return value
}

func TestShouldReportWatchError(t *testing.T) {
	now := time.Unix(100, 0)
	require.True(t, shouldReportWatchError(now, time.Time{}))
	require.False(t, shouldReportWatchError(now.Add(watchErrorEventInterval-time.Nanosecond), now))
	require.True(t, shouldReportWatchError(now.Add(watchErrorEventInterval), now))
	require.False(t, shouldReportWatchError(now.Add(-time.Second), now))
}

func assertRecovery(t *testing.T, recovered func() bool) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if recovered() {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
	}
}
