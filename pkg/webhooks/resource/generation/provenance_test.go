package generation

import (
	"context"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/background/generate/provenance"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

// Use the actual policy's intended target name and the actual trigger UID in
// every candidate. Neither matching those values nor retaining all labels in a
// no-op update is proof that the background controller generated the object.
func TestNonTriggerRequiresControllerProvenance(t *testing.T) {
	for _, operation := range []kyvernov1.AdmissionOperation{kyvernov1.Update, kyvernov1.Delete} {
		for _, tc := range []struct {
			name       string
			sign       bool
			mutate     func(*unstructured.Unstructured, *kyvernov1.ClusterPolicy)
			wantQueued bool
		}{
			{name: "pre-policy planted target"},
			{name: "forged MAC", mutate: func(obj *unstructured.Unstructured, _ *kyvernov1.ClusterPolicy) {
				obj.SetAnnotations(map[string]string{provenance.Annotation: "v1:" + strings.Repeat("A", 43)})
			}},
			{name: "copied MAC on replacement UID", sign: true, mutate: func(obj *unstructured.Unstructured, _ *kyvernov1.ClusterPolicy) { obj.SetUID("replacement-uid") }},
			{name: "copied MAC in another namespace", sign: true, mutate: func(obj *unstructured.Unstructured, _ *kyvernov1.ClusterPolicy) { obj.SetNamespace("another-tenant") }},
			{name: "changed trigger UID", sign: true, mutate: func(obj *unstructured.Unstructured, _ *kyvernov1.ClusterPolicy) {
				labels := obj.GetLabels()
				labels[common.GenerateTriggerUIDLabel] = "another-trigger"
				obj.SetLabels(labels)
			}},
			{name: "recreated policy", sign: true, mutate: func(_ *unstructured.Unstructured, policy *kyvernov1.ClusterPolicy) { policy.SetUID("new-policy-uid") }},
			{name: "removed rule", sign: true, mutate: func(_ *unstructured.Unstructured, policy *kyvernov1.ClusterPolicy) { policy.Spec.Rules = nil }},
			{name: "rule no longer generates", sign: true, mutate: func(_ *unstructured.Unstructured, policy *kyvernov1.ClusterPolicy) {
				policy.Spec.Rules[0].Generation = nil
				policy.Spec.Rules[0].Mutation = &kyvernov1.Mutation{}
			}},
			{name: "synchronization disabled", sign: true, mutate: func(_ *unstructured.Unstructured, policy *kyvernov1.ClusterPolicy) {
				policy.Spec.Rules[0].Generation.Synchronize = false
			}},
			{name: "genuine downstream", sign: true, wantQueued: true},
			{name: "ordinary data and clone marker edits", sign: true, wantQueued: true, mutate: func(obj *unstructured.Unstructured, _ *kyvernov1.ClusterPolicy) {
				obj.Object["data"] = map[string]interface{}{"value": "edited"}
				labels := obj.GetLabels()
				labels[common.GenerateTypeCloneSourceLabel] = ""
				obj.SetLabels(labels)
			}},
		} {
			t.Run(string(operation)+"/"+tc.name, func(t *testing.T) {
				policy, target, _ := provenanceFixtures()
				kube := kubefake.NewSimpleClientset()
				require.NoError(t, provenance.EnsureKey(t.Context(), kube.CoreV1().Secrets(config.KyvernoNamespace())))
				store := provenance.NewStore(kube)
				if tc.sign {
					signTarget(t, store, policy, target)
				}
				if tc.mutate != nil {
					tc.mutate(target, policy)
				}
				h, recorder := provenanceHandler(t, store, policy)
				// Exercise non-trigger dispatch: no matching trigger policy is needed
				// for the vulnerable oldObject path to run.
				pctx := nonTriggerContext(t, target, operation)
				h.handleNonTrigger(t.Context(), pctx)
				if !tc.wantQueued {
					require.Empty(t, recorder.specs, "untrusted metadata must not enqueue generation")
					return
				}
				require.Len(t, recorder.specs, 1)
				require.Len(t, recorder.specs[0].RuleContext, 1)
				require.Equal(t, "sync", recorder.specs[0].RuleContext[0].Rule)
				require.Equal(t, types.UID("existing-trigger-uid"), recorder.specs[0].RuleContext[0].Trigger.UID)
				require.False(t, recorder.specs[0].RuleContext[0].DeleteDownstream)
			})
		}
	}
}

func TestCloneSourceVerifiesEveryActualDownstream(t *testing.T) {
	for _, operation := range []kyvernov1.AdmissionOperation{kyvernov1.Update, kyvernov1.Delete} {
		t.Run(string(operation), func(t *testing.T) {
			policy, target, source := provenanceFixtures()
			kube := kubefake.NewSimpleClientset()
			require.NoError(t, provenance.EnsureKey(t.Context(), kube.CoreV1().Secrets(config.KyvernoNamespace())))
			store := provenance.NewStore(kube)
			signTarget(t, store, policy, target)
			unsigned := target.DeepCopy()
			unsigned.SetName("unsigned")
			unsigned.SetUID("unsigned-uid")
			unsigned.SetAnnotations(nil)
			copied := target.DeepCopy()
			copied.SetName("copied")
			copied.SetUID("copied-uid")
			staleSource := target.DeepCopy()
			staleSource.SetName("stale-source")
			staleSource.SetUID("stale-source-target")
			labels := staleSource.GetLabels()
			labels[common.GenerateSourceUIDLabel] = "previous-source-uid"
			staleSource.SetLabels(labels)
			signTarget(t, store, policy, staleSource)
			// Each current-source candidate is returned by BOTH the source-name and UID selectors.
			// Only the genuine object's UID may contribute a single UpdateRequest.
			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))
			dyn := dynamicfake.NewSimpleDynamicClient(scheme, target, unsigned, copied, staleSource)
			h, recorder := provenanceHandler(t, store, policy)
			h.client = dclient.NewFakeClientWithDisco(dyn, kube, dclient.NewFakeDiscoveryClient(nil))
			require.NoError(t, h.processRequest(t.Context(), nonTriggerContext(t, source, operation)))
			require.Len(t, recorder.specs, 1)
			require.Len(t, recorder.specs[0].RuleContext, 1)
			require.Equal(t, operation == kyvernov1.Delete, recorder.specs[0].RuleContext[0].DeleteDownstream)
			require.Equal(t, types.UID("existing-trigger-uid"), recorder.specs[0].RuleContext[0].Trigger.UID)
		})
	}
}

