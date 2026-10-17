package generate

import (
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/background/generate/provenance"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

func TestProcessURDoesNotExecuteRecreatedPolicy(t *testing.T) {
	t.Parallel()
	for _, namespaced := range []bool{false, true} {
		name := "ClusterPolicy"
		if namespaced {
			name = "Policy"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			policyName := "generate-sensitive-resource"
			metadata := metav1.ObjectMeta{Name: policyName, UID: "replacement-policy-uid"}
			indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
			policyKey := policyName
			if namespaced {
				metadata.Namespace = "tenant"
				policyKey = metadata.Namespace + "/" + policyName
				require.NoError(t, indexer.Add(&kyvernov1.Policy{ObjectMeta: metadata}))
			} else {
				require.NoError(t, indexer.Add(&kyvernov1.ClusterPolicy{ObjectMeta: metadata}))
			}
			status := &fakeStatusControl{}
			controller := &GenerateController{
				policyLister:  kyvernov1listers.NewClusterPolicyLister(indexer),
				npolicyLister: kyvernov1listers.NewPolicyLister(indexer),
				statusControl: status,
				log:           logr.Discard(),
				// No resource client or engine is provided: any trigger lookup or
				// policy execution would panic instead of silently passing this test.
			}
			ur := &kyvernov2.UpdateRequest{
				ObjectMeta: metav1.ObjectMeta{Name: "stale-request", Annotations: map[string]string{provenance.PolicyUIDAnnotation: "original-policy-uid"}},
				Spec: kyvernov2.UpdateRequestSpec{
					Type: kyvernov2.Generate, Policy: policyKey,
					RuleContext: []kyvernov2.RuleContext{{Rule: "generate", Trigger: kyvernov1.ResourceSpec{APIVersion: "v1", Kind: "Namespace", Name: "trigger", UID: "trigger-uid"}}},
				},
			}
			require.NoError(t, controller.ProcessUR(ur))
			assert.True(t, status.successCalled, "stale work must complete without executing the replacement policy")
			assert.False(t, status.failedCalled)
		})
	}
}

func TestProcessURCleansUpOriginalPolicyAfterRecreation(t *testing.T) {
	t.Parallel()
	for _, namespaced := range []bool{false, true} {
		for _, mode := range []string{"authenticated", "missing cleanup identity", "mismatched cleanup identity", "missing target records", "delete conflict"} {
			t.Run(fmt.Sprintf("namespaced=%t/%s", namespaced, mode), func(t *testing.T) {
				t.Parallel()
				gen, base := newProvenanceStampGenerator(t)
				original := gen.policy
				if namespaced {
					original = &kyvernov1.Policy{ObjectMeta: metav1.ObjectMeta{
						Name: original.GetName(), Namespace: "tenant", UID: original.GetUID(),
					}}
				}
				replacement := original.CreateDeepCopy()
				replacement.SetUID("replacement-policy-uid")
				indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
				require.NoError(t, indexer.Add(replacement))
				policyKey := original.GetName()
				if namespaced {
					policyKey = original.GetNamespace() + "/" + policyKey
				}

				newTarget := func(name string, policy kyvernov1.PolicyInterface) unstructured.Unstructured {
					target := gen.trigger.DeepCopy()
					target.SetName(name)
					target.SetUID(types.UID(name + "-uid"))
					target.SetResourceVersion("1")
					common.ManageLabels(target, gen.trigger, policy, gen.rule.Name)
					stamp, err := gen.provenance.Sign(t.Context(), policy, target)
					require.NoError(t, err)
					target.SetAnnotations(map[string]string{provenance.Annotation: stamp})
					return *target
				}
				targets := []unstructured.Unstructured{
					newTarget("original", original), newTarget("unsigned", original),
					newTarget("recreated-target", original), newTarget("replacement-policy-target", replacement),
				}
				ur := &kyvernov2.UpdateRequest{
					ObjectMeta: metav1.ObjectMeta{Name: "old-policy-cleanup", Annotations: map[string]string{
						provenance.PolicyUIDAnnotation:        string(original.GetUID()),
						provenance.CleanupPolicyUIDAnnotation: string(original.GetUID()),
					}},
					Spec: kyvernov2.UpdateRequestSpec{
						Type: kyvernov2.Generate, Policy: policyKey,
						RuleContext: []kyvernov2.RuleContext{
							{Rule: gen.rule.Name, Trigger: common.ResourceSpecFromUnstructured(gen.trigger), DeleteDownstream: true},
							// Mixed contexts must not execute replacement generation either.
							{Rule: gen.rule.Name, Trigger: common.ResourceSpecFromUnstructured(gen.trigger)},
						},
					},
				}
				for _, target := range targets {
					ur.Status.GeneratedResources = append(ur.Status.GeneratedResources, common.ResourceSpecFromUnstructured(target))
				}
				targets[1].SetAnnotations(nil)
				targets[2].SetUID("replacement-target-uid")
				client := &provenanceCleanupClient{Interface: base, items: targets}
				switch mode {
				case "missing cleanup identity":
					delete(ur.Annotations, provenance.CleanupPolicyUIDAnnotation)
				case "mismatched cleanup identity":
					ur.Annotations[provenance.CleanupPolicyUIDAnnotation] = string(replacement.GetUID())
				case "missing target records":
					ur.Status.GeneratedResources = nil
				case "delete conflict":
					client.beforeDelete = func(target *unstructured.Unstructured) { target.SetUID("concurrent-replacement") }
				}
				status := &fakeStatusControl{}
				controller := &GenerateController{
					client: client, provenance: gen.provenance, statusControl: status,
					policyLister:  kyvernov1listers.NewClusterPolicyLister(indexer),
					npolicyLister: kyvernov1listers.NewPolicyLister(indexer),
					log:           logr.Discard(),
					// No engine or live trigger: cleanup must use only the old policy's
					// authenticated status records, never replacement policy evaluation.
				}
				require.NoError(t, controller.ProcessUR(ur))
				require.Equal(t, mode == "delete conflict", status.failedCalled)
				require.Equal(t, mode != "delete conflict", status.successCalled)
				if mode == "authenticated" {
					require.Equal(t, []string{"original"}, client.deleted)
				} else {
					require.Empty(t, client.deleted)
				}
			})
		}
	}
}
