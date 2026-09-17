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
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	imageverifycache "github.com/kyverno/kyverno/pkg/image/verification/cache"
	"github.com/kyverno/kyverno/pkg/informers/health"
	runtimeutils "github.com/kyverno/kyverno/pkg/utils/runtime"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clocktesting "k8s.io/utils/clock/testing"
	ctrlcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type healthyCertificates struct{}

func (healthyCertificates) ValidateCert(context.Context) (bool, error) { return true, nil }

// This regression uses the actual generated REST client, reflector, shared
// informer, runtime and probe. Requests unrelated to list/watch still succeed
// during the interruption, just as in #15626.
func TestAdmissionWatchReadinessRegression(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"list-eof", "watchlist-error"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			var policy kyvernov1.ClusterPolicy
			require.NoError(t, json.Unmarshal([]byte(`{"apiVersion":"kyverno.io/v1","kind":"ClusterPolicy","metadata":{"name":"require-label","resourceVersion":"1"},"spec":{"background":false,"rules":[{"name":"label","match":{"resources":{"kinds":["ConfigMap"]}},"validate":{"message":"label required","pattern":{"metadata":{"labels":{"approved":"old"}}}}}]}}`), &policy))
			var mu sync.Mutex
			disconnected := false
			drop := make(chan struct{})
			var failedRequests atomic.Int32
			var opened atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/unrelated" {
					w.WriteHeader(http.StatusOK)
					return
				}
				mu.Lock()
				down, stop, current := disconnected, drop, policy.DeepCopy()
				mu.Unlock()
				if down {
					failedRequests.Add(1)
					http.Error(w, "watch path disconnected", http.StatusServiceUnavailable)
					return
				}
				resource := path.Base(r.URL.Path)
				kinds := map[string]string{"clusterpolicies": "ClusterPolicy", "policies": "Policy", "globalcontextentries": "GlobalContextEntry", "policyexceptions": "PolicyException"}
				kind, ok := kinds[resource]
				if !ok {
					http.NotFound(w, r)
					return
				}
				apiVersion := "kyverno.io/v1"
				if resource == "globalcontextentries" {
					apiVersion = "kyverno.io/v2beta1"
				}
				if resource == "policyexceptions" {
					apiVersion = "kyverno.io/v2"
				}
				w.Header().Set("Content-Type", "application/json")
				enc := json.NewEncoder(w)
				if r.URL.Query().Get("watch") != "true" {
					items := []any{}
					if resource == "clusterpolicies" {
						items = append(items, current)
					}
					_ = enc.Encode(map[string]any{"apiVersion": apiVersion, "kind": kind + "List", "metadata": map[string]string{"resourceVersion": current.ResourceVersion}, "items": items})
					return
				}
				initial := r.URL.Query().Get("sendInitialEvents") == "true"
				if initial && mode == "list-eof" {
					w.WriteHeader(http.StatusBadRequest)
					_ = enc.Encode(metav1.Status{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Status"}, Status: metav1.StatusFailure, Reason: metav1.StatusReasonBadRequest, Code: 400, Message: "sendInitialEvents unsupported"})
					return
				}
				w.WriteHeader(http.StatusOK)
				if initial {
					if resource == "clusterpolicies" {
						_ = enc.Encode(map[string]any{"type": "ADDED", "object": current})
					}
					_ = enc.Encode(map[string]any{"type": "BOOKMARK", "object": map[string]any{"apiVersion": apiVersion, "kind": kind, "metadata": map[string]any{"resourceVersion": current.ResourceVersion, "annotations": map[string]string{metav1.InitialEventsAnnotationKey: "true"}}}})
				} else if resource == "clusterpolicies" {
					// Replay the current state if client-go resumes directly without a relist.
					_ = enc.Encode(map[string]any{"type": "MODIFIED", "object": current})
				}
				w.(http.Flusher).Flush()
				opened.Add(1)
				select {
				case <-r.Context().Done():
				case <-stop:
					if mode == "watchlist-error" {
						_ = enc.Encode(map[string]any{"type": "ERROR", "object": metav1.Status{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Status"}, Status: metav1.StatusFailure, Reason: metav1.StatusReasonInternalError, Code: 500, Message: "http2: client connection lost"}})
						w.(http.Flusher).Flush()
					}
				}
			}))
			defer srv.Close()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			client, err := kyvernoclient.NewForConfig(&rest.Config{Host: srv.URL, QPS: 100, Burst: 100})
			require.NoError(t, err)
			clock := clocktesting.NewFakeClock(time.Now())
			tracker := health.NewTracker(logr.Discard(), clock)
			factory := kyvernoinformers.NewSharedInformerFactory(client, 0)
			registerAdmissionInformers(factory, client, tracker, true)
			cpol := factory.Kyverno().V1().ClusterPolicies()
			require.Same(t, cpol.Informer(), factory.Kyverno().V1().ClusterPolicies().Informer())
			factory.Start(ctx.Done())
			defer factory.Shutdown() // Cancel before waiting for the factory below.
			defer cancel()
			require.Eventually(t, func() bool { return cpol.Informer().HasSynced() && opened.Load() >= 4 && tracker.Ready() }, 10*time.Second, 10*time.Millisecond)

			deployment := informers.NewSharedInformerFactory(fake.NewSimpleClientset(), 0).Apps().V1().Deployments()
			readiness := runtimeutils.NewRuntime(logr.Discard(), "", deployment, healthyCertificates{}, tracker.Ready)
			probe := func() int {
				out := httptest.NewRecorder()
				handlers.Probe(readiness.IsReady)(out, httptest.NewRequest(http.MethodGet, "/health/readiness", nil))
				return out.Code
			}
			cfg := config.NewDefaultConfiguration(false)
			jp := jmespath.New(cfg)
			policyEngine := engine.NewEngine(cfg, jp, nil, nil, imageverifycache.DisabledImageVerifyCache(), factories.DefaultContextLoaderFactory(nil), nil, nil)
			admitted := func() bool {
				current, err := cpol.Lister().Get("require-label")
				require.NoError(t, err)
				resource := unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", "metadata": map[string]any{"name": "example", "namespace": "default", "labels": map[string]any{"approved": "old"}}}}
				pc, err := engine.NewPolicyContext(jp, resource, kyvernov1.Create, nil, cfg)
				require.NoError(t, err)
				response := policyEngine.Validate(ctx, pc.WithPolicy(current))
				require.Len(t, response.PolicyResponse.Rules, 1)
				return response.IsSuccessful()
			}
			require.Equal(t, http.StatusOK, probe())
			require.True(t, admitted())
			mu.Lock()
			disconnected = true
			close(drop)
			policy.ResourceVersion = "2"
			// Change the policy through the independent control path while watches fail.
			require.NoError(t, json.Unmarshal([]byte(`{"metadata":{"labels":{"approved":"new"}}}`), &policy.Spec.Rules[0].Validation.RawPattern))
			mu.Unlock()
			require.Eventually(t, func() bool { return failedRequests.Load() > 0 }, 10*time.Second, 10*time.Millisecond)
			require.True(t, cpol.Informer().HasSynced(), "HasSynced remains true after disconnection")
			response, err := srv.Client().Get(srv.URL + "/unrelated")
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())
			clock.Step(health.GracePeriod - time.Nanosecond)
			require.Equal(t, http.StatusOK, probe())
			clock.Step(time.Nanosecond)
			require.Equal(t, http.StatusInternalServerError, probe(), "regression: certificates alone incorrectly leave this replica ready")
			require.True(t, readiness.IsLive(ctx))
			require.True(t, admitted(), "the informer still holds the old policy during the outage")
			mu.Lock()
			disconnected = false
			drop = make(chan struct{})
			mu.Unlock()
			require.Eventually(t, func() bool {
				current, err := cpol.Lister().Get("require-label")
				return err == nil && current.ResourceVersion == "2" && probe() == http.StatusOK
			}, 15*time.Second, 10*time.Millisecond)
			require.False(t, admitted(), "the recovered cache must evaluate the updated policy")
		})
	}
}

