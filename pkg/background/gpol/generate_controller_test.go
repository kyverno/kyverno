package gpol

import (
	"context"
	"sync"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	gpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	gpolengine "github.com/kyverno/kyverno/pkg/cel/policies/gpol/engine"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/logging"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/utils/ptr"
)

type testEngine struct {
	generated []*unstructured.Unstructured
}

func (e *testEngine) Handle(_ celengine.EngineRequest, p gpolengine.Policy, cacheRestore bool) (gpolengine.EngineResponse, error) {
	rule := engineapi.RulePass("rule", engineapi.Generation, "ok", nil)
	if !cacheRestore {
		rule = rule.WithGeneratedResources(e.generated)
	}
	return gpolengine.EngineResponse{
		Trigger: &unstructured.Unstructured{},
		Policies: []gpolengine.GeneratingPolicyResponse{{
			Policy: p.Policy,
			Result: rule,
		}},
	}, nil
}

type countingEngine struct {
	calls int
}

func (e *countingEngine) Handle(_ celengine.EngineRequest, p gpolengine.Policy, _ bool) (gpolengine.EngineResponse, error) {
	e.calls++
	return gpolengine.EngineResponse{
		Trigger: &unstructured.Unstructured{},
		Policies: []gpolengine.GeneratingPolicyResponse{{
			Policy: p.Policy,
			Result: engineapi.RulePass("rule", engineapi.Generation, "ok", nil),
		}},
	}, nil
}

type testProvider struct {
	policy gpolengine.Policy
}

func (p *testProvider) Get(context.Context, string) (gpolengine.Policy, error) {
	return p.policy, nil
}

type testStatusControl struct{}

func (testStatusControl) Failed(string, string, []kyvernov1.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	return nil, nil
}
func (testStatusControl) Success(string, []kyvernov1.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	return nil, nil
}
func (testStatusControl) Skip(string, []kyvernov1.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	return nil, nil
}

type testEventGen struct{}

func (testEventGen) Add(...event.Info) {}

type errorEngine struct{}

func (e *errorEngine) Handle(_ celengine.EngineRequest, p gpolengine.Policy, _ bool) (gpolengine.EngineResponse, error) {
	return gpolengine.EngineResponse{
		Trigger: &unstructured.Unstructured{},
		Policies: []gpolengine.GeneratingPolicyResponse{{
			Policy: p.Policy,
			Result: engineapi.RuleError("rule", engineapi.Generation, "failed to evaluate policy", assert.AnError, nil),
		}},
	}, nil
}

type recordingStatusControl struct {
	failed  bool
	success bool
	message string
}

func (r *recordingStatusControl) Failed(_ string, message string, _ []kyvernov1.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	r.failed = true
	r.message = message
	return nil, nil
}

func (r *recordingStatusControl) Success(string, []kyvernov1.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	r.success = true
	return nil, nil
}

func (r *recordingStatusControl) Skip(string, []kyvernov1.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	return nil, nil
}

type triggerClient struct {
	*MockClient
	trigger *unstructured.Unstructured
}

func (c *triggerClient) GetResource(_ context.Context, apiVersion string, kind string, namespace, name string, _ ...string) (*unstructured.Unstructured, error) {
	if c.trigger.GetAPIVersion() == apiVersion &&
		c.trigger.GetKind() == kind &&
		c.trigger.GetNamespace() == namespace &&
		c.trigger.GetName() == name {
		return c.trigger.DeepCopy(), nil
	}
	return nil, nil
}

