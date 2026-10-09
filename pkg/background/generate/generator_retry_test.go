package generate

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"
)

// Keep all resources across reconciliations while retaining the real JSON-patch
// evaluator and server-assigned identities from provenanceStampClient.
type provenanceRetryClient struct {
	*provenanceStampClient
	objects map[string]*unstructured.Unstructured
}

func retryObjectKey(namespace, name string) string { return namespace + "/" + name }

func (c *provenanceRetryClient) GetResource(_ context.Context, _, kind, namespace, name string, _ ...string) (*unstructured.Unstructured, error) {
	if obj := c.objects[retryObjectKey(namespace, name)]; obj != nil {
		return obj.DeepCopy(), nil
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{Resource: kind}, name)
}

func (c *provenanceRetryClient) ListResource(_ context.Context, _, kind, namespace string, selector *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	selected := labels.Everything()
	if selector != nil {
		var err error
		selected, err = metav1.LabelSelectorAsSelector(selector)
		if err != nil {
			return nil, err
		}
	}
	list := &unstructured.UnstructuredList{}
	for _, obj := range c.objects {
		if obj.GetNamespace() == namespace && obj.GetKind() == kind && selected.Matches(labels.Set(obj.GetLabels())) {
			list.Items = append(list.Items, *obj.DeepCopy())
		}
	}
	sort.Slice(list.Items, func(i, j int) bool { return list.Items[i].GetName() < list.Items[j].GetName() })
	return list, nil
}

func (c *provenanceRetryClient) CreateResource(ctx context.Context, apiVersion, kind, namespace string, obj any, dryRun bool) (*unstructured.Unstructured, error) {
	key := retryObjectKey(namespace, obj.(*unstructured.Unstructured).GetName())
	c.target = c.objects[key]
	persisted, err := c.provenanceStampClient.CreateResource(ctx, apiVersion, kind, namespace, obj, dryRun)
	c.objects[key] = c.target
	return persisted, err
}

func (c *provenanceRetryClient) UpdateResource(ctx context.Context, apiVersion, kind, namespace string, obj any, dryRun bool, subresources ...string) (*unstructured.Unstructured, error) {
	key := retryObjectKey(namespace, obj.(*unstructured.Unstructured).GetName())
	c.target = c.objects[key]
	persisted, err := c.provenanceStampClient.UpdateResource(ctx, apiVersion, kind, namespace, obj, dryRun, subresources...)
	c.objects[key] = c.target
	return persisted, err
}

func (c *provenanceRetryClient) PatchResource(ctx context.Context, apiVersion, kind, namespace, name string, patch []byte) (*unstructured.Unstructured, error) {
	key := retryObjectKey(namespace, name)
	c.target = c.objects[key]
	persisted, err := c.provenanceStampClient.PatchResource(ctx, apiVersion, kind, namespace, name, patch)
	c.objects[key] = c.target
	return persisted, err
}

