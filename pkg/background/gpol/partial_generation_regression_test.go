package gpol

import (
	"context"
	"fmt"
	"strings"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	gpolvalidation "github.com/kyverno/kyverno/pkg/cel/policies/gpol"
	gpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	gpolengine "github.com/kyverno/kyverno/pkg/cel/policies/gpol/engine"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/logging"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"
)

// These tests use real validation, CEL compilation, matching, generation,
// ProcessUR and WatchManager cleanup. API responses, policy/namespace lookup
// and status/event sinks are faked.
// Assert that acknowledged writes survive error paths for both trace modes.
type partialReproClient struct {
	dclient.Interface
	objects map[string]*unstructured.Unstructured
	deny    string
	writes  int
	deletes int
	dyn     dynamic.Interface
}

func (c *partialReproClient) GetDynamicInterface() dynamic.Interface { return c.dyn }

func (c *partialReproClient) GetResource(_ context.Context, _, kind, namespace, name string, _ ...string) (*unstructured.Unstructured, error) {
	if obj := c.objects[kind+"/"+namespace+"/"+name]; obj != nil {
		return obj.DeepCopy(), nil
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{Resource: strings.ToLower(kind) + "s"}, name)
}

func (c *partialReproClient) CreateResource(_ context.Context, _, kind, namespace string, object interface{}, _ bool) (*unstructured.Unstructured, error) {
	obj := object.(*unstructured.Unstructured).DeepCopy()
	if obj.GetName() == c.deny {
		return nil, apierrors.NewForbidden(schema.GroupResource{Resource: "configmaps"}, c.deny, fmt.Errorf("injected write refusal"))
	}
	c.writes++
	obj.SetUID(types.UID("uid-" + obj.GetName()))
	obj.SetResourceVersion(fmt.Sprint(100 + c.writes))
	c.objects[kind+"/"+namespace+"/"+obj.GetName()] = obj
	return obj.DeepCopy(), nil
}

func (c *partialReproClient) DeleteResource(_ context.Context, _, kind, namespace, name string, _ bool, _ metav1.DeleteOptions) error {
	c.deletes++
	delete(c.objects, kind+"/"+namespace+"/"+name)
	return nil
}

type partialReproStatus struct {
	failed    bool
	succeeded bool
	message   string
	resources []kyverno.ResourceSpec
}

func (s *partialReproStatus) Failed(_ string, message string, resources []kyverno.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	s.failed, s.message, s.resources = true, message, resources
	return nil, nil
}
func (s *partialReproStatus) Success(_ string, resources []kyverno.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	s.succeeded, s.resources = true, resources
	return nil, nil
}
func (*partialReproStatus) Skip(string, []kyverno.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	return nil, nil
}

// recordingPartialEngine delegates to the real engine and keeps its response.
type recordingPartialEngine struct {
	gpolengine.Engine
	response gpolengine.EngineResponse
}

func (e *recordingPartialEngine) Handle(ctx context.Context, request celengine.EngineRequest, policy gpolengine.Policy, restore bool) (gpolengine.EngineResponse, error) {
	response, err := e.Engine.Handle(ctx, request, policy, restore)
	e.response = response
	return response, err
}

