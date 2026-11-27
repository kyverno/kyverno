package admissionpolicygenerator

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// A PolicyException update must reconcile every generated native admission policy
// affected by the change. In particular, removing a ref must invalidate the old
// policy, not just generate the newly referenced policy.
func TestUpdateCELExceptionRequeuesOldAndNewPolicyRefs(t *testing.T) {
	vpolA := policiesv1beta1.PolicyRef{Kind: "ValidatingPolicy", Name: "vpol-a"}
	vpolB := policiesv1beta1.PolicyRef{Kind: "ValidatingPolicy", Name: "vpol-b"}
	mpolA := policiesv1beta1.PolicyRef{Kind: "MutatingPolicy", Name: "mpol-a"}
	mpolB := policiesv1beta1.PolicyRef{Kind: "MutatingPolicy", Name: "mpol-b"}

	tests := []struct {
		name         string
		oldRefs      []policiesv1beta1.PolicyRef
		newRefs      []policiesv1beta1.PolicyRef
		oldCondition string
		newCondition string
		want         []string
	}{
		{
			name:    "validating policy retarget",
			oldRefs: []policiesv1beta1.PolicyRef{vpolA},
			newRefs: []policiesv1beta1.PolicyRef{vpolB},
			want:    []string{"ValidatingPolicy/vpol-a", "ValidatingPolicy/vpol-b"},
		},
		{
			name:    "mutating policy retarget",
			oldRefs: []policiesv1beta1.PolicyRef{mpolA},
			newRefs: []policiesv1beta1.PolicyRef{mpolB},
			want:    []string{"MutatingPolicy/mpol-a", "MutatingPolicy/mpol-b"},
		},
		{
			name:    "removed reference with unchanged reference",
			oldRefs: []policiesv1beta1.PolicyRef{vpolA, vpolB},
			newRefs: []policiesv1beta1.PolicyRef{vpolB},
			want:    []string{"ValidatingPolicy/vpol-a", "ValidatingPolicy/vpol-b"},
		},
		{
			name:    "cross-kind retarget",
			oldRefs: []policiesv1beta1.PolicyRef{vpolA},
			newRefs: []policiesv1beta1.PolicyRef{mpolA},
			want:    []string{"ValidatingPolicy/vpol-a", "MutatingPolicy/mpol-a"},
		},
		{
			name:         "match condition changed without retargeting",
			oldRefs:      []policiesv1beta1.PolicyRef{vpolA},
			newRefs:      []policiesv1beta1.PolicyRef{vpolA},
			oldCondition: "true",
			newCondition: "false",
			want:         []string{"ValidatingPolicy/vpol-a"},
		},
		{
			name:    "unchanged spec skips reconciliation",
			oldRefs: []policiesv1beta1.PolicyRef{vpolA},
			newRefs: []policiesv1beta1.PolicyRef{vpolA},
		},
		{
			name:    "missing former policy does not block new policy",
			oldRefs: []policiesv1beta1.PolicyRef{{Kind: "ValidatingPolicy", Name: "removed"}},
			newRefs: []policiesv1beta1.PolicyRef{mpolB},
			want:    []string{"MutatingPolicy/mpol-b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vpolIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			mpolIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
			for _, name := range []string{"vpol-a", "vpol-b"} {
				require.NoError(t, vpolIndexer.Add(&policiesv1beta1.ValidatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: name},
				}))
			}
			for _, name := range []string{"mpol-a", "mpol-b"} {
				require.NoError(t, mpolIndexer.Add(&policiesv1beta1.MutatingPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: name},
				}))
			}

			queue := workqueue.NewTypedRateLimitingQueueWithConfig(
				workqueue.DefaultTypedControllerRateLimiter[any](),
				workqueue.TypedRateLimitingQueueConfig[any]{Name: "policy-exception-retarget"},
			)
			defer queue.ShutDown()

			c := &controller{
				vpolLister: policiesv1beta1listers.NewValidatingPolicyLister(vpolIndexer),
				mpolLister: policiesv1beta1listers.NewMutatingPolicyLister(mpolIndexer),
				queue:      queue,
			}

			old := &policiesv1beta1.PolicyException{
				ObjectMeta: metav1.ObjectMeta{Name: "retarget-example", Namespace: "default"},
				Spec: policiesv1beta1.PolicyExceptionSpec{
					PolicyRefs: tt.oldRefs,
				},
			}
			if tt.oldCondition != "" {
				old.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{
					Name: "gate", Expression: tt.oldCondition,
				}}
			}
			updated := old.DeepCopy()
			updated.Spec.PolicyRefs = tt.newRefs
			if tt.newCondition != "" {
				updated.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{{
					Name: "gate", Expression: tt.newCondition,
				}}
			}

			c.updateCELException(old, updated)

			require.Equal(t, len(tt.want), queue.Len(), "each affected native policy should be queued once")
			got := make([]string, 0, queue.Len())
			for queue.Len() > 0 {
				item, shutdown := queue.Get()
				require.False(t, shutdown)
				key, ok := item.(string)
				require.True(t, ok)
				got = append(got, key)
				queue.Done(item)
			}
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}