func TestCELManagerCacheWatchHealth(t *testing.T) {
	t.Parallel()
	var down atomic.Bool
	var failures atomic.Int32
	var watches atomic.Int32
	drop := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if down.Load() {
			failures.Add(1)
			http.Error(w, "disconnected", http.StatusServiceUnavailable)
			return
		}
		kind := "ValidatingPolicy"
		if path.Base(r.URL.Path) == "policyexceptions" {
			kind = "PolicyException"
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("watch") != "true" {
			_ = json.NewEncoder(w).Encode(map[string]any{"apiVersion": "policies.kyverno.io/v1beta1", "kind": kind + "List", "metadata": map[string]string{"resourceVersion": "1"}, "items": []any{}})
			return
		}
		if r.URL.Query().Get("sendInitialEvents") == "true" {
			_ = json.NewEncoder(w).Encode(map[string]any{"type": "BOOKMARK", "object": map[string]any{"apiVersion": "policies.kyverno.io/v1beta1", "kind": kind, "metadata": map[string]any{"resourceVersion": "1", "annotations": map[string]string{metav1.InitialEventsAnnotationKey: "true"}}}})
		}
		w.(http.Flusher).Flush()
		watches.Add(1)
		select {
		case <-r.Context().Done():
		case <-drop:
		}
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	scheme := kruntime.NewScheme()
	require.NoError(t, policiesv1beta1.Install(scheme))
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{policiesv1beta1.SchemeGroupVersion})
	mapper.Add(policiesv1beta1.SchemeGroupVersion.WithKind("ValidatingPolicy"), meta.RESTScopeRoot)
	mapper.Add(policiesv1beta1.SchemeGroupVersion.WithKind("PolicyException"), meta.RESTScopeNamespace)
	clock := clocktesting.NewFakeClock(time.Now())
	tracker := health.NewTracker(logr.Discard(), clock)
	managerCache, err := ctrlcache.New(&rest.Config{Host: srv.URL}, ctrlcache.Options{Scheme: scheme, Mapper: mapper, NewInformer: tracker.NewInformer})
	require.NoError(t, err)
	for _, obj := range []client.Object{&policiesv1beta1.ValidatingPolicy{}, &policiesv1beta1.PolicyException{}} {
		first, err := managerCache.GetInformer(ctx, obj, ctrlcache.BlockUntilSynced(false))
		require.NoError(t, err)
		second, err := managerCache.GetInformer(ctx, obj, ctrlcache.BlockUntilSynced(false))
		require.NoError(t, err)
		require.Same(t, first, second, "monitor the shared manager informer, not a separate exception cache")
	}
	done := make(chan error, 1)
	go func() { done <- managerCache.Start(ctx) }()
	defer func() { cancel(); require.NoError(t, <-done) }()
	require.Eventually(t, func() bool { return watches.Load() >= 2 && tracker.Ready() }, 10*time.Second, 10*time.Millisecond)
	down.Store(true)
	close(drop)
	require.Eventually(t, func() bool { return failures.Load() > 0 }, 10*time.Second, 10*time.Millisecond)
	clock.Step(health.GracePeriod)
	require.False(t, tracker.Ready())
}