func TestNonTriggerProvenanceFailsClosed(t *testing.T) {
	for _, missingPolicy := range []bool{false, true} {
		t.Run(map[bool]string{false: "key unavailable", true: "policy deleted"}[missingPolicy], func(t *testing.T) {
			policy, target, _ := provenanceFixtures()
			kube := kubefake.NewSimpleClientset()
			require.NoError(t, provenance.EnsureKey(t.Context(), kube.CoreV1().Secrets(config.KyvernoNamespace())))
			store := provenance.NewStore(kube)
			signTarget(t, store, policy, target)
			require.NoError(t, kube.CoreV1().Secrets(config.KyvernoNamespace()).Delete(t.Context(), provenance.SecretName, metav1.DeleteOptions{}))
			if missingPolicy {
				policy = nil
			}
			h, recorder := provenanceHandler(t, store, policy)
			err := h.processRequest(t.Context(), nonTriggerContext(t, target, kyvernov1.Delete))
			if missingPolicy {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "failed to verify generated resource provenance")
			}
			require.Empty(t, recorder.specs)
		})
	}
}

func provenanceFixtures() (*kyvernov1.ClusterPolicy, *unstructured.Unstructured, *unstructured.Unstructured) {
	policy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sync-policy", UID: "policy-uid"},
		Spec: kyvernov1.Spec{Rules: []kyvernov1.Rule{{
			Name:           "sync",
			MatchResources: kyvernov1.MatchResources{ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"Namespace"}}},
			Generation: &kyvernov1.Generation{Synchronize: true, GeneratePattern: kyvernov1.GeneratePattern{
				ResourceSpec: kyvernov1.ResourceSpec{APIVersion: "v1", Kind: "ConfigMap", Namespace: "tenant", Name: "intended-target"},
			}},
		}}},
	}
	source := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "v1", "kind": "ConfigMap"}}
	source.SetName("source")
	source.SetNamespace("source-namespace")
	source.SetUID("source-uid")
	// A standalone managed-by value must not prevent clone-source traversal.
	source.SetLabels(map[string]string{common.GenerateTypeCloneSourceLabel: "", kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp})
	target := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "v1", "kind": "ConfigMap"}}
	target.SetName("intended-target")
	target.SetNamespace("tenant")
	target.SetUID("target-uid")
	trigger := unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": "v1", "kind": "Namespace"}}
	trigger.SetName("tenant")
	trigger.SetUID("existing-trigger-uid")
	common.ManageLabels(target, trigger, policy, "sync")
	labels := target.GetLabels()
	labels[common.GenerateSourceGroupLabel] = ""
	labels[common.GenerateSourceVersionLabel] = "v1"
	labels[common.GenerateSourceKindLabel] = "ConfigMap"
	labels[common.GenerateSourceNSLabel] = source.GetNamespace()
	labels[common.GenerateSourceNameLabel] = source.GetName()
	labels[common.GenerateSourceUIDLabel] = string(source.GetUID())
	target.SetLabels(labels)
	return policy, target, source
}

func signTarget(t *testing.T, store *provenance.Store, policy kyvernov1.PolicyInterface, target *unstructured.Unstructured) {
	t.Helper()
	stamp, err := store.Sign(t.Context(), policy, target)
	require.NoError(t, err)
	target.SetAnnotations(map[string]string{provenance.Annotation: stamp})
}

func nonTriggerContext(t *testing.T, old *unstructured.Unstructured, operation kyvernov1.AdmissionOperation) *engine.PolicyContext {
	t.Helper()
	configuration := config.NewDefaultConfiguration(false)
	pctx, err := engine.NewPolicyContext(jmespath.New(configuration), *old, operation, nil, configuration)
	require.NoError(t, err)
	pctx = pctx.WithOldResource(*old).WithResourceKind(schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, "")
	if operation == kyvernov1.Update {
		pctx = pctx.WithNewResource(*old.DeepCopy())
	}
	return pctx
}

func provenanceHandler(t *testing.T, store *provenance.Store, policy *kyvernov1.ClusterPolicy) (*generationHandler, *provenanceURRecorder) {
	t.Helper()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	if policy != nil {
		require.NoError(t, indexer.Add(policy))
	}
	recorder := &provenanceURRecorder{}
	return &generationHandler{
		log: logr.Discard(), provenance: store,
		cpolLister: kyvernov1listers.NewClusterPolicyLister(indexer),
		polLister:  kyvernov1listers.NewPolicyLister(indexer), urGenerator: recorder,
	}, recorder
}

type provenanceURRecorder struct {
	specs []kyvernov2.UpdateRequestSpec
}

func (r *provenanceURRecorder) Apply(_ context.Context, spec kyvernov2.UpdateRequestSpec) error {
	r.specs = append(r.specs, spec)
	return nil
}
