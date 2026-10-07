package engine

import (
	"context"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestPolicyExceptionHandler_RequeuesNamespacedPoliciesWithNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, policiesv1beta1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&policiesv1beta1.NamespacedImageValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "require-team", Namespace: "team-a"}},
		&policiesv1beta1.NamespacedImageValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "require-team", Namespace: "team-b"}},
		&policiesv1beta1.NamespacedImageValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "team-a"}},
	).Build()
	polex := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "exempt", Namespace: "team-a"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{
				{Name: "require-team", Kind: policieskyvernoio.NamespacedImageValidatingPolicyKind},
				{Name: "cluster-policy", Kind: policieskyvernoio.ImageValidatingPolicyKind},
			},
		},
	}
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer q.ShutDown()

	newPolicyExceptionHandler(c).Create(context.Background(), event.TypedCreateEvent[client.Object]{Object: polex}, q)

	var got []reconcile.Request
	for q.Len() > 0 {
		item, _ := q.Get()
		got = append(got, item)
		q.Done(item)
	}
	assert.ElementsMatch(t, []reconcile.Request{
		{NamespacedName: client.ObjectKey{Namespace: "team-a", Name: "require-team"}},
		{NamespacedName: client.ObjectKey{Namespace: "team-b", Name: "require-team"}},
		{NamespacedName: client.ObjectKey{Name: "cluster-policy"}},
	}, got)
}

func TestPolicyExceptionHandler_RequeuesOldAndNewRefs(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, policiesv1beta1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	polex := func(names ...string) *policiesv1beta1.PolicyException {
		var refs []policiesv1beta1.PolicyRef
		for _, name := range names {
			refs = append(refs, policiesv1beta1.PolicyRef{Name: name, Kind: policieskyvernoio.ImageValidatingPolicyKind})
		}
		return &policiesv1beta1.PolicyException{
			ObjectMeta: metav1.ObjectMeta{Name: "exempt", Namespace: "team-a"},
			Spec:       policiesv1beta1.PolicyExceptionSpec{PolicyRefs: refs},
		}
	}
	drain := func(q workqueue.TypedRateLimitingInterface[reconcile.Request]) []reconcile.Request {
		var got []reconcile.Request
		for q.Len() > 0 {
			item, _ := q.Get()
			got = append(got, item)
			q.Done(item)
		}
		return got
	}

	t.Run("update requeues refs removed from the exception", func(t *testing.T) {
		q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
		defer q.ShutDown()
		newPolicyExceptionHandler(c).Update(context.Background(), event.TypedUpdateEvent[client.Object]{
			ObjectOld: polex("policy-a", "policy-b"),
			ObjectNew: polex("policy-a"),
		}, q)
		assert.ElementsMatch(t, []reconcile.Request{
			{NamespacedName: client.ObjectKey{Name: "policy-a"}},
			{NamespacedName: client.ObjectKey{Name: "policy-b"}},
		}, drain(q))
	})

	t.Run("delete requeues the refs", func(t *testing.T) {
		q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
		defer q.ShutDown()
		newPolicyExceptionHandler(c).Delete(context.Background(), event.TypedDeleteEvent[client.Object]{
			Object: polex("policy-a"),
		}, q)
		assert.ElementsMatch(t, []reconcile.Request{
			{NamespacedName: client.ObjectKey{Name: "policy-a"}},
		}, drain(q))
	})
}