func newGenerationRetryRequest(t *testing.T, gen *generator) (*kyvernov2.UpdateRequest, *kyvernofake.Clientset) {
	t.Helper()
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "generate-retry", Namespace: config.KyvernoNamespace(), UID: "request-uid", ResourceVersion: "1",
			Annotations: map[string]string{provenance.PolicyUIDAnnotation: string(gen.policy.GetUID())},
		},
		Spec: kyvernov2.UpdateRequestSpec{
			Type: kyvernov2.Generate, Policy: gen.policy.GetName(),
			RuleContext: []kyvernov2.RuleContext{{Rule: gen.rule.Name, Trigger: common.ResourceSpecFromUnstructured(gen.trigger)}},
		},
	}
	ur.Spec.RuleContext = nil
	for _, rule := range gen.policy.GetSpec().Rules {
		ur.Spec.RuleContext = append(ur.Spec.RuleContext, kyvernov2.RuleContext{Rule: rule.Name, Trigger: common.ResourceSpecFromUnstructured(gen.trigger)})
	}
	client := kyvernofake.NewSimpleClientset(ur)
	// Unlike the ordinary fake tracker, enforce fresh resourceVersions for both
	// metadata and status writes. Receipt writes must not make status stale.
	version := 1
	client.PrependReactor("update", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
		candidate := action.(k8stesting.UpdateAction).GetObject().(*kyvernov2.UpdateRequest)
		stored, err := client.Tracker().Get(kyvernov2.SchemeGroupVersion.WithResource("updaterequests"), ur.Namespace, ur.Name)
		require.NoError(t, err)
		current := stored.(*kyvernov2.UpdateRequest)
		if candidate.ResourceVersion != current.ResourceVersion {
			return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "updaterequests"}, ur.Name, fmt.Errorf("stale resourceVersion"))
		}
		updated := candidate.DeepCopy()
		if action.GetSubresource() == "status" {
			updated = current.DeepCopy()
			updated.Status = candidate.Status
		} else {
			updated.Status = current.Status
		}
		version++
		updated.ResourceVersion = strconv.Itoa(version)
		require.NoError(t, client.Tracker().Update(kyvernov2.SchemeGroupVersion.WithResource("updaterequests"), updated, ur.Namespace))
		return true, updated, nil
	})
	gen.pending = &generationRetry{requests: client.KyvernoV2().UpdateRequests(ur.Namespace), name: ur.Name, uid: ur.UID, policyUID: gen.policy.GetUID()}
	return ur, client
}

func retryReceipts(t *testing.T, client *kyvernofake.Clientset, ur *kyvernov2.UpdateRequest) []generationReceipt {
	t.Helper()
	latest, err := client.KyvernoV2().UpdateRequests(ur.Namespace).Get(t.Context(), ur.Name, metav1.GetOptions{})
	require.NoError(t, err)
	var receipts []generationReceipt
	if raw := latest.Annotations[pendingProvenanceAnnotation]; raw != "" {
		require.NoError(t, json.Unmarshal([]byte(raw), &receipts))
	}
	return receipts
}