func TestProcessUR_FilteredTriggerSkipsEngine(t *testing.T) {
	policyName := "test-gpol"
	trigger := makeUnstructured("", "", "v1", "ConfigMap", "trigger-cm", "tenant-a", "trigger-uid", nil)
	cfg := config.NewDefaultConfiguration(false)
	cfg.Load(&corev1.ConfigMap{
		Data: map[string]string{
			"resourceFilters": "[ConfigMap,tenant-a,*]",
		},
	})

	engine := &countingEngine{}
	controller := &CELGenerateController{
		client: &triggerClient{
			MockClient: &MockClient{},
			trigger:    trigger,
		},
		context:       libs.NewFakeContextProvider(),
		engine:        engine,
		provider:      &testProvider{policy: gpolengine.Policy{Policy: &policiesv1beta1.GeneratingPolicy{ObjectMeta: metav1.ObjectMeta{Name: policyName}}}},
		watchManager:  &WatchManager{},
		statusControl: testStatusControl{},
		eventGen:      testEventGen{},
		log:           logging.WithName("test-gpol-controller"),
		configuration: cfg,
	}

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-filtered-trigger"},
		Spec: kyvernov2.UpdateRequestSpec{
			Type:   kyvernov2.CELGenerate,
			Policy: policyName,
			RuleContext: []kyvernov2.RuleContext{{
				Rule: "rule",
				Trigger: kyvernov1.ResourceSpec{
					APIVersion: trigger.GetAPIVersion(),
					Kind:       trigger.GetKind(),
					Namespace:  trigger.GetNamespace(),
					Name:       trigger.GetName(),
					UID:        trigger.GetUID(),
				},
			}},
		},
	}

	require.NoError(t, controller.ProcessUR(ur))
	assert.Zero(t, engine.calls)
}

func TestProcessUR_ConcurrentCacheRestoreAndGenerateExistingDoesNotDeleteDownstream(t *testing.T) {
	policyName := "test-gpol"
	// The trigger is now resolved by name through triggerClient.GetResource, so
	// ProcessUR runs the engine and reaches needsReports, which dereferences the
	// global reporting configuration. Set it for this test and restore it after.
	prevReportingCfg := reportutils.ReportingCfg
	reportutils.ReportingCfg = reportutils.NewReportingConfig(nil)
	t.Cleanup(func() { reportutils.ReportingCfg = prevReportingCfg })
	trigger := makeUnstructured("", "example.io", "v1", "TestTrigger", "existing-trigger", "tenant-a", "trigger-uid", nil)
	downstream := makeUnstructured("", "", "v1", "ConfigMap", "test-cm", "tenant-a", "downstream-uid", map[string]string{
		common.GeneratePolicyLabel:     policyName,
		common.GenerateTriggerUIDLabel: string(trigger.GetUID()),
	})
	configMapGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}

	client := &triggerClient{
		MockClient: &MockClient{},
		trigger:    trigger,
	}
	wm := &WatchManager{
		client: client,
		restMapper: &mockRESTMapper{fn: func(gk schema.GroupKind, version string) (*meta.RESTMapping, error) {
			switch {
			case gk.Group == "" && gk.Kind == "ConfigMap" && version == "v1":
				return &meta.RESTMapping{Resource: configMapGVR}, nil
			case gk.Group == "example.io" && gk.Kind == "TestTrigger" && version == "v1":
				return &meta.RESTMapping{Resource: schema.GroupVersionResource{Group: "example.io", Version: "v1", Resource: "testtriggers"}}, nil
			default:
				return nil, assert.AnError
			}
		}},
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{
			configMapGVR: {
				watcher: watch.NewFake(),
				metadataCache: map[types.UID]Resource{
					downstream.GetUID(): {
						Name:      downstream.GetName(),
						Namespace: downstream.GetNamespace(),
						Labels:    downstream.GetLabels(),
						Data:      nil,
					},
				},
			},
		},
		policyRefs: map[string][]schema.GroupVersionResource{
			policyName: {configMapGVR},
		},
		refCount: map[schema.GroupVersionResource]int{
			configMapGVR: 1,
		},
		log: logging.WithName("test-watch-manager"),
	}

	policy := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: policyName},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.GeneratingPolicyEvaluationConfiguration{
				SynchronizationConfiguration: &policiesv1beta1.SynchronizationConfiguration{
					Enabled: ptr.To(true),
				},
			},
		},
	}
	controller := &CELGenerateController{
		client:        client,
		restMapper:    wm.restMapper,
		context:       libs.NewFakeContextProvider(),
		engine:        &testEngine{generated: []*unstructured.Unstructured{downstream.DeepCopy()}},
		provider:      &testProvider{policy: gpolengine.Policy{Policy: policy}},
		watchManager:  wm,
		statusControl: testStatusControl{},
		eventGen:      testEventGen{},
		log:           logging.WithName("test-gpol-controller"),
	}

	newUR := func(cacheRestore bool) *kyvernov2.UpdateRequest {
		return &kyvernov2.UpdateRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ur-test",
			},
			Spec: kyvernov2.UpdateRequestSpec{
				Type:   kyvernov2.CELGenerate,
				Policy: policyName,
				RuleContext: []kyvernov2.RuleContext{{
					Rule:         "rule",
					CacheRestore: cacheRestore,
					Trigger: kyvernov1.ResourceSpec{
						APIVersion: trigger.GetAPIVersion(),
						Kind:       trigger.GetKind(),
						Namespace:  trigger.GetNamespace(),
						Name:       trigger.GetName(),
						UID:        trigger.GetUID(),
					},
				}},
			},
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		require.NoError(t, controller.ProcessUR(newUR(true)))
	}()
	go func() {
		defer wg.Done()
		require.NoError(t, controller.ProcessUR(newUR(false)))
	}()
	wg.Wait()

	wm.lock.Lock()
	defer wm.lock.Unlock()
	assert.Len(t, client.deleted, 0)
	assert.Contains(t, wm.dynamicWatchers[configMapGVR].metadataCache, downstream.GetUID())
}