func TestPartialGenerationTracksCommittedWrites(t *testing.T) {
	previousContext, previousReporting := libs.LibraryContext, reportutils.ReportingCfg
	t.Cleanup(func() { libs.LibraryContext, reportutils.ReportingCfg = previousContext, previousReporting })
	reportutils.ReportingCfg = reportutils.NewReportingConfig(nil)
	const namespace = "gpol-partial-repro"
	resource := func(name, value string) string {
		return fmt.Sprintf(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":%q},"data":{"value":%s}}`, name, value)
	}
	first, second := resource("first", `"ok"`), resource("second", `"ok"`)
	apply := func(resources ...string) string {
		return fmt.Sprintf(`generator.Apply(%q, [%s])`, namespace, strings.Join(resources, ","))
	}
	cases := []struct {
		name        string
		generations []string
		audit       string
		deny        string
		wantWrites  int
		wantError   string
	}{
		{"second_api_write_fails", []string{apply(first, second)}, "", "second", 1, "injected write refusal"},
		{"later_generation_expression_fails", []string{apply(first), apply(resource("second", "object.data.missing"))}, "", "", 1, "missing"},
		{"audit_annotation_fails_after_write", []string{apply(first)}, "string(object.data.missing)", "", 1, "auditAnnotation"},
		{"control_first_write_fails", []string{apply(first, second)}, "", "first", 0, "injected write refusal"},
		{"control_success", []string{apply(first, second)}, "", "", 2, ""},
	}
	for _, traced := range []bool{false, true} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s/trace=%v", tc.name, traced), func(t *testing.T) {
				trigger := makeUnstructured("50", "", "v1", "ConfigMap", "trigger", namespace, "trigger-uid", nil)
				trigger.Object["data"] = map[string]any{"present": "yes"}
				fakeDynamic := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
				fakeDynamic.PrependWatchReactor("*", func(clienttesting.Action) (bool, watch.Interface, error) {
					return true, watch.NewRaceFreeFake(), nil
				})
				client := &partialReproClient{objects: map[string]*unstructured.Unstructured{
					"ConfigMap/" + namespace + "/trigger": trigger,
				}, deny: tc.deny, dyn: fakeDynamic}
				mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Version: "v1"}})
				mapper.Add(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)
				library, err := libs.NewContextProvider(client, nil, nil, mapper, false)
				require.NoError(t, err)
				policy := &policiesv1beta1.GeneratingPolicy{
					TypeMeta:   metav1.TypeMeta{APIVersion: "policies.kyverno.io/v1", Kind: "GeneratingPolicy"},
					ObjectMeta: metav1.ObjectMeta{Name: "partial-repro"},
					Spec: policiesv1beta1.GeneratingPolicySpec{
						EvaluationConfiguration: &policiesv1beta1.GeneratingPolicyEvaluationConfiguration{
							SynchronizationConfiguration: &policiesv1beta1.SynchronizationConfiguration{Enabled: ptr.To(true)},
						},
						MatchConstraints: &admissionregistrationv1.MatchResources{ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
							ResourceNames: []string{"trigger"},
							RuleWithOperations: admissionregistrationv1.RuleWithOperations{
								Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
								Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"configmaps"}},
							},
						}}},
					},
				}
				for _, expression := range tc.generations {
					policy.Spec.Generation = append(policy.Spec.Generation, policiesv1beta1.Generation{Expression: expression})
				}
				if tc.audit != "" {
					policy.Spec.AuditAnnotations = []admissionregistrationv1.AuditAnnotation{{Key: "reason", ValueExpression: tc.audit}}
				}
				_, err = gpolvalidation.Validate(policy)
				require.NoError(t, err, "the policy must pass Kyverno's policy validation")
				compiled, errs := gpolcompiler.NewCompilerWithTrace(traced).Compile(policy, nil)
				require.Empty(t, errs)
				engine := &recordingPartialEngine{Engine: gpolengine.NewEngine(func(name string) *corev1.Namespace {
					return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
				}, matching.NewMatcher())}
				wm := &WatchManager{client: client, restMapper: mapper,
					dynamicWatchers: map[schema.GroupVersionResource]*watcher{},
					policyRefs:      map[string][]schema.GroupVersionResource{},
					refCount:        map[schema.GroupVersionResource]int{}, log: logging.WithName("partial-repro")}
				t.Cleanup(wm.StopWatchers)
				status := &partialReproStatus{}
				controller := &CELGenerateController{
					client: client, restMapper: mapper, context: library, engine: engine,
					provider:     &testProvider{policy: gpolengine.Policy{Policy: policy, CompiledPolicy: compiled}},
					watchManager: wm, statusControl: status, eventGen: testEventGen{}, log: logging.WithName("partial-repro"),
				}
				ur := &kyvernov2.UpdateRequest{ObjectMeta: metav1.ObjectMeta{Name: "partial-repro"}, Spec: kyvernov2.UpdateRequestSpec{
					Type: kyvernov2.CELGenerate, Policy: policy.Name,
					RuleContext: []kyvernov2.RuleContext{{Rule: "generate", Trigger: common.ResourceSpecFromUnstructured(*trigger)}},
				}}
				require.NoError(t, controller.ProcessUR(ur), "evaluation failure is stored through StatusControl")
				require.Equal(t, tc.wantWrites, client.writes)
				tracked := len(wm.GetDownstreams(policy.Name))
				if tc.wantError != "" {
					require.True(t, status.failed)
					require.False(t, status.succeeded)
					require.Contains(t, status.message, tc.wantError)
					require.Len(t, status.resources, tc.wantWrites, "committed downstream identities must survive evaluation failure")
					require.Len(t, engine.response.Policies[0].Result.GeneratedResources(), tc.wantWrites)
					require.Equal(t, tc.wantWrites, tracked)
				} else {
					require.True(t, status.succeeded)
					require.False(t, status.failed)
					require.Len(t, status.resources, tc.wantWrites)
					require.Equal(t, tc.wantWrites, tracked)
				}
				// Check identities in addition to counts, including on failed URs.
				for index, resource := range status.resources {
					expectedName := "first"
					if index == 1 {
						expectedName = "second"
					}
					require.Equal(t, "v1", resource.APIVersion)
					require.Equal(t, "ConfigMap", resource.Kind)
					require.Equal(t, namespace, resource.Namespace)
					require.Equal(t, expectedName, resource.Name)
					require.Equal(t, types.UID("uid-"+expectedName), resource.UID, "UR status must preserve the acknowledged UID to prevent replacement adoption")
				}
				// Invoke the exact cleanup operation used by policy deletion.
				wm.RemoveWatchersForPolicy(policy.Name, true)
				remaining := len(client.objects) - 1 // exclude the trigger
				require.Zero(t, remaining, "all acknowledged downstreams must be discoverable for cleanup")
				require.Equal(t, tc.wantWrites, client.deletes)
				t.Logf("committed=%d tracked=%d failed=%v statusResources=%d remainingAfterPolicyCleanup=%d", client.writes, tracked, status.failed, len(status.resources), remaining)
			})
		}
	}
}