// Reporting configuration is global, so this actual controller/status lifecycle
// regression deliberately does not run in parallel.
func TestProcessURRetriesProvenanceAfterControllerRestart(t *testing.T) {
	previousReporting := reportutils.ReportingCfg
	reportutils.ReportingCfg = nil
	reportutils.NewReportingConfig(nil)
	t.Cleanup(func() { reportutils.ReportingCfg = previousReporting })
	for _, testName := range []string{"data", "clone", "cloneList", "foreach", "multiple rules", "clone missing source", "cloneList missing source", "foreach empty list"} {
		t.Run(testName, func(t *testing.T) {
			mode := strings.TrimSuffix(strings.TrimSuffix(testName, " missing source"), " empty list")
			gen, base := newProvenanceStampGenerator(t)
			gen.rule.Generation.Synchronize = false
			client := &provenanceRetryClient{provenanceStampClient: base, objects: map[string]*unstructured.Unstructured{
				retryObjectKey(gen.trigger.GetNamespace(), gen.trigger.GetName()): gen.trigger.DeepCopy(),
			}}
			names := []string{"target"}
			if mode == "clone" || mode == "cloneList" {
				gen.rule.Generation.RawData = nil
				source := &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": "v1", "kind": "ConfigMap",
					"metadata": map[string]any{"name": "source", "namespace": "sources", "uid": "source-uid", "resourceVersion": "1"},
					"data":     map[string]any{"key": "generated"},
				}}
				client.objects["sources/source"] = source
				if mode == "clone" {
					gen.rule.Generation.Clone = kyvernov1.CloneFrom{Namespace: "sources", Name: "source"}
				} else {
					gen.rule.Generation.CloneList = kyvernov1.CloneList{Namespace: "sources", Kinds: []string{"v1/ConfigMap"}}
					names = []string{"source", "source-two"}
					second := source.DeepCopy()
					second.SetName("source-two")
					second.SetUID("source-two-uid")
					client.objects["sources/source-two"] = second
				}
			} else if mode == "foreach" {
				pattern := *gen.pattern.DeepCopy()
				pattern.Name = "{{element}}"
				gen.rule.Generation.GeneratePattern = kyvernov1.GeneratePattern{}
				gen.rule.Generation.ForEachGeneration = []kyvernov1.ForEachGeneration{{List: "`[\"first\",\"second\"]`", GeneratePattern: pattern}}
				names = []string{"first", "second"}
			}
			gen.policy.GetSpec().Rules = []kyvernov1.Rule{gen.rule}
			if mode == "multiple rules" {
				second := gen.rule.DeepCopy()
				second.Name, second.Generation.Name = "generate-other", "other-target"
				gen.policy.GetSpec().Rules = append(gen.policy.GetSpec().Rules, *second)
				names = []string{"target", "other-target"}
			}
			ur, kyvernoClient := newGenerationRetryRequest(t, gen)
			requests := kyvernoClient.KyvernoV2().UpdateRequests(ur.Namespace)
			configuration := config.NewDefaultConfiguration(false)
			process := func(request *kyvernov2.UpdateRequest) {
				controller := NewGenerateController(client, kyvernoClient, common.NewStatusControl(kyvernoClient, nil), cleanupRetryEngine{},
					&fakeClusterPolicyLister{policy: gen.policy.(*kyvernov1.ClusterPolicy)}, nil, nil, nil, configuration, event.NewFake(), logr.Discard(), jmespath.New(configuration))
				require.NoError(t, controller.ProcessUR(request))
			}
			client.patchErr = apierrors.NewTimeoutError("temporary annotation failure", 1)
			process(ur.DeepCopy())
			failed, err := requests.Get(t.Context(), ur.Name, metav1.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, kyvernov2.Failed, failed.Status.State)
			require.Contains(t, failed.Status.Message, "temporary annotation failure")
			receipts := retryReceipts(t, kyvernoClient, ur)
			require.NotEmpty(t, receipts)
			for _, receipt := range receipts {
				obj := client.objects[retryObjectKey(receipt.Target.Namespace, receipt.Target.Name)]
				require.NotNil(t, obj)
				require.Empty(t, obj.GetAnnotations()[provenance.Annotation])
				obj.Object["data"] = map[string]any{"key": "ordinary-user-edit"}
				obj.SetResourceVersion("42")
			}
			client.patchErr = nil
			if strings.HasSuffix(testName, " missing source") || strings.HasSuffix(testName, " empty list") {
				missingSources := make(map[string]*unstructured.Unstructured)
				for key, obj := range client.objects {
					if obj.GetNamespace() == "sources" {
						missingSources[key] = obj.DeepCopy()
						delete(client.objects, key)
					}
				}
				if mode == "foreach" {
					gen.policy.GetSpec().Rules[0].Generation.ForEachGeneration[0].List = "`[]`"
				}
				failed.Status.State = kyvernov2.Pending
				pending, err := requests.UpdateStatus(t.Context(), failed, metav1.UpdateOptions{})
				require.NoError(t, err)
				patchesBefore := len(client.patches)
				process(pending)
				failed, err = requests.Get(t.Context(), ur.Name, metav1.GetOptions{})
				require.NoError(t, err)
				require.Equal(t, kyvernov2.Failed, failed.Status.State, "a missing source/empty list must not discard pending receipts")
				if mode == "clone" {
					require.Contains(t, failed.Status.Message, "not found")
				} else {
					require.Contains(t, failed.Status.Message, "pending receipt")
				}
				require.Equal(t, receipts, retryReceipts(t, kyvernoClient, ur))
				require.Len(t, client.patches, patchesBefore)
				for _, receipt := range receipts {
					obj := client.objects[retryObjectKey(receipt.Target.Namespace, receipt.Target.Name)]
					require.Equal(t, receipt.Target.UID, obj.GetUID())
					require.Empty(t, obj.GetAnnotations()[provenance.Annotation])
					require.Equal(t, "ordinary-user-edit", obj.Object["data"].(map[string]any)["key"])
				}
				for key, source := range missingSources {
					client.objects[key] = source
				}
				if mode == "foreach" {
					gen.policy.GetSpec().Rules[0].Generation.ForEachGeneration[0].List = "`[\"first\",\"second\"]`"
				}
			}
			failed.Status.State = kyvernov2.Pending
			pending, err := requests.UpdateStatus(t.Context(), failed, metav1.UpdateOptions{})
			require.NoError(t, err)
			process(pending) // A new controller and generator must recover durable receipts.
			completed, err := requests.Get(t.Context(), ur.Name, metav1.GetOptions{})
			require.NoError(t, err)
			require.Equal(t, kyvernov2.Completed, completed.Status.State, completed.Status.Message)
			require.Empty(t, retryReceipts(t, kyvernoClient, ur))
			for _, name := range names {
				obj := client.objects[retryObjectKey("tenant", name)]
				require.NotNil(t, obj)
				valid, err := provenance.NewStore(client.GetKubeClient()).Verify(t.Context(), gen.policy, obj)
				require.NoError(t, err)
				require.True(t, valid, name)
			}
			for _, receipt := range receipts {
				obj := client.objects[retryObjectKey(receipt.Target.Namespace, receipt.Target.Name)]
				require.Equal(t, "ordinary-user-edit", obj.Object["data"].(map[string]any)["key"])
			}
			require.Equal(t, len(names), client.createCount, "retry must not recreate existing targets")
		})
	}
}