// TestDynamicWatcher_HandleUpdateRevertsOwnWriteDuringStaleCacheWindow is the
// first half of the gpol clone/sync race behind the chainsaw scenario
// generating-policies/clone/sync/sync-modify-trigger: it proves the defect
// mechanism in dynamic_watcher.go's handleUpdate in isolation, deterministically
// and without any goroutine scheduling. handleUpdate has no way to distinguish
// "the cache is stale because a legitimate Kyverno write hasn't been synced into
// it yet" from "a user modified the downstream out of band" -- both look like a
// hash mismatch against whatever is currently cached, and both are reverted
// (dynamic_watcher.go:576 "downstream resource updated by user, reverting
// changes"). generate_controller.go's ProcessUR is what can leave the cache in
// exactly that first state (see the sibling test below), so this is the
// mechanism that turns that window into data loss.
func TestDynamicWatcher_HandleUpdateRevertsOwnWriteDuringStaleCacheWindow(t *testing.T) {
	secretGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	downstreamOld := makeUnstructured("", "", "v1", "Secret", "sync-modify-trigger", "sync-modify-trigger", "downstream-uid", map[string]string{
		common.GeneratePolicyLabel:     "test-gpol-sync-race",
		common.GenerateTriggerUIDLabel: "trigger-uid",
	})
	require.NoError(t, unstructured.SetNestedField(downstreamOld.Object, "YmFy", "data", "foo")) // base64("bar")
	downstreamNew := downstreamOld.DeepCopy()
	require.NoError(t, unstructured.SetNestedField(downstreamNew.Object, "Z2l0bGFi", "data", "foo")) // base64("gitlab")

	client := &MockClient{}
	wm := &WatchManager{
		client: client,
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{
			secretGVR: {
				watcher: watch.NewFake(),
				metadataCache: map[types.UID]Resource{
					// The cache still holds the OLD hash: this is the exact
					// state that exists between engine.Handle's write and
					// SyncWatchers refreshing the cache for it.
					downstreamOld.GetUID(): {
						Name:      downstreamOld.GetName(),
						Namespace: downstreamOld.GetNamespace(),
						Labels:    downstreamOld.GetLabels(),
						Hash:      reportutils.CalculateResourceHash(*downstreamOld),
						Data:      downstreamOld,
					},
				},
			},
		},
		log: logging.WithName("test-watch-manager"),
	}

	// The watch event for Kyverno's own write (the new content) arrives while
	// the cache is still stale.
	wm.handleUpdate(downstreamNew.DeepCopy(), secretGVR)

	assert.NotEmpty(t, client.updated, "handleUpdate must revert a hash mismatch while the cache is stale -- this is the defect: it cannot tell a pending Kyverno write apart from real user tampering")
}

