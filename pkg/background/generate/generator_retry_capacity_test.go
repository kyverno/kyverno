package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/background/generate/provenance"
	kyvernofake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	k8stesting "k8s.io/client-go/testing"
)

// The ledger has a lower limit than Kubernetes' total annotation limit, leaving
// room for unrelated metadata and the UpdateRequest's embedded admission data.
const testReceiptCapacity = 128 * 1024

func replaceCapacityRequest(t *testing.T, client *kyvernofake.Clientset, ur *kyvernov2.UpdateRequest) {
	t.Helper()
	require.NoError(t, client.Tracker().Update(kyvernov2.SchemeGroupVersion.WithResource("updaterequests"), ur, ur.Namespace))
}

func setCapacityReceipts(t *testing.T, client *kyvernofake.Clientset, ur *kyvernov2.UpdateRequest, receipts []generationReceipt) {
	t.Helper()
	encoded, err := json.Marshal(receipts)
	require.NoError(t, err)
	ur.Annotations[pendingProvenanceAnnotation] = string(encoded)
	replaceCapacityRequest(t, client, ur)
}

func capacityRoutingLabels(count int) map[string]any {
	labels := make(map[string]any, count)
	for i := range count {
		labels[fmt.Sprintf("generate.kyverno.io/custom-%04d", i)] = strings.Repeat("x", 63)
	}
	return labels
}

// Use Kubernetes' actual metadata validator rather than letting the ordinary
// fake tracker accept a ledger that the API server could never persist.
func rejectOversizedCapacityAnnotations(t *testing.T, client *kyvernofake.Clientset) *int {
	t.Helper()
	rejected := 0
	client.PrependReactor("update", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
		candidate := action.(k8stesting.UpdateAction).GetObject().(*kyvernov2.UpdateRequest)
		if errors := apivalidation.ValidateAnnotations(candidate.Annotations, field.NewPath("metadata", "annotations")); len(errors) != 0 {
			rejected++
			return true, nil, apierrors.NewInvalid(kyvernov2.SchemeGroupVersion.WithKind("UpdateRequest").GroupKind(), candidate.Name, errors)
		}
		return false, nil, nil
	})
	return &rejected
}

