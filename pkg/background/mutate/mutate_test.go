package mutate

import (
	"errors"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/util/retry"
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

// newTargetClient returns a fake client whose first `conflicts` update calls fail
// with a conflict, plus a counter of how many update calls were made.
func newTargetClient(target *unstructured.Unstructured, conflicts int, failWith error) (dclient.Interface, *int) {
	scheme := runtime.NewScheme()
	gvk := target.GroupVersionKind()
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	listGVK := gvk
	listGVK.Kind += "List"
	scheme.AddKnownTypeWithName(listGVK, &unstructured.UnstructuredList{})

	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "configmaps"}: "ConfigMapList",
		},
		target,
	)

	updates := 0
	dyn.PrependReactor("update", "configmaps", func(k8stesting.Action) (bool, runtime.Object, error) {
		updates++
		if updates <= conflicts {
			return true, nil, failWith
		}
		return false, nil, nil
	})

	client := dclient.NewFakeClientWithDisco(dyn, kubefake.NewSimpleClientset(), dclient.NewFakeDiscoveryClient(nil))
	return client, &updates
}

func targetConfigMap() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":            "shared-target",
			"namespace":       "default",
			"resourceVersion": "1",
		},
		"data": map[string]interface{}{"registered": ""},
	}}
}

// mutationResponse builds the engine response the controller would get for a
// mutate-existing rule that patched `target`.
func mutationResponse(target *unstructured.Unstructured) engineapi.EngineResponse {
	rule := engineapi.RulePass("register", engineapi.Mutation, "", nil).
		WithPatchedTarget(target.DeepCopy(), metav1.GroupVersionResource{Version: "v1", Resource: "configmaps"}, "")
	var pr engineapi.PolicyResponse
	pr.Add(engineapi.ExecutionStats{}, *rule)
	return engineapi.EngineResponse{}.WithPolicyResponse(pr)
}

func conflictErr(name string) error {
	return apierrors.NewConflict(schema.GroupResource{Resource: "configmaps"}, name, errors.New("the object has been modified"))
}

// A conflict must reach the caller so it can recompute and retry, rather than
// being collected as a terminal error or reported as a failure.
func TestApplyMutations_ConflictIsReturnedNotCollected(t *testing.T) {
	t.Parallel()

	target := targetConfigMap()
	client, updates := newTargetClient(target, 1, conflictErr(target.GetName()))
	c := &mutateExistingController{client: client}

	reports, errs, conflict := c.applyMutations(logr.Discard(), "register", mutationResponse(target))

	assert.True(t, apierrors.IsConflict(conflict), "conflict must be returned to the caller")
	assert.Empty(t, errs, "a conflict is retryable and must not be collected as an error")
	assert.Empty(t, reports, "a conflict must not be reported before the retry settles")
	assert.Equal(t, 1, *updates)
}

// The mutation is recomputed on every attempt. Replaying the previously computed
// patch against a newer resourceVersion would overwrite the other writer instead
// of merging with it, so the recompute has to happen inside the retry.
func TestApplyMutations_RetryRecomputesTheMutation(t *testing.T) {
	t.Parallel()

	target := targetConfigMap()
	client, updates := newTargetClient(target, 1, conflictErr(target.GetName()))
	c := &mutateExistingController{client: client}

	recomputes := 0
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		recomputes++
		_, _, conflict := c.applyMutations(logr.Discard(), "register", mutationResponse(target))
		return conflict
	})

	assert.NoError(t, err)
	assert.Equal(t, 2, recomputes, "the mutation must be recomputed after a conflict, not replayed")
	assert.Equal(t, 2, *updates)
}

func TestApplyMutations_NonConflictErrorIsCollected(t *testing.T) {
	t.Parallel()

	target := targetConfigMap()
	failure := apierrors.NewInternalError(errors.New("boom"))
	client, updates := newTargetClient(target, 1, failure)
	c := &mutateExistingController{client: client}

	reports, errs, conflict := c.applyMutations(logr.Discard(), "register", mutationResponse(target))

	assert.NoError(t, conflict, "only conflicts are retryable")
	assert.Len(t, errs, 1)
	assert.Len(t, reports, 1, "a terminal failure is still reported")
	assert.Error(t, reports[0].err)
	assert.Equal(t, 1, *updates)
}

func TestApplyMutations_SuccessIsReportedOnce(t *testing.T) {
	t.Parallel()

	target := targetConfigMap()
	client, updates := newTargetClient(target, 0, nil)
	c := &mutateExistingController{client: client}

	reports, errs, conflict := c.applyMutations(logr.Discard(), "register", mutationResponse(target))

	assert.NoError(t, conflict)
	assert.Empty(t, errs)
	assert.Len(t, reports, 1)
	assert.NoError(t, reports[0].err)
	assert.Equal(t, "shared-target", reports[0].target.GetName())
	assert.Equal(t, 1, *updates)
}
