package generate

import (
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/generate/provenance"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