// TestProcessUR_SyncWatchersMustCompleteBeforeReturn is the second half of the
// same race: it proves ProcessUR (generate_controller.go) actually leaves the
// watcher cache in the stale state the test above exploits, because it
// dispatches SyncWatchers in a goroutine (`go func(...) { c.watchManager.
// SyncWatchers(...) }`) instead of calling it inline before returning.
//
// This does NOT race real wall-clock time against a fixed sleep (an earlier
// version of this test did exactly that, comparing "elapsed" against the
// mock's delay, and was flaky under load: ~15/20 runs correctly caught the
// bug, but ~5/20 falsely passed because scheduling jitter let the background
// goroutine finish before the assertion ran). Instead it proves a dependency,
// not a timing window: the mocked RESTMapper blocks on a channel that only
// the test controls, and is never released until AFTER we've already
// observed whether ProcessUR returned. If ProcessUR returns anyway, that
// deterministically proves it does not wait for SyncWatchers, regardless of
// CPU contention. No channel/lock coordination with handleUpdate is needed
// (and must be avoided: SyncWatchers holds wm.lock for its whole body, so
// calling handleUpdate, which also needs wm.lock, while SyncWatchers is
// deliberately stalled inside that body would deadlock both goroutines --
// the mistake in that same earlier version).
func TestProcessUR_SyncWatchersMustCompleteBeforeReturn(t *testing.T) {
	// needsReports (called at the end of ProcessUR) dereferences the global
	// reporting configuration; set it explicitly for this test and restore the
	// previous value afterwards, mirroring TestProcessUR_ErrorResultMarksURFailed
	// below.
	prevReportingCfg := reportutils.ReportingCfg
	reportutils.ReportingCfg = reportutils.NewReportingConfig(nil)
	t.Cleanup(func() { reportutils.ReportingCfg = prevReportingCfg })

	policyName := "test-gpol-sync-race"
	secretGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}

	trigger := makeUnstructured("1", "", "v1", "Secret", "sync-modify-trigger", "default", "trigger-uid", map[string]string{
		"argocd.argoproj.io/secret-type": "repository",
	})

	downstreamOld := makeUnstructured("", "", "v1", "Secret", "sync-modify-trigger", "sync-modify-trigger", "downstream-uid", map[string]string{
		common.GeneratePolicyLabel:     policyName,
		common.GenerateTriggerUIDLabel: string(trigger.GetUID()),
	})
	require.NoError(t, unstructured.SetNestedField(downstreamOld.Object, "YmFy", "data", "foo")) // base64("bar")
	downstreamNew := downstreamOld.DeepCopy()
	require.NoError(t, unstructured.SetNestedField(downstreamNew.Object, "Z2l0bGFi", "data", "foo")) // base64("gitlab")

	release := make(chan struct{})
	var releaseOnce sync.Once
	unblockMock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblockMock) // never leak the blocked goroutine past this test, on any exit path

	mapper := &mockRESTMapper{fn: func(gk schema.GroupKind, version string) (*meta.RESTMapping, error) {
		if gk.Group != "" || gk.Kind != "Secret" {
			return nil, assert.AnError
		}
		<-release
		return &meta.RESTMapping{Resource: secretGVR}, nil
	}}

	client := &triggerClient{MockClient: &MockClient{}, trigger: trigger}
	wm := &WatchManager{
		client:     client,
		restMapper: mapper,
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{
			secretGVR: {
				watcher: watch.NewFake(),
				metadataCache: map[types.UID]Resource{
					downstreamOld.GetUID(): {
						Name:      downstreamOld.GetName(),
						Namespace: downstreamOld.GetNamespace(),
						Labels:    downstreamOld.GetLabels(),
						Hash:      reportutils.CalculateResourceHash(*downstreamOld),
						Data:      downstreamOld,
					},
				},
			},
		},
		policyRefs: map[string][]schema.GroupVersionResource{policyName: {secretGVR}},
		refCount:   map[schema.GroupVersionResource]int{secretGVR: 1},
		log:        logging.WithName("test-watch-manager"),
	}

	policy := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: policyName},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.GeneratingPolicyEvaluationConfiguration{
				SynchronizationConfiguration: &policiesv1beta1.SynchronizationConfiguration{Enabled: ptr.To(true)},
			},
		},
	}
	statusRecorder := &recordingStatusControl{}
	controller := &CELGenerateController{
		client:        client,
		restMapper:    mapper,
		context:       libs.NewFakeContextProvider(),
		engine:        &testEngine{generated: []*unstructured.Unstructured{downstreamNew}},
		provider:      &testProvider{policy: gpolengine.Policy{Policy: policy}},
		watchManager:  wm,
		statusControl: statusRecorder,
		eventGen:      testEventGen{},
		log:           logging.WithName("test-gpol-controller"),
	}

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-sync-race"},
		Spec: kyvernov2.UpdateRequestSpec{
			Type:   kyvernov2.CELGenerate,
			Policy: policyName,
			Context: kyvernov2.UpdateRequestSpecContext{
				// Mirrors the real "modify the trigger" step of the chainsaw
				// scenario (an UPDATE admission request drives the UR): without
				// this, ProcessUR resolves the trigger's own GVR via
				// c.restMapper.RESTMapping(...) synchronously on the main path
				// (generate_controller.go, before the engine even runs), and
				// since the trigger here is also a Secret, that call would hit
				// the SAME mocked RESTMapper as SyncWatchers' call below and
				// confound the two -- this test's mock must only ever see the
				// ONE RESTMapping call that matters (SyncWatchers', for the
				// generated downstream), not an unrelated one for the trigger.
				AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{
					AdmissionRequest: &admissionv1.AdmissionRequest{Operation: admissionv1.Update},
					Operation:        admissionv1.Update,
				},
			},
			RuleContext: []kyvernov2.RuleContext{{
				Rule:        "rule",
				Synchronize: true,
				// Deliberately no UID: with one set, common.GetTrigger routes
				// through common.GetResource's ListResource-by-UID branch, and
				// triggerClient (like the real production dclient wiring, and
				// like TestProcessUR_ErrorResultMarksURFailed below) only
				// overrides GetResource, not ListResource -- matches the same
				// UID-less pattern that test already uses for the same reason.
				Trigger: kyvernov1.ResourceSpec{
					APIVersion: trigger.GetAPIVersion(),
					Kind:       trigger.GetKind(),
					Namespace:  trigger.GetNamespace(),
					Name:       trigger.GetName(),
				},
			}},
		},
	}

	// Run ProcessUR on its own goroutine so its return can be observed via
	// select instead of being awaited directly -- the point of this test is
	// precisely whether it returns before or only after SyncWatchers'
	// (mocked) RESTMapping call is released.
	done := make(chan error, 1)
	go func() {
		done <- controller.ProcessUR(ur)
	}()

	select {
	case err := <-done:
		// ProcessUR returned WITHOUT us ever releasing the mocked RESTMapping
		// call inside SyncWatchers. It cannot have waited for SyncWatchers to
		// finish, so the watcher cache is not guaranteed to reflect this write
		// by the time callers (e.g. the webhook handler) see ProcessUR return.
		// This is deterministic regardless of CPU contention: it is not a
		// timing race, it is proof that the return value does not depend on
		// release at all.
		unblockMock()
		require.NoError(t, err)
		t.Fatal("ProcessUR returned before SyncWatchers (still blocked on the mocked RESTMapping call) could complete -- the cache refresh runs in a detached goroutine (`go func(...) { c.watchManager.SyncWatchers(...) }`), leaving a window where a watch event for this same write is reverted as user tampering (see TestDynamicWatcher_HandleUpdateRevertsOwnWriteDuringStaleCacheWindow)")
	case <-time.After(3 * time.Second):
		// ProcessUR has not returned: SyncWatchers must be running inline and
		// blocked on the mocked RESTMapping call. Release it and confirm
		// ProcessUR then completes, with the cache already synchronized.
		unblockMock()
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(3 * time.Second):
			t.Fatal("ProcessUR did not return even after releasing the mocked RESTMapping call")
		}
	}

	require.False(t, statusRecorder.failed, "UR must not be marked Failed: %s", statusRecorder.message)
	wm.lock.Lock()
	hash := wm.dynamicWatchers[secretGVR].metadataCache[downstreamOld.GetUID()].Hash
	wm.lock.Unlock()
	assert.Equal(t, reportutils.CalculateResourceHash(*downstreamNew), hash,
		"the watcher cache must reflect the new content once ProcessUR has fully completed (including its SyncWatchers dispatch)")
}