func TestGenerateProvenanceRetryRejectsChangedIdentity(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"target UID", "routing labels", "policy UID", "forged receipt", "request UID"} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()
			gen, client := newProvenanceStampGenerator(t)
			gen.rule.Generation.Synchronize = false
			ur, requests := newGenerationRetryRequest(t, gen)
			client.patchErr = apierrors.NewTimeoutError("temporary annotation failure", 1)
			_, err := gen.generate()
			require.Error(t, err)
			require.Len(t, retryReceipts(t, requests, ur), 1)
			client.patchErr = nil
			client.target.Object["data"] = map[string]any{"key": "keep-existing-data"}
			switch change {
			case "target UID":
				client.target.SetUID("replacement-uid")
			case "routing labels":
				labels := client.target.GetLabels()
				labels[common.GenerateSourceUIDLabel] = "different-source"
				client.target.SetLabels(labels)
			case "policy UID":
				gen.policy.SetUID("replacement-policy")
			case "forged receipt", "request UID":
				latest, err := gen.pending.requests.Get(t.Context(), ur.Name, metav1.GetOptions{})
				require.NoError(t, err)
				if change == "request UID" {
					latest.UID = "replacement-request"
				} else {
					receipts := retryReceipts(t, requests, ur)
					receipts[0].Stamp = "v1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
					encoded, err := json.Marshal(receipts)
					require.NoError(t, err)
					latest.Annotations[pendingProvenanceAnnotation] = string(encoded)
				}
				_, err = gen.pending.requests.Update(t.Context(), latest, metav1.UpdateOptions{})
				require.NoError(t, err)
			}
			gen.pending.loaded = false // A fresh reconciliation reads current request metadata.
			_, err = gen.generate()
			require.ErrorIs(t, err, errGeneratedProvenance)
			require.Len(t, client.patches, 1, "retry must not stamp an unproven target")
			require.Empty(t, client.target.GetAnnotations()[provenance.Annotation])
			require.Equal(t, "keep-existing-data", client.target.Object["data"].(map[string]any)["key"])
			require.Zero(t, client.updateCount)
		})
	}
}