func TestRetainAcknowledgedDownstreamsValidatesOwnershipAndUID(t *testing.T) {
	const namespace = "team-a"
	const policyName = "my-policy"
	labels := map[string]string{
		kyverno.LabelAppManagedBy:      kyverno.ValueKyvernoApp,
		common.GeneratePolicyLabel:     policyName,
		common.GenerateTriggerNSLabel:  namespace,
		common.GenerateTriggerUIDLabel: "trigger-uid",
	}
	old := makeUnstructured("1", "", "v1", "ConfigMap", "old", namespace, "old-uid", labels)
	foreign := makeUnstructured("1", "", "v1", "ConfigMap", "foreign", namespace, "foreign-uid", map[string]string{
		common.GeneratePolicyLabel:     policyName,
		common.GenerateTriggerNSLabel:  namespace,
		common.GenerateTriggerUIDLabel: "different-trigger",
	})
	replaced := makeUnstructured("1", "", "v1", "ConfigMap", "replaced", namespace, "new-uid", labels)
	client := &partialReproClient{objects: map[string]*unstructured.Unstructured{
		"ConfigMap/" + namespace + "/old":      old,
		"ConfigMap/" + namespace + "/foreign":  foreign,
		"ConfigMap/" + namespace + "/replaced": replaced,
	}}
	toSpec := func(name string, uid types.UID) kyverno.ResourceSpec {
		return kyverno.ResourceSpec{APIVersion: "v1", Kind: "ConfigMap", Namespace: namespace, Name: name, UID: uid}
	}
	ur := &kyvernov2.UpdateRequest{
		Spec: kyvernov2.UpdateRequestSpec{Policy: namespace + "/" + policyName},
		Status: kyvernov2.UpdateRequestStatus{GeneratedResources: []kyverno.ResourceSpec{
			toSpec("old", old.GetUID()), toSpec("old", old.GetUID()),
			toSpec("foreign", foreign.GetUID()), toSpec("replaced", "stale-uid"),
		}},
	}
	controller := &CELGenerateController{client: client}
	got := controller.retainAcknowledgedDownstreams(context.Background(), ur, kyverno.ResourceSpec{UID: "trigger-uid"}, nil)
	require.Len(t, got, 1)
	require.Equal(t, old.GetUID(), got[0].GetUID())
}