// Regression test for kyverno/kyverno#16983: when the engine evaluation
// returns an error result (e.g. a CEL evaluation error or a failure while
// creating the downstream resource), ProcessUR must mark the UpdateRequest
// as Failed with the error message instead of silently reporting it as
// Completed with nothing generated.
func TestProcessUR_ErrorResultMarksURFailed(t *testing.T) {
	policyName := "test-gpol"
	trigger := makeUnstructured("", "", "v1", "ConfigMap", "trigger-cm", "tenant-a", "trigger-uid", nil)
	// needsReports dereferences the global reporting configuration; set it
	// explicitly for this test and restore the previous value afterwards to
	// avoid order-dependent behavior across tests in this package.
	prevReportingCfg := reportutils.ReportingCfg
	reportutils.ReportingCfg = reportutils.NewReportingConfig(nil)
	t.Cleanup(func() { reportutils.ReportingCfg = prevReportingCfg })
	statusControl := &recordingStatusControl{}
	controller := &CELGenerateController{
		client: &triggerClient{
			MockClient: &MockClient{},
			trigger:    trigger,
		},
		restMapper: &mockRESTMapper{fn: func(gk schema.GroupKind, version string) (*meta.RESTMapping, error) {
			return &meta.RESTMapping{Resource: schema.GroupVersionResource{Group: gk.Group, Version: version, Resource: "configmaps"}}, nil
		}},
		context:       libs.NewFakeContextProvider(),
		engine:        &errorEngine{},
		provider:      &testProvider{policy: gpolengine.Policy{Policy: &policiesv1beta1.GeneratingPolicy{ObjectMeta: metav1.ObjectMeta{Name: policyName}}}},
		watchManager:  &WatchManager{},
		statusControl: statusControl,
		eventGen:      testEventGen{},
		log:           logging.WithName("test-gpol-controller"),
	}

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-error-result"},
		Spec: kyvernov2.UpdateRequestSpec{
			Type:   kyvernov2.CELGenerate,
			Policy: policyName,
			RuleContext: []kyvernov2.RuleContext{{
				Rule: "rule",
				Trigger: kyvernov1.ResourceSpec{
					APIVersion: trigger.GetAPIVersion(),
					Kind:       trigger.GetKind(),
					Namespace:  trigger.GetNamespace(),
					Name:       trigger.GetName(),
				},
			}},
		},
	}

	require.NoError(t, controller.ProcessUR(ur))
	assert.True(t, statusControl.failed, "UR should be marked as Failed")
	assert.False(t, statusControl.success, "UR should not be marked as Completed")
	assert.Contains(t, statusControl.message, "failed to evaluate policy")
}