func TestGenerateProvenanceRetryRetiresAlreadyPersistedStamp(t *testing.T) {
	t.Parallel()
	gen, client := newProvenanceStampGenerator(t)
	gen.rule.Generation.Synchronize = false
	ur, requests := newGenerationRetryRequest(t, gen)
	client.patchErr = apierrors.NewTimeoutError("temporary annotation failure", 1)
	_, err := gen.generate()
	require.Error(t, err)
	require.Len(t, retryReceipts(t, requests, ur), 1)
	client.patchErr = nil
	failRetirement := true
	requests.PrependReactor("update", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
		obj := action.(k8stesting.UpdateAction).GetObject().(*kyvernov2.UpdateRequest)
		if failRetirement && obj.Annotations[pendingProvenanceAnnotation] == "" {
			return true, nil, apierrors.NewTimeoutError("temporary receipt retirement failure", 1)
		}
		return false, nil, nil
	})
	_, err = gen.generate()
	require.ErrorContains(t, err, "retire generated resource receipt")
	assertGeneratedProvenance(t, gen, client)
	require.Len(t, retryReceipts(t, requests, ur), 1)
	failRetirement = false
	_, err = gen.generate()
	require.NoError(t, err)
	require.Empty(t, retryReceipts(t, requests, ur))
	require.Len(t, client.patches, 2, "an already applied stamp must not be patched again")
	require.Equal(t, 1, client.createCount)
}

func TestGenerationReceiptConflictPreservesOtherProgress(t *testing.T) {
	t.Parallel()
	gen, client := newProvenanceStampGenerator(t)
	ur, requests := newGenerationRetryRequest(t, gen)
	client.patchErr = apierrors.NewTimeoutError("temporary annotation failure", 1)
	other := generationReceipt{Target: kyvernov1.ResourceSpec{Name: "other"}, Stamp: "other-receipt"}
	conflict := true
	requests.PrependReactor("update", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if !conflict {
			return false, nil, nil
		}
		conflict = false
		concurrent := ur.DeepCopy()
		encoded, err := json.Marshal([]generationReceipt{other})
		require.NoError(t, err)
		concurrent.Annotations[pendingProvenanceAnnotation] = string(encoded)
		concurrent.Annotations["example.com/keep"] = "concurrent"
		require.NoError(t, requests.Tracker().Update(kyvernov2.SchemeGroupVersion.WithResource("updaterequests"), concurrent, ur.Namespace))
		return true, nil, apierrors.NewConflict(schema.GroupResource{Resource: "updaterequests"}, ur.Name, fmt.Errorf("concurrent receipt"))
	})
	_, err := gen.generate()
	require.Error(t, err)
	require.Len(t, retryReceipts(t, requests, ur), 2)
	client.patchErr = nil
	_, err = gen.generate()
	require.NoError(t, err)
	require.Equal(t, []generationReceipt{other}, retryReceipts(t, requests, ur))
	latest, err := gen.pending.requests.Get(t.Context(), ur.Name, metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, "concurrent", latest.Annotations["example.com/keep"])
}

func TestGenerateProvenanceNormalBatchDoesNotWriteRequestMetadata(t *testing.T) {
	t.Parallel()
	gen, client := newProvenanceStampGenerator(t)
	_, requests := newGenerationRetryRequest(t, gen)
	for _, name := range []string{"first", "second", "third"} {
		gen.pattern.Name = name
		_, err := gen.generate()
		require.NoError(t, err)
	}
	reads, writes := 0, 0
	for _, action := range requests.Actions() {
		if action.GetVerb() == "get" {
			reads++
		} else if action.GetVerb() == "update" || action.GetVerb() == "patch" {
			writes++
		}
	}
	require.Equal(t, 1, reads, "the retry ledger must be read once per bounded batch")
	require.Zero(t, writes, "successful generation must not write a retry ledger")
	require.Len(t, client.patches, 3)
}

