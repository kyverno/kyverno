package gpol

import (
	"context"
	"sync"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	gpolengine "github.com/kyverno/kyverno/pkg/cel/policies/gpol/engine"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/logging"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

	require.NoError(t, controller.ProcessUR(context.TODO(), ur))
	assert.Zero(t, engine.calls)
}

func TestProcessUR_ConcurrentCacheRestoreAndGenerateExistingDoesNotDeleteDownstream(t *testing.T) {
	policyName := "test-gpol"
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
		require.NoError(t, controller.ProcessUR(context.TODO(), newUR(true)))
	}()
	go func() {
		defer wg.Done()
		require.NoError(t, controller.ProcessUR(context.TODO(), newUR(false)))
	}()
	wg.Wait()

	wm.lock.Lock()
	defer wm.lock.Unlock()
	assert.Len(t, client.deleted, 0)
	assert.Contains(t, wm.dynamicWatchers[configMapGVR].metadataCache, downstream.GetUID())
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