const (
	mirrorPolicyName  = "tenant-settings-mirror"
	mirrorNamespace   = "tenant-a"
	mirrorTriggerUID  = "trigger-uid"
	mirrorDownstreamU = "downstream-uid"
)

// writeHookClient runs onWrite right after each successful UpdateResource. It
// stands in for the watch event of that write reaching handleUpdate before
// ProcessUR has refreshed the watcher cache.
type writeHookClient struct {
	dclient.Interface
	updates []*unstructured.Unstructured
	onWrite func(written *unstructured.Unstructured)
}

func (c *writeHookClient) UpdateResource(ctx context.Context, apiVersion, kind, namespace string, obj interface{}, dryRun bool, subresources ...string) (*unstructured.Unstructured, error) {
	res, err := c.Interface.UpdateResource(ctx, apiVersion, kind, namespace, obj, dryRun, subresources...)
	if err == nil {
		c.updates = append(c.updates, res.DeepCopy())
		if c.onWrite != nil {
			c.onWrite(res.DeepCopy())
		}
	}
	return res, err
}

func mirrorSpec(namespaced bool) policiesv1beta1.GeneratingPolicySpec {
	variables := []admissionregistrationv1.Variable{{
		Name: "configmap",
		Expression: `[
			{
				"kind": dyn("ConfigMap"),
				"apiVersion": dyn("v1"),
				"metadata": dyn({"name": "tenant-settings-mirror"}),
				"data": dyn({"QUOTA": string(object.data.quota)})
			}
		]`,
	}}
	apply := `generator.apply(variables.configmap)`
	if !namespaced {
		variables = append(variables, admissionregistrationv1.Variable{Name: "nsName", Expression: "object.metadata.namespace"})
		apply = `generator.apply(variables.nsName, variables.configmap)`
	}
	return policiesv1beta1.GeneratingPolicySpec{
		EvaluationConfiguration: &policiesv1beta1.GeneratingPolicyEvaluationConfiguration{
			SynchronizationConfiguration: &policiesv1beta1.SynchronizationConfiguration{Enabled: ptr.To(true)},
		},
		MatchConstraints: &admissionregistrationv1.MatchResources{
			ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"configmaps"},
					},
				},
			}},
		},
		Variables:  variables,
		Generation: []policiesv1beta1.Generation{{Expression: apply}},
	}
}

