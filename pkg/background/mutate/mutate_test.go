package mutate

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"go.uber.org/multierr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// mockStatusControl implements StatusControlInterface for testing
// It tracks which methods were called and allows simulating errors
type mockStatusControl struct {
	failedCalled  bool
	successCalled bool
	failedName    string
	successName   string
	failedMsg     string
	returnError   error
}

func (m *mockStatusControl) Failed(name string, message string, genResources []kyvernov1.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	m.failedCalled = true
	m.failedName = name
	m.failedMsg = message
	return nil, m.returnError
}

func (m *mockStatusControl) Success(name string, genResources []kyvernov1.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	m.successCalled = true
	m.successName = name
	return nil, m.returnError
}

func (m *mockStatusControl) Skip(name string, genResources []kyvernov1.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	return nil, m.returnError
}

func TestUpdateURStatus_SuccessCase(t *testing.T) {
	mock := &mockStatusControl{}
	ur := kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ur",
		},
	}

	err := updateURStatus(mock, ur, nil)

	assert.NoError(t, err)
	assert.True(t, mock.successCalled, "Success should be called when err is nil")
	assert.False(t, mock.failedCalled, "Failed should not be called when err is nil")
	assert.Equal(t, "test-ur", mock.successName)
}

func TestUpdateURStatus_FailureCase(t *testing.T) {
	mock := &mockStatusControl{}
	ur := kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ur",
		},
	}
	testErr := errors.New("mutation failed")

	err := updateURStatus(mock, ur, testErr)

	assert.NoError(t, err)
	assert.True(t, mock.failedCalled, "Failed should be called when err is not nil")
	assert.False(t, mock.successCalled, "Success should not be called when err is not nil")
	assert.Equal(t, "test-ur", mock.failedName)
	assert.Equal(t, "mutation failed", mock.failedMsg)
}

func TestUpdateURStatus_SuccessReturnsError(t *testing.T) {
	mock := &mockStatusControl{
		returnError: errors.New("status update failed"),
	}
	ur := kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ur",
		},
	}

	err := updateURStatus(mock, ur, nil)

	assert.Error(t, err)
	assert.Equal(t, "status update failed", err.Error())
	assert.True(t, mock.successCalled)
}

func TestUpdateURStatus_FailedReturnsError(t *testing.T) {
	mock := &mockStatusControl{
		returnError: errors.New("status update failed"),
	}
	ur := kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ur",
		},
	}
	testErr := errors.New("mutation failed")

	err := updateURStatus(mock, ur, testErr)

	assert.Error(t, err)
	assert.Equal(t, "status update failed", err.Error())
	assert.True(t, mock.failedCalled)
}

// newTargetClient returns a fake client for the given targets. Each target
// named in conflicts fails its first n updates with a conflict. The returned map
// counts update calls per target name.
func newTargetClient(targets []*unstructured.Unstructured, conflicts map[string]int, failWith error) (dclient.Interface, map[string]int) {
	scheme := runtime.NewScheme()
	gvk := targets[0].GroupVersionKind()
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	listGVK := gvk
	listGVK.Kind += "List"
	scheme.AddKnownTypeWithName(listGVK, &unstructured.UnstructuredList{})

	objs := make([]runtime.Object, 0, len(targets))
	for _, t := range targets {
		objs = append(objs, t)
	}
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "configmaps"}: "ConfigMapList",
		},
		objs...,
	)

	updates := map[string]int{}
	dyn.PrependReactor("update", "configmaps", func(action k8stesting.Action) (bool, runtime.Object, error) {
		name := action.(k8stesting.UpdateAction).GetObject().(*unstructured.Unstructured).GetName()
		updates[name]++
		if updates[name] <= conflicts[name] {
			return true, nil, failWith
		}
		return false, nil, nil
	})

	client := dclient.NewFakeClientWithDisco(dyn, kubefake.NewSimpleClientset(), dclient.NewFakeDiscoveryClient(nil))
	return client, updates
}

func configMap(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":            name,
			"namespace":       "default",
			"resourceVersion": "1",
		},
		"data": map[string]interface{}{"registered": ""},
	}}
}

// mutationResponse builds the engine response for a mutate-existing rule that
// patched each of the targets.
func mutationResponse(targets ...*unstructured.Unstructured) engineapi.EngineResponse {
	var pr engineapi.PolicyResponse
	for _, target := range targets {
		rule := engineapi.RulePass("register", engineapi.Mutation, "", nil).
			WithPatchedTarget(target.DeepCopy(), metav1.GroupVersionResource{Version: "v1", Resource: "configmaps"}, "")
		pr.Add(engineapi.ExecutionStats{}, *rule)
	}
	return engineapi.EngineResponse{}.WithPolicyResponse(pr)
}