// Reporting configuration is global, so this controller/status regression must
// not run in parallel. Ninety-six targets are enough to exceed the real API
// annotation limit without a bound, while keeping the test inexpensive.
func TestProcessURReceiptCapacityStopsBeforeUnrecoverableWrites(t *testing.T) {
	previousReporting := reportutils.ReportingCfg
	reportutils.ReportingCfg = nil
	reportutils.NewReportingConfig(nil)
	t.Cleanup(func() { reportutils.ReportingCfg = previousReporting })

	gen, base := newProvenanceStampGenerator(t)
	gen.rule.Generation.Synchronize = false
	client := &provenanceRetryClient{provenanceStampClient: base, objects: map[string]*unstructured.Unstructured{
		retryObjectKey(gen.trigger.GetNamespace(), gen.trigger.GetName()): gen.trigger.DeepCopy(),
	}}
	names := make([]string, 96)
	for i := range names {
		names[i] = fmt.Sprintf("target-%03d", i)
	}
	encodedNames, err := json.Marshal(names)
	require.NoError(t, err)
	pattern := *gen.pattern.DeepCopy()
	pattern.Name = "{{element}}"
	pattern.SetData(map[string]any{
		"metadata": map[string]any{"labels": capacityRoutingLabels(32)},
		"data":     map[string]any{"key": "generated"},
	})
	gen.rule.Generation.GeneratePattern = kyvernov1.GeneratePattern{}
	gen.rule.Generation.ForEachGeneration = []kyvernov1.ForEachGeneration{{List: "`" + string(encodedNames) + "`", GeneratePattern: pattern}}
	gen.policy.GetSpec().Rules = []kyvernov1.Rule{gen.rule}
	ur, requests := newGenerationRetryRequest(t, gen)
	rejected := rejectOversizedCapacityAnnotations(t, requests)
	configuration := config.NewDefaultConfiguration(false)
	process := func(request *kyvernov2.UpdateRequest) {
		controller := NewGenerateController(client, requests, common.NewStatusControl(requests, nil), cleanupRetryEngine{},
			&fakeClusterPolicyLister{policy: gen.policy.(*kyvernov1.ClusterPolicy)}, nil, nil, nil, configuration, event.NewFake(), logr.Discard(), jmespath.New(configuration))
		require.NoError(t, controller.ProcessUR(request))
	}
	client.patchErr = apierrors.NewTimeoutError("temporary annotation failure", 1)
	process(ur.DeepCopy())
	failed, err := requests.KyvernoV2().UpdateRequests(ur.Namespace).Get(t.Context(), ur.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, kyvernov2.Failed, failed.Status.State)
	require.Contains(t, failed.Status.Message, "capacity")
	require.LessOrEqual(t, strings.Count(failed.Status.Message, "capacity"), 1, "one exhausted batch must not grow status per skipped element")
	require.Zero(t, *rejected, "stop before the API server rejects an oversized receipt write")
	receipts := retryReceipts(t, requests, ur)
	require.NotEmpty(t, receipts)
	require.Less(t, len(receipts), len(names))
	require.LessOrEqual(t, len(failed.Annotations[pendingProvenanceAnnotation]), testReceiptCapacity)
	require.Equal(t, len(receipts), client.createCount, "every created unsigned target must retain its own durable receipt")
	for _, receipt := range receipts {
		obj := client.objects[retryObjectKey(receipt.Target.Namespace, receipt.Target.Name)]
		require.NotNil(t, obj)
		require.Empty(t, obj.GetAnnotations()[provenance.Annotation])
		obj.Object["data"] = map[string]any{"key": "ordinary-user-edit"}
		obj.SetResourceVersion("42")
	}

	// A new reconciliation first stamps old targets, freeing their ledger space,
	// and then generates the rest without rewriting the original targets' data.
	client.patchErr = nil
	failed.Status.State = kyvernov2.Pending
	pending, err := requests.KyvernoV2().UpdateRequests(ur.Namespace).UpdateStatus(t.Context(), failed, metav1.UpdateOptions{})
	require.NoError(t, err)
	process(pending)
	completed, err := requests.KyvernoV2().UpdateRequests(ur.Namespace).Get(t.Context(), ur.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, kyvernov2.Completed, completed.Status.State, completed.Status.Message)
	require.Empty(t, retryReceipts(t, requests, ur))
	require.Equal(t, len(names), client.createCount)
	require.Zero(t, *rejected)
	for _, name := range names {
		obj := client.objects[retryObjectKey("tenant", name)]
		require.NotNil(t, obj)
		valid, err := gen.provenance.Verify(t.Context(), gen.policy, obj)
		require.NoError(t, err)
		require.True(t, valid, name)
	}
	for _, receipt := range receipts {
		require.Equal(t, "ordinary-user-edit", client.objects[retryObjectKey(receipt.Target.Namespace, receipt.Target.Name)].Object["data"].(map[string]any)["key"])
	}
}

type capacityDisappearingClient struct {
	*provenanceStampClient
	reads int
}

func (c *capacityDisappearingClient) GetResource(ctx context.Context, apiVersion, kind, namespace, name string, subresources ...string) (*unstructured.Unstructured, error) {
	c.reads++
	if c.reads > 1 {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: kind}, name)
	}
	return c.provenanceStampClient.GetResource(ctx, apiVersion, kind, namespace, name, subresources...)
}