// mirrorFixture is a synchronized GeneratingPolicy or NamespacedGeneratingPolicy
// that mirrors a tenant ConfigMap, run through the real gpol engine and the
// real CEL context provider, so the labels on the downstream are the ones
// production stamps. The downstream already exists with the old quota and the
// watcher cache holds its hash.
type mirrorFixture struct {
	controller    *CELGenerateController
	wm            *WatchManager
	client        *writeHookClient
	statusControl *recordingStatusControl
	ur            *kyvernov2.UpdateRequest
	written       func() *unstructured.Unstructured
	cachedHash    func() string
}

var configMapGVR = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}

func newMirrorFixture(t *testing.T, policy policiesv1beta1.GeneratingPolicyLike, policyKey string) *mirrorFixture {
	t.Helper()
	prevReportingCfg := reportutils.ReportingCfg
	reportutils.ReportingCfg = reportutils.NewReportingConfig(nil)
	t.Cleanup(func() { reportutils.ReportingCfg = prevReportingCfg })
	// NewContextProvider also publishes itself as the process-wide libs context.
	prevLibsCtx := libs.LibraryContext
	t.Cleanup(func() { libs.LibraryContext = prevLibsCtx })

	compiled, errs := gpolcompiler.NewCompiler().Compile(policy, nil)
	require.Empty(t, errs)

	trigger := makeUnstructured("", "", "v1", "ConfigMap", "tenant-settings", mirrorNamespace, mirrorTriggerUID, nil)
	require.NoError(t, unstructured.SetNestedField(trigger.Object, "50", "data", "quota"))
	downstream := makeUnstructured("", "", "v1", "ConfigMap", "tenant-settings-mirror", mirrorNamespace, mirrorDownstreamU, map[string]string{
		kyverno.LabelAppManagedBy:      kyverno.ValueKyvernoApp,
		common.GeneratePolicyLabel:     mirrorPolicyName,
		common.GenerateTriggerUIDLabel: mirrorTriggerUID,
	})
	require.NoError(t, unstructured.SetNestedField(downstream.Object, "10", "data", "QUOTA"))

	fake, err := dclient.NewFakeClient(runtime.NewScheme(), nil, trigger, downstream)
	require.NoError(t, err)
	fake.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))
	client := &writeHookClient{Interface: fake}

	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Version: "v1"}})
	mapper.Add(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)
	libsCtx, err := libs.NewContextProvider(client, nil, nil, mapper, false)
	require.NoError(t, err)

	wm := &WatchManager{
		log:        logging.WithName("test-watch-manager"),
		client:     client,
		restMapper: mapper,
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{
			configMapGVR: {
				watcher: watch.NewFake(),
				metadataCache: map[types.UID]Resource{
					downstream.GetUID(): {
						Name:      downstream.GetName(),
						Namespace: downstream.GetNamespace(),
						Labels:    downstream.GetLabels(),
						Hash:      reportutils.CalculateResourceHash(*downstream),
						Data:      downstream,
					},
				},
			},
		},
		policyRefs: map[string][]schema.GroupVersionResource{policyKey: {configMapGVR}},
		refCount:   map[schema.GroupVersionResource]int{configMapGVR: 1},
	}
	nsResolver := func(name string) *corev1.Namespace {
		return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	}
	statusControl := &recordingStatusControl{}
	controller := &CELGenerateController{
		client:        client,
		restMapper:    mapper,
		context:       libsCtx,
		engine:        gpolengine.NewEngine(nsResolver, nil),
		provider:      &testProvider{policy: gpolengine.Policy{Policy: policy, CompiledPolicy: compiled}},
		watchManager:  wm,
		statusControl: statusControl,
		eventGen:      testEventGen{},
		log:           logging.WithName("test-gpol-controller"),
	}
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ur-mirror"},
		Spec: kyvernov2.UpdateRequestSpec{
			Type:   kyvernov2.CELGenerate,
			Policy: policyKey,
			RuleContext: []kyvernov2.RuleContext{{
				Rule:        "rule",
				Synchronize: true,
				Trigger: kyvernov1.ResourceSpec{
					APIVersion: trigger.GetAPIVersion(),
					Kind:       trigger.GetKind(),
					Namespace:  trigger.GetNamespace(),
					Name:       trigger.GetName(),
					UID:        trigger.GetUID(),
				},
			}},
		},
	}
	return &mirrorFixture{
		controller:    controller,
		wm:            wm,
		client:        client,
		statusControl: statusControl,
		ur:            ur,
		written: func() *unstructured.Unstructured {
			if len(client.updates) == 0 {
				return nil
			}
			return client.updates[0]
		},
		cachedHash: func() string {
			wm.lock.Lock()
			defer wm.lock.Unlock()
			return wm.dynamicWatchers[configMapGVR].metadataCache[downstream.GetUID()].Hash
		},
	}
}