func conflictErr(name string) error {
	return apierrors.NewConflict(schema.GroupResource{Resource: "configmaps"}, name, errors.New("the object has been modified"))
}

// mutatingEngine is an engine stub whose Mutate returns a fixed response and
// counts how often it is called; retryResponse, when set, is returned from the second call on.
type mutatingEngine struct {
	engineapi.Engine
	response      engineapi.EngineResponse
	retryResponse *engineapi.EngineResponse
	calls         int
}

func (e *mutatingEngine) Mutate(context.Context, engineapi.PolicyContext) engineapi.EngineResponse {
	e.calls++
	if e.calls > 1 && e.retryResponse != nil {
		return *e.retryResponse
	}
	return e.response
}

func TestApplyMutations_ConflictIsReturnedSeparately(t *testing.T) {
	t.Parallel()

	a := configMap("a")
	client, updates := newTargetClient([]*unstructured.Unstructured{a}, map[string]int{"a": 1}, conflictErr("a"))
	c := &mutateExistingController{client: client}

	reports, errs, conflicted, conflict := c.applyMutations(logr.Discard(), "register", mutationResponse(a), nil)

	assert.True(t, apierrors.IsConflict(conflict))
	assert.Len(t, conflicted, 1, "the conflicting target is handed back for a retry")
	assert.Empty(t, reports, "a conflict is not settled until the retries give up")
	assert.Empty(t, errs)
	assert.Equal(t, 1, updates["a"])
}

func TestApplyMutations_SkipsSettledTargets(t *testing.T) {
	t.Parallel()

	a, b := configMap("a"), configMap("b")
	client, updates := newTargetClient([]*unstructured.Unstructured{a, b}, nil, nil)
	c := &mutateExistingController{client: client}

	only := targetKeys([]targetMutation{{target: b}})
	reports, errs, conflicted, conflict := c.applyMutations(logr.Discard(), "register", mutationResponse(a, b), only)

	assert.NoError(t, conflict)
	assert.Empty(t, errs)
	assert.Empty(t, conflicted)
	assert.Len(t, reports, 1)
	assert.Equal(t, 0, updates["a"], "a target settled in an earlier attempt must not be patched again")
	assert.Equal(t, 1, updates["b"])
}

func TestApplyMutations_NonConflictErrorIsCollected(t *testing.T) {
	t.Parallel()

	a := configMap("a")
	client, updates := newTargetClient([]*unstructured.Unstructured{a}, map[string]int{"a": 1}, apierrors.NewInternalError(errors.New("boom")))
	c := &mutateExistingController{client: client}

	reports, errs, conflicted, conflict := c.applyMutations(logr.Discard(), "register", mutationResponse(a), nil)

	assert.NoError(t, conflict, "only conflicts are retryable")
	assert.Empty(t, conflicted)
	assert.Len(t, errs, 1)
	assert.Len(t, reports, 1, "a terminal failure is still reported")
	assert.Error(t, reports[0].err)
	assert.Equal(t, 1, updates["a"])
}

func TestApplyMutations_SuccessIsReportedOnce(t *testing.T) {
	t.Parallel()

	a := configMap("a")
	client, updates := newTargetClient([]*unstructured.Unstructured{a}, nil, nil)
	c := &mutateExistingController{client: client}

	reports, errs, conflicted, conflict := c.applyMutations(logr.Discard(), "register", mutationResponse(a), nil)

	assert.NoError(t, conflict)
	assert.Empty(t, errs)
	assert.Empty(t, conflicted)
	assert.Len(t, reports, 1)
	assert.NoError(t, reports[0].err)
	assert.Equal(t, 1, updates["a"])
}

// The engine has to run again inside the retry: replaying a patch computed
// against an older resourceVersion would overwrite the other writer.
func TestMutateWithRetry_RecomputesTheMutation(t *testing.T) {
	t.Parallel()

	a := configMap("a")
	client, updates := newTargetClient([]*unstructured.Unstructured{a}, map[string]int{"a": 1}, conflictErr("a"))
	engine := &mutatingEngine{response: mutationResponse(a)}
	c := &mutateExistingController{client: client, engine: engine}

	_, reports, errs, err := c.mutateWithRetry(logr.Discard(), "register", nil)

	assert.NoError(t, err)
	assert.Empty(t, errs)
	assert.Len(t, reports, 1, "the target is reported once")
	assert.Equal(t, 2, engine.calls, "the mutation must be recomputed after a conflict, not replayed")
	assert.Equal(t, 2, updates["a"])
}