func TestPartialGenerationRetryStatusRestoresPriorResources(t *testing.T) {
	previousReporting := reportutils.ReportingCfg
	reportutils.ReportingCfg = reportutils.NewReportingConfig(nil)
	t.Cleanup(func() { reportutils.ReportingCfg = previousReporting })

	const namespace = "team-a"
	const policyName = "partial-retry"
	trigger := makeUnstructured("50", "", "v1", "ConfigMap", "trigger", namespace, "trigger-uid", nil)
	old := makeUnstructured("100", "", "v1", "ConfigMap", "old", namespace, "old-uid", map[string]string{
		kyverno.LabelAppManagedBy:      kyverno.ValueKyvernoApp,
		common.GeneratePolicyLabel:     policyName,
		common.GenerateTriggerNSLabel:  namespace,
		common.GenerateTriggerUIDLabel: "trigger-uid",
	})
	fakeDynamic := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	fakeDynamic.PrependWatchReactor("*", func(clienttesting.Action) (bool, watch.Interface, error) {
		return true, watch.NewRaceFreeFake(), nil
	})
	client := &partialReproClient{
		objects: map[string]*unstructured.Unstructured{
			"ConfigMap/" + namespace + "/trigger": trigger,
			"ConfigMap/" + namespace + "/old":     old,
		},
		dyn: fakeDynamic,
	}
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Version: "v1"}})
	mapper.Add(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)
	library, err := libs.NewContextProvider(client, nil, nil, mapper, false)
	require.NoError(t, err)
	policy := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: policyName},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.GeneratingPolicyEvaluationConfiguration{
				SynchronizationConfiguration: &policiesv1beta1.SynchronizationConfiguration{Enabled: ptr.To(true)},
			},
		},
	}
	watchManager := &WatchManager{
		client: client, restMapper: mapper, log: logging.WithName("partial-retry"),
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{},
		policyRefs: map[string][]schema.GroupVersionResource{},
		refCount:   map[schema.GroupVersionResource]int{},
	}
	t.Cleanup(watchManager.StopWatchers)
	status := &partialReproStatus{}
	controller := &CELGenerateController{
		client: client, restMapper: mapper, context: library,
		engine: &errorEngine{},
		provider: &testProvider{policy: gpolengine.Policy{Policy: policy}},
		watchManager: watchManager, statusControl: status, eventGen: testEventGen{},
		log: logging.WithName("partial-retry"),
	}
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "partial-retry"},
		Spec: kyvernov2.UpdateRequestSpec{
			Type: kyvernov2.CELGenerate, Policy: policyName,
			RuleContext: []kyvernov2.RuleContext{{
				Rule: "generate", Trigger: common.ResourceSpecFromUnstructured(*trigger), CacheRestore: true,
			}},
		},
		Status: kyvernov2.UpdateRequestStatus{
			State: kyvernov2.Pending, RetryCount: 1,
			GeneratedResources: []kyverno.ResourceSpec{common.ResourceSpecFromUnstructured(*old)},
		},
	}
	require.NoError(t, controller.ProcessUR(ur))
	require.True(t, status.failed, "retry must retain its failure verdict")
	require.Len(t, status.resources, 1)
	require.Equal(t, "old", status.resources[0].Name)
	require.Equal(t, namespace, status.resources[0].Namespace)
	require.Equal(t, old.GetUID(), status.resources[0].UID, "a retry must retain the original downstream UID")
	require.Len(t, watchManager.GetDownstreams(policyName), 1)
	watchManager.RemoveWatchersForPolicy(policyName, true)
	require.Equal(t, 1, client.deletes)
	require.NotContains(t, client.objects, "ConfigMap/" + namespace + "/old")
}

