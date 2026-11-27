//go:build integration

package mpol_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	"github.com/kyverno/kyverno/test/integration/framework"
	"github.com/stretchr/testify/require"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Observes the actual manager client used by the production exception watch,
// not a direct invocation of newPolicyExceptionHandler.
type indexedWatchClient struct {
	client.Client
	indexedLists atomic.Int64
}

func (c *indexedWatchClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if _, ok := list.(*policiesv1beta1.NamespacedMutatingPolicyList); ok {
		options := new(client.ListOptions).ApplyOptions(opts)
		if options.FieldSelector != nil {
			name, exact := options.FieldSelector.RequiresExactMatch(celengine.NamespacedPolicyNameIndex)
			if exact && name == "watch-target" {
				c.indexedLists.Add(1)
			}
		}
	}
	return c.Client.List(ctx, list, opts...)
}

var watchObserver *indexedWatchClient

type indexedWatchManager struct {
	ctrl.Manager
	observed client.Client
}

func (m *indexedWatchManager) GetClient() client.Client {
	return m.observed
}

// This envtest exercises production NewKubeProvider registration, the manager
// watch, and index setup. One exception event must refresh same-named policies
// in two namespaces with only one indexed namespaced-policy lookup.
func TestPolicyExceptionWatch_IndexesAndReconcilesBothNamespaces(t *testing.T) {
	ctx := context.Background()
	// TestMain registers the production MPOL provider and exception watch.
	// Reuse them to avoid re-registering controllers in this test process.
	for _, ns := range []string{"watch-team-a", "watch-team-b"} {
		_, err := testEnv.KubeClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
		}, metav1.CreateOptions{})
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = testEnv.KubeClient.CoreV1().Namespaces().Delete(context.Background(), ns, metav1.DeleteOptions{})
		})

		policy := &policiesv1beta1.NamespacedMutatingPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: "watch-target", Namespace: ns},
			Spec: policiesv1beta1.MutatingPolicySpec{
				MatchConstraints: framework.PodMatchRules(),
				Mutations: []admissionregistrationv1alpha1.Mutation{{
					PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
					JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
						Expression: `[JSONPatch{op: "add", path: "/metadata/labels/watch", value: "true"}]`,
					},
				}},
			},
		}
		require.NoError(t, testEnv.Client.Create(ctx, policy))
		t.Cleanup(func() { _ = testEnv.Client.Delete(context.Background(), policy) })
	}

	snapshot := func() map[string]*mpolcompiler.Policy {
		compiled := make(map[string]*mpolcompiler.Policy)
		for _, policy := range provider.Fetch(ctx, false) {
			if policy.Policy.GetName() != "watch-target" {
				continue
			}
			ns := policy.Policy.GetNamespace()
			if compiled[ns] == nil {
				compiled[ns] = policy.CompiledPolicy
			}
		}
		return compiled
	}
	require.Eventually(t, func() bool {
		policies := snapshot()
		return policies["watch-team-a"] != nil && policies["watch-team-b"] != nil
	}, 10*time.Second, 100*time.Millisecond, "both namespaced policies must be compiled")

	original := snapshot()
	require.Equal(t, int64(0), watchObserver.indexedLists.Load(),
		"ordinary policy CREATE must not trigger a PolicyException name lookup")

	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "watch-exception", Namespace: "watch-team-a"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{
				Name: "watch-target", Kind: "NamespacedMutatingPolicy",
			}},
		},
	}
	require.NoError(t, testEnv.Client.Create(ctx, exception))
	t.Cleanup(func() { _ = testEnv.Client.Delete(context.Background(), exception) })

	require.Eventually(t, func() bool {
		current := snapshot()
		return watchObserver.indexedLists.Load() == 1 &&
			current["watch-team-a"] != nil &&
			current["watch-team-b"] != nil &&
			current["watch-team-a"] != original["watch-team-a"] &&
			current["watch-team-b"] != original["watch-team-b"]
	}, 10*time.Second, 100*time.Millisecond,
		"one manager exception event must recompile both namespaces using one indexed lookup")

	require.Equal(t, int64(1), watchObserver.indexedLists.Load(),
		"single exception-watch ownership must avoid duplicate indexed lookups")
}