// A retry must not patch a target that already succeeded, or a non-idempotent
// patch (an append, a counter) would be applied twice.
func TestMutateWithRetry_SettledTargetIsNotReapplied(t *testing.T) {
	t.Parallel()

	a, b := configMap("a"), configMap("b")
	client, updates := newTargetClient([]*unstructured.Unstructured{a, b}, map[string]int{"b": 1}, conflictErr("b"))
	c := &mutateExistingController{client: client, engine: &mutatingEngine{response: mutationResponse(a, b)}}

	_, reports, errs, err := c.mutateWithRetry(logr.Discard(), "register", nil)

	assert.NoError(t, err)
	assert.Empty(t, errs)
	assert.Len(t, reports, 2, "each target is reported exactly once")
	assert.Equal(t, 1, updates["a"], "a target that already succeeded must not be patched again")
	assert.Equal(t, 2, updates["b"])
}

// A conflict on one target must not stop the targets after it from being
// applied, even when that conflict outlives the retries.
func TestMutateWithRetry_TargetsAfterAConflictAreApplied(t *testing.T) {
	t.Parallel()

	b, cm := configMap("b"), configMap("c")
	client, updates := newTargetClient([]*unstructured.Unstructured{b, cm}, map[string]int{"b": 100}, conflictErr("b"))
	c := &mutateExistingController{client: client, engine: &mutatingEngine{response: mutationResponse(b, cm)}}

	_, reports, errs, err := c.mutateWithRetry(logr.Discard(), "register", nil)

	assert.True(t, apierrors.IsConflict(err), "the retries give up on the contended target")
	assert.Equal(t, 1, updates["c"], "the uncontended target is applied once, on the first attempt")
	assert.Greater(t, updates["b"], 1)
	assert.Len(t, reports, 2, "both targets are reported")
	assert.Len(t, errs, 1, "only the contended target is a terminal failure")
	assert.True(t, apierrors.IsConflict(errs[0]))
}

// A conflict that outlives the retries is a terminal failure with a report, not
// just a log line.
func TestMutateWithRetry_ExhaustedRetriesAreReportable(t *testing.T) {
	t.Parallel()

	a := configMap("a")
	client, _ := newTargetClient([]*unstructured.Unstructured{a}, map[string]int{"a": 100}, conflictErr("a"))
	c := &mutateExistingController{client: client, engine: &mutatingEngine{response: mutationResponse(a)}}

	_, reports, errs, err := c.mutateWithRetry(logr.Discard(), "register", nil)

	assert.True(t, apierrors.IsConflict(err))
	assert.Len(t, errs, 1)
	assert.Len(t, reports, 1)
	assert.True(t, apierrors.IsConflict(reports[0].err))
}

// A retry whose recompute fails to load the conflicted target must end as a terminal failure, not a silent success.
func TestMutateWithRetry_TargetLoadErrorOnRetryIsTerminal(t *testing.T) {
	t.Parallel()

	a := configMap("a")
	client, _ := newTargetClient([]*unstructured.Unstructured{a}, map[string]int{"a": 1}, conflictErr("a"))
	var pr engineapi.PolicyResponse
	pr.Add(engineapi.ExecutionStats{}, *engineapi.RuleError("register", engineapi.Mutation, "failed to load targets", errors.New("not found"), nil))
	retryResponse := engineapi.EngineResponse{}.WithPolicyResponse(pr)
	c := &mutateExistingController{client: client, engine: &mutatingEngine{response: mutationResponse(a), retryResponse: &retryResponse}}

	_, _, errs, err := c.mutateWithRetry(logr.Discard(), "register", nil)

	assert.NoError(t, err, "the retry itself did not conflict")
	assert.ErrorContains(t, multierr.Combine(errs...), "failed to load targets", "the target-load error must fail the update request")
}

// A conflicted target that drops out of the recomputed response keeps its conflict as a terminal failure.
func TestMutateWithRetry_VanishedTargetKeepsItsConflict(t *testing.T) {
	t.Parallel()

	a := configMap("a")
	client, _ := newTargetClient([]*unstructured.Unstructured{a}, map[string]int{"a": 1}, conflictErr("a"))
	empty := engineapi.EngineResponse{}
	c := &mutateExistingController{client: client, engine: &mutatingEngine{response: mutationResponse(a), retryResponse: &empty}}

	_, reports, errs, err := c.mutateWithRetry(logr.Discard(), "register", nil)

	assert.NoError(t, err)
	assert.Len(t, errs, 1)
	assert.True(t, apierrors.IsConflict(errs[0]))
	assert.Len(t, reports, 1)
}