func TestPartialGenerationSuccessfulRetryCleansStaleSameKind(t *testing.T) {
	previousReporting := reportutils.ReportingCfg
	reportutils.ReportingCfg = reportutils.NewReportingConfig(nil)
	t.Cleanup(func() { reportutils.ReportingCfg = previousReporting })

	const namespace = "team-a"
	const policyName = "partial-retry-success"
	trigger := makeUnstructured("50", "", "v1", "ConfigMap", "trigger", namespace, "trigger-uid", nil)
	labels := map[string]string{
		kyverno.LabelAppManagedBy:      kyverno.ValueKyvernoApp,
		common.GeneratePolicyLabel:     policyName,
		common.GenerateTriggerNSLabel:  namespace,
		common.GenerateTriggerUIDLabel: "trigger-uid",
	}
	old := makeUnstructured("100", "", "v1", "ConfigMap", "old", namespace, "old-uid", labels)
	newResource := makeUnstructured("101", "", "v1", "ConfigMap", "new", namespace, "new-uid", labels)
	client := &partialReproClient{objects: map[string]*unstructured.Unstructured{
		"ConfigMap/" + namespace + "/trigger": trigger,
		"ConfigMap/" + namespace + "/old":     old,
		"ConfigMap/" + namespace + "/new":     newResource,
	}}
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{{Version: "v1"}})
	mapper.Add(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, meta.RESTScopeNamespace)
	library, err := libs.NewContextProvider(client, nil, nil, mapper, false)
	require.NoError(t, err)
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	watchManager := &WatchManager{
		client: client, restMapper: mapper, log: logging.WithName("partial-success-retry"),
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{
			gvr: {
				watcher: watch.MockWatcher{StopFunc: func() {}},
				metadataCache: map[types.UID]Resource{
					old.GetUID(): {Name: old.GetName(), Namespace: namespace, Labels: labels, Hash: "old-hash", Data: old},
				},
			},
		},
		policyRefs: map[string][]schema.GroupVersionResource{policyName: {gvr}},
		refCount:   map[schema.GroupVersionResource]int{gvr: 1},
	}
	t.Cleanup(watchManager.StopWatchers)
	policy := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: policyName},
		Spec: policiesv1beta1.GeneratingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.GeneratingPolicyEvaluationConfiguration{
				SynchronizationConfiguration: &policiesv1beta1.SynchronizationConfiguration{Enabled: ptr.To(true)},
			},
		},
	}
	status := &partialReproStatus{}
	controller := &CELGenerateController{
		client: client, restMapper: mapper, context: library,
		engine: &testEngine{generated: []*unstructured.Unstructured{newResource}},
		provider: &testProvider{policy: gpolengine.Policy{Policy: policy}},
		watchManager: watchManager, statusControl: status, eventGen: testEventGen{},
		log: logging.WithName("partial-success-retry"),
	}
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "partial-success-retry"},
		Spec: kyvernov2.UpdateRequestSpec{
			Type: kyvernov2.CELGenerate, Policy: policyName,
			RuleContext: []kyvernov2.RuleContext{{
				Rule: "generate", Trigger: common.ResourceSpecFromUnstructured(*trigger),
				Synchronize: false,
			}},
		},
		Status: kyvernov2.UpdateRequestStatus{
			State: kyvernov2.Pending, RetryCount: 1,
			GeneratedResources: []kyverno.ResourceSpec{common.ResourceSpecFromUnstructured(*old)},
		},
	}
	require.NoError(t, controller.ProcessUR(ur))
	require.True(t, status.succeeded)
	require.Equal(t, 1, client.deletes)
	require.NotContains(t, client.objects, "ConfigMap/" + namespace + "/old")
	require.Contains(t, client.objects, "ConfigMap/" + namespace + "/new")
}