func TestGenerateReceiptCapacityBlocksBeforeTargetWrite(t *testing.T) {
	t.Parallel()
	for _, budget := range []string{"unrelated annotations", "whole request", "routing labels"} {
		for _, action := range []string{"create", "update", "fallback create", "apply create", "apply update"} {
			t.Run(budget+"/"+action, func(t *testing.T) {
				t.Parallel()
				gen, client := newProvenanceStampGenerator(t)
				if strings.Contains(action, "update") || action == "fallback create" {
					client.target = &unstructured.Unstructured{Object: map[string]any{
						"apiVersion": "v1", "kind": "ConfigMap",
						"metadata": map[string]any{"name": "target", "namespace": "tenant", "uid": "existing-target", "resourceVersion": "1"},
						"data":     map[string]any{"key": "keep-existing-data"},
					}}
				}
				if action == "fallback create" {
					gen.client = &capacityDisappearingClient{provenanceStampClient: client}
				}
				gen.policy.GetSpec().UseServerSideApply = strings.HasPrefix(action, "apply")
				ur, requests := newGenerationRetryRequest(t, gen)
				switch budget {
				case "unrelated annotations":
					ur.Annotations["example.com/keep"] = strings.Repeat("x", 256*1024-1024)
				case "whole request":
					ur.Spec.Context.UserRequestInfo.Roles = []string{strings.Repeat("x", 1024*1024-2048)}
				case "routing labels":
					gen.pattern.SetData(map[string]any{"metadata": map[string]any{"labels": capacityRoutingLabels(1500)}})
				}
				replaceCapacityRequest(t, requests, ur)
				before := client.target.DeepCopy()
				_, err := gen.generate()
				require.ErrorContains(t, err, "capacity")
				require.Zero(t, client.createCount)
				require.Zero(t, client.updateCount)
				require.Zero(t, client.applyCount)
				require.Empty(t, client.patches)
				require.Equal(t, before, client.target, "an existing target must remain untouched")
				require.Empty(t, retryReceipts(t, requests, ur))
			})
		}
	}
}

func TestGenerateReceiptCapacityBlocksStampOnlyPatch(t *testing.T) {
	t.Parallel()
	gen, client := newProvenanceStampGenerator(t)
	_, err := gen.generate()
	require.NoError(t, err)
	annotations := client.target.GetAnnotations()
	delete(annotations, provenance.Annotation)
	client.target.SetAnnotations(annotations)
	ur, requests := newGenerationRetryRequest(t, gen)
	ur.Annotations["example.com/keep"] = strings.Repeat("x", 256*1024-256)
	replaceCapacityRequest(t, requests, ur)
	client.patchErr = apierrors.NewTimeoutError("temporary annotation failure", 1)
	patches := len(client.patches)
	err = gen.stampProvenance(t.Context(), client.target.DeepCopy(), client.target.GetLabels())
	require.ErrorContains(t, err, "capacity")
	require.Len(t, client.patches, patches)
	require.Empty(t, retryReceipts(t, requests, ur))
}