func mirrorPolicies() []struct {
	name   string
	policy policiesv1beta1.GeneratingPolicyLike
	key    string
} {
	return []struct {
		name   string
		policy policiesv1beta1.GeneratingPolicyLike
		key    string
	}{
		{
			name: "GeneratingPolicy",
			policy: &policiesv1beta1.GeneratingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: mirrorPolicyName},
				Spec:       mirrorSpec(false),
			},
			key: mirrorPolicyName,
		},
		{
			name: "NamespacedGeneratingPolicy",
			policy: &policiesv1beta1.NamespacedGeneratingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: mirrorPolicyName, Namespace: mirrorNamespace},
				Spec:       mirrorSpec(true),
			},
			key: mirrorNamespace + "/" + mirrorPolicyName,
		},
	}
}

func quotaOf(t *testing.T, obj *unstructured.Unstructured) string {
	t.Helper()
	quota, _, err := unstructured.NestedString(obj.Object, "data", "QUOTA")
	require.NoError(t, err)
	return quota
}

// The watch event for the downstream write ProcessUR itself makes can reach
// handleUpdate before SyncWatchers has recorded that write's hash. That event
// must not be reverted to the previous content, for a cluster policy and for a
// namespaced one (whose UpdateRequest key is namespace/name while the label on
// the downstream carries the bare name).
func TestProcessUR_SyncDoesNotRevertOwnWriteBeforeCacheRefresh(t *testing.T) {
	for _, tc := range mirrorPolicies() {
		t.Run(tc.name, func(t *testing.T) {
			f := newMirrorFixture(t, tc.policy, tc.key)
			fired := false
			f.client.onWrite = func(written *unstructured.Unstructured) {
				if fired {
					return
				}
				fired = true
				f.wm.handleUpdate(written, configMapGVR)
			}

			require.NoError(t, f.controller.ProcessUR(f.ur))
			require.False(t, f.statusControl.failed, "UR must not be marked Failed: %s", f.statusControl.message)

			require.True(t, fired, "the generate call never wrote the downstream")
			require.Len(t, f.client.updates, 1, "only the generate write may reach the API server, a second update is a revert")
			assert.Equal(t, "50", quotaOf(t, f.client.updates[0]))
			assert.Equal(t, reportutils.CalculateResourceHash(*f.written()), f.cachedHash())
		})
	}
}

// A tenant edit that lands after ProcessUR's own write but before the watcher
// cache is refreshed is not reverted when its event arrives, so it has to be
// reverted once the generate window closes instead of being lost.
func TestProcessUR_SyncRevertsEditMadeBeforeCacheRefresh(t *testing.T) {
	for _, tc := range mirrorPolicies() {
		t.Run(tc.name, func(t *testing.T) {
			f := newMirrorFixture(t, tc.policy, tc.key)
			fired := false
			f.client.onWrite = func(written *unstructured.Unstructured) {
				if fired {
					return
				}
				fired = true
				edited := written.DeepCopy()
				require.NoError(t, unstructured.SetNestedField(edited.Object, "999", "data", "QUOTA"))
				f.wm.handleUpdate(written, configMapGVR)
				f.wm.handleUpdate(edited, configMapGVR)
			}

			require.NoError(t, f.controller.ProcessUR(f.ur))
			require.False(t, f.statusControl.failed, "UR must not be marked Failed: %s", f.statusControl.message)

			require.Len(t, f.client.updates, 2, "the generate write plus one revert of the edit")
			assert.Equal(t, "50", quotaOf(t, f.client.updates[0]))
			assert.Equal(t, "50", quotaOf(t, f.client.updates[1]))
			assert.Equal(t, reportutils.CalculateResourceHash(*f.written()), f.cachedHash())
		})
	}
}