func TestGenerateProvenanceRetryAfterAmbiguousPatchTimeout(t *testing.T) {
	t.Parallel()
	gen, client := newProvenanceStampGenerator(t)
	gen.rule.Generation.Synchronize = false
	ur, requests := newGenerationRetryRequest(t, gen)
	// The client applies the actual JSON patch, then loses its response.
	client.afterPatchErr = apierrors.NewTimeoutError("response lost after patch", 1)
	_, err := gen.generate()
	require.ErrorContains(t, err, "response lost after patch")
	assertGeneratedProvenance(t, gen, client)
	require.Len(t, retryReceipts(t, requests, ur), 1)
	client.afterPatchErr = nil
	_, err = gen.generate()
	require.NoError(t, err)
	require.Empty(t, retryReceipts(t, requests, ur))
	require.Len(t, client.patches, 1, "the ambiguous completed patch must not be replayed")
}

func TestGenerateProvenanceReceiptFailureDoesNotAdoptUnsignedTarget(t *testing.T) {
	t.Parallel()
	gen, client := newProvenanceStampGenerator(t)
	gen.rule.Generation.Synchronize = false
	ur, requests := newGenerationRetryRequest(t, gen)
	client.patchErr = apierrors.NewTimeoutError("temporary annotation failure", 1)
	requests.PrependReactor("update", "updaterequests", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewTimeoutError("receipt could not be saved", 1)
	})
	_, err := gen.generate()
	require.ErrorContains(t, err, "receipt could not be saved")
	require.Empty(t, retryReceipts(t, requests, ur))
	client.patchErr = nil
	client.target.Object["data"] = map[string]any{"key": "ordinary-user-edit"}
	// Even a status record claiming the right UID cannot replace an authentic
	// receipt. This also represents the unavoidable create/receipt crash gap.
	latest, err := gen.pending.requests.Get(t.Context(), ur.Name, metav1.GetOptions{})
	require.NoError(t, err)
	latest.Status.GeneratedResources = []kyvernov1.ResourceSpec{common.ResourceSpecFromUnstructured(*client.target)}
	require.NoError(t, requests.Tracker().Update(kyvernov2.SchemeGroupVersion.WithResource("updaterequests"), latest, ur.Namespace))
	gen.pending.loaded = false
	_, err = gen.generate()
	require.NoError(t, err)
	require.Empty(t, client.target.GetAnnotations()[provenance.Annotation])
	require.Len(t, client.patches, 1, "status and labels must never authorize a new stamp")
	require.Equal(t, "ordinary-user-edit", client.target.Object["data"].(map[string]any)["key"])
}

func TestGenerateProvenanceRetryUsesResolvedTargetIdentity(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, apiVersion, kind, namespace, resolvedVersion, resolvedNamespace string
	}{
		{"omitted noncore version", "", "Widget", "tenant", "example.io/v1", "tenant"},
		{"default namespace", "v1", "ConfigMap", "", "v1", "default"},
		{"cluster scope", "", "Namespace", "", "v1", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			gen, client := newProvenanceStampGenerator(t)
			gen.rule.Generation.Synchronize = false
			gen.pattern.APIVersion, gen.pattern.Kind, gen.pattern.Namespace = test.apiVersion, test.kind, test.namespace
			ur, requests := newGenerationRetryRequest(t, gen)
			client.beforeReturn = func(obj *unstructured.Unstructured) {
				obj.SetAPIVersion(test.resolvedVersion)
				obj.SetNamespace(test.resolvedNamespace)
			}
			client.patchErr = apierrors.NewTimeoutError("temporary annotation failure", 1)
			_, err := gen.generate()
			require.Error(t, err)
			require.Len(t, retryReceipts(t, requests, ur), 1)
			client.patchErr = nil
			_, err = gen.generate()
			require.NoError(t, err)
			assertGeneratedProvenance(t, gen, client)
			require.Empty(t, retryReceipts(t, requests, ur))
			require.Equal(t, 1, client.createCount)
		})
	}
}
