package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/informers/health"
	runtimeutils "github.com/kyverno/kyverno/pkg/utils/runtime"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clocktesting "k8s.io/utils/clock/testing"
)

func TestCleanupWatchReadiness(t *testing.T) {
	t.Parallel()
	var gceObj kyvernov2beta1.GlobalContextEntry
	require.NoError(t, json.Unmarshal([]byte(`{"apiVersion":"kyverno.io/v2beta1","kind":"GlobalContextEntry","metadata":{"name":"test-gce","resourceVersion":"1"},"spec":{"kubernetesResource":{"version":"v1","resource":"configmaps"}}}`), &gceObj))

	var mu sync.Mutex
	disconnected := false
	drop := make(chan struct{})
	var failedRequests atomic.Int32
	var opened atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		down, stop, current := disconnected, drop, gceObj.DeepCopy()
		mu.Unlock()
		if down {
			failedRequests.Add(1)
			http.Error(w, "watch path disconnected", http.StatusServiceUnavailable)
			return
		}
		if path.Base(r.URL.Path) != "globalcontextentries" {
			http.NotFound(w, r)
			return
		}
		apiVersion := "kyverno.io/v2beta1"
		w.Header().Set("Content-Type", "application/json")
		enc := json.NewEncoder(w)
		if r.URL.Query().Get("watch") != "true" {
			_ = enc.Encode(map[string]any{
				"apiVersion": apiVersion,
				"kind":       "GlobalContextEntryList",
				"metadata":   map[string]string{"resourceVersion": current.ResourceVersion},
				"items":      []any{current},
			})
			return
		}
		initial := r.URL.Query().Get("sendInitialEvents") == "true"
		w.WriteHeader(http.StatusOK)
		if initial {
			_ = enc.Encode(map[string]any{"type": "ADDED", "object": current})
			_ = enc.Encode(map[string]any{
				"type": "BOOKMARK",
				"object": map[string]any{
					"apiVersion": apiVersion,
					"kind":       "GlobalContextEntry",
					"metadata": map[string]any{
						"resourceVersion": current.ResourceVersion,
						"annotations":     map[string]string{metav1.InitialEventsAnnotationKey: "true"},
					},
				},
			})
		} else {
			_ = enc.Encode(map[string]any{"type": "MODIFIED", "object": current})
		}
		w.(http.Flusher).Flush()
		opened.Add(1)
		select {
		case <-r.Context().Done():
		case <-stop:
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	client, err := kyvernoclient.NewForConfig(&rest.Config{Host: srv.URL, QPS: 100, Burst: 100})
	require.NoError(t, err)

	fakeClock := clocktesting.NewFakeClock(time.Now())
	tracker := health.NewTracker(logr.Discard(), fakeClock)
	factory := kyvernoinformers.NewSharedInformerFactory(client, 0)
	registerCleanupInformers(factory, client, tracker)

	gce := factory.Kyverno().V2beta1().GlobalContextEntries()
	require.Same(t, gce.Informer(), factory.Kyverno().V2beta1().GlobalContextEntries().Informer())

	factory.Start(ctx.Done())
	defer factory.Shutdown()
	defer cancel()

	require.Eventually(t, func() bool {
		return gce.Informer().HasSynced() && opened.Load() >= 1 && tracker.Ready()
	}, 10*time.Second, 10*time.Millisecond)

	deployment := informers.NewSharedInformerFactory(fake.NewSimpleClientset(), 0).Apps().V1().Deployments()
	readiness := runtimeutils.NewRuntime(logr.Discard(), "", deployment, mockCertValidator{valid: true}, tracker.Ready)
	router := buildTestServer(t, readiness)

	probe := func(path string) int {
		out := httptest.NewRecorder()
		router.ServeHTTP(out, httptest.NewRequest(http.MethodGet, path, nil))
		return out.Code
	}

	require.Equal(t, http.StatusOK, probe(config.ReadinessServicePath))
	require.Equal(t, http.StatusOK, probe(config.LivenessServicePath))

	mu.Lock()
	disconnected = true
	close(drop)
	gceObj.ResourceVersion = "2"
	mu.Unlock()

	require.Eventually(t, func() bool { return failedRequests.Load() > 0 }, 10*time.Second, 10*time.Millisecond)
	fakeClock.Step(health.GracePeriod - time.Nanosecond)
	require.Equal(t, http.StatusOK, probe(config.ReadinessServicePath))

	fakeClock.Step(time.Nanosecond)
	require.Equal(t, http.StatusInternalServerError, probe(config.ReadinessServicePath))
	require.Equal(t, http.StatusOK, probe(config.LivenessServicePath))

	mu.Lock()
	disconnected = false
	drop = make(chan struct{})
	mu.Unlock()

	require.Eventually(t, func() bool {
		fakeClock.Step(time.Second)
		current, err := gce.Lister().Get("test-gce")
		return err == nil && current.ResourceVersion == "2" && probe(config.ReadinessServicePath) == http.StatusOK
	}, 15*time.Second, 10*time.Millisecond)
}