func TestGenerateReceiptCapacityDoesNotStarveLaterRecovery(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"cloneList", "foreach"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			gen, base := newProvenanceStampGenerator(t)
			gen.rule.Generation.Synchronize = false
			gen.pattern.Name = "zz-pending"
			client := &provenanceRetryClient{provenanceStampClient: base, objects: make(map[string]*unstructured.Unstructured)}
			gen.client = client
			ur, requests := newGenerationRetryRequest(t, gen)
			client.patchErr = apierrors.NewTimeoutError("temporary annotation failure", 1)
			_, err := gen.generate()
			require.Error(t, err)
			receipts := retryReceipts(t, requests, ur)
			require.Len(t, receipts, 1)
			original := receipts[0]
			filler := generationReceipt{Target: kyvernov1.ResourceSpec{Name: "other-rule"}, Stamp: "unused"}
			receipts = append(receipts, filler)
			encoded, err := json.Marshal(receipts)
			require.NoError(t, err)
			receipts[1].Stamp += strings.Repeat("x", testReceiptCapacity-len(encoded)-32)
			latest, err := requests.KyvernoV2().UpdateRequests(ur.Namespace).Get(t.Context(), ur.Name, metav1.GetOptions{})
			require.NoError(t, err)
			setCapacityReceipts(t, requests, latest, receipts)
			gen.pending.loaded = false
			client.patchErr = nil
			client.objects["tenant/zz-pending"].Object["data"] = map[string]any{"key": "ordinary-user-edit"}
			client.objects["tenant/zz-pending"].SetResourceVersion("42")
			if mode == "cloneList" {
				for _, name := range []string{"aa-new", "zz-pending"} {
					client.objects["sources/"+name] = &unstructured.Unstructured{Object: map[string]any{
						"apiVersion": "v1", "kind": "ConfigMap",
						"metadata": map[string]any{"name": name, "namespace": "sources", "uid": "source-" + name, "resourceVersion": "1"},
						"data":     map[string]any{"key": "generated"},
					}}
				}
				gen.pattern.RawData = nil
				gen.pattern.CloneList = kyvernov1.CloneList{Namespace: "sources", Kinds: []string{"v1/ConfigMap"}}
				_, err = gen.generate()
			} else {
				pattern := *gen.pattern.DeepCopy()
				pattern.Name = "{{element}}"
				gen.forEach = []kyvernov1.ForEachGeneration{{List: "`[\"aa-new\",\"zz-pending\"]`", GeneratePattern: pattern}}
				_, err = gen.generateForeach()
			}
			require.ErrorContains(t, err, "capacity")
			require.Equal(t, 1, client.createCount, "the earlier new target cannot be created without receipt capacity")
			require.Nil(t, client.objects["tenant/aa-new"])
			require.Equal(t, []generationReceipt{receipts[1]}, retryReceipts(t, requests, ur), "later receipts must still be retired when an earlier new target is blocked")
			recovered := client.objects["tenant/zz-pending"]
			require.Equal(t, original.Target.UID, recovered.GetUID())
			require.Equal(t, "ordinary-user-edit", recovered.Object["data"].(map[string]any)["key"])
			valid, err := gen.provenance.Verify(t.Context(), gen.policy, recovered)
			require.NoError(t, err)
			require.True(t, valid)
		})
	}
}

func TestGenerationReceiptCapacityCountsEncodedBytes(t *testing.T) {
	t.Parallel()
	gen, _ := newProvenanceStampGenerator(t)
	ur, requests := newGenerationRetryRequest(t, gen)
	receipt := generationReceipt{Target: kyvernov1.ResourceSpec{Name: "target"}, Labels: map[string]string{"escaped": "\"\\\n<>&"}, Stamp: "stamp"}
	encoded, err := json.Marshal([]generationReceipt{receipt})
	require.NoError(t, err)
	receipt.Stamp += strings.Repeat("x", testReceiptCapacity-len(encoded))
	require.NoError(t, gen.pending.save(t.Context(), receipt))
	latest, err := requests.KyvernoV2().UpdateRequests(ur.Namespace).Get(t.Context(), ur.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.Len(t, latest.Annotations[pendingProvenanceAnnotation], testReceiptCapacity)
	require.ErrorContains(t, gen.pending.save(t.Context(), generationReceipt{Target: kyvernov1.ResourceSpec{Name: "another"}, Stamp: "stamp"}), "capacity")
	require.Equal(t, []generationReceipt{receipt}, retryReceipts(t, requests, ur))
}

func TestGenerationReceiptCapacityRechecksAfterConflict(t *testing.T) {
	t.Parallel()
	gen, _ := newProvenanceStampGenerator(t)
	ur, requests := newGenerationRetryRequest(t, gen)
	rejected := rejectOversizedCapacityAnnotations(t, requests)
	attempts := 0
	requests.PrependReactor("update", "updaterequests", func(k8stesting.Action) (bool, runtime.Object, error) {
		attempts++
		if attempts != 1 {
			return false, nil, nil
		}
		concurrent := ur.DeepCopy()
		concurrent.Annotations["example.com/keep"] = strings.Repeat("x", 256*1024-128)
		replaceCapacityRequest(t, requests, concurrent)
		return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "updaterequests"}, ur.Name, fmt.Errorf("concurrent annotations"))
	})
	err := gen.pending.save(t.Context(), generationReceipt{Target: kyvernov1.ResourceSpec{Name: "target"}, Stamp: strings.Repeat("s", 256)})
	require.ErrorContains(t, err, "capacity")
	require.Equal(t, 1, attempts, "fresh metadata must be checked before retrying the write")
	require.Zero(t, *rejected)
	require.Empty(t, retryReceipts(t, requests, ur))
	latest, err := requests.KyvernoV2().UpdateRequests(ur.Namespace).Get(t.Context(), ur.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.Len(t, latest.Annotations["example.com/keep"], 256*1024-128)
}

func TestGenerationReceiptCapacityAllowsOversizedLedgerReduction(t *testing.T) {
	t.Parallel()
	gen, _ := newProvenanceStampGenerator(t)
	ur, requests := newGenerationRetryRequest(t, gen)
	receipts := make([]generationReceipt, 3)
	for i := range receipts {
		receipts[i] = generationReceipt{Target: kyvernov1.ResourceSpec{Name: fmt.Sprintf("old-%d", i)}, Stamp: strings.Repeat("s", 70*1024)}
	}
	setCapacityReceipts(t, requests, ur, receipts)
	rejected := rejectOversizedCapacityAnnotations(t, requests)
	require.NoError(t, gen.pending.remove(t.Context(), receipts[0]))
	latest, err := requests.KyvernoV2().UpdateRequests(ur.Namespace).Get(t.Context(), ur.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.Greater(t, len(latest.Annotations[pendingProvenanceAnnotation]), testReceiptCapacity, "recovery must make progress even before the ledger shrinks below the new bound")
	require.Equal(t, receipts[1:], retryReceipts(t, requests, ur))
	require.NoError(t, gen.pending.remove(t.Context(), receipts[1]))
	require.NoError(t, gen.pending.remove(t.Context(), receipts[2]))
	require.Empty(t, retryReceipts(t, requests, ur))
	require.Zero(t, *rejected)
}

type capacityReadErrorClient struct{ *provenanceStampClient }

func (c *capacityReadErrorClient) GetResource(ctx context.Context, apiVersion, kind, namespace, name string, subresources ...string) (*unstructured.Unstructured, error) {
	if name == "unavailable" {
		return nil, apierrors.NewTimeoutError("target read interrupted", 1)
	}
	return c.provenanceStampClient.GetResource(ctx, apiVersion, kind, namespace, name, subresources...)
}

func TestGenerateReceiptCapacityPreservesOtherForeachErrors(t *testing.T) {
	t.Parallel()
	gen, client := newProvenanceStampGenerator(t)
	gen.client = &capacityReadErrorClient{provenanceStampClient: client}
	ur, requests := newGenerationRetryRequest(t, gen)
	ur.Annotations["example.com/keep"] = strings.Repeat("x", 256*1024-1024)
	replaceCapacityRequest(t, requests, ur)
	pattern := *gen.pattern.DeepCopy()
	pattern.Name = "{{element}}"
	gen.forEach = []kyvernov1.ForEachGeneration{
		{List: "`[\"unavailable\",\"aa-new\",\"bb-new\"]`", GeneratePattern: pattern},
		{List: "`[\"cc-new\",\"dd-new\"]`", GeneratePattern: pattern},
	}
	_, err := gen.generateForeach()
	require.ErrorContains(t, err, "target read interrupted", "coalescing capacity failures must retain ordinary failures from earlier groups")
	require.ErrorContains(t, err, "capacity")
	require.Equal(t, 1, strings.Count(err.Error(), "capacity"), "capacity errors must be coalesced across foreach groups and elements")
	require.Zero(t, client.createCount)
	require.Empty(t, client.patches)
}
