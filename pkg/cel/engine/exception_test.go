package engine

import (
	"errors"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type fakePolicyExceptionLister struct {
	err        error
	exceptions []*policiesv1beta1.PolicyException
	scope      ExceptionScope
}

func (l *fakePolicyExceptionLister) List(selector labels.Selector) ([]*policiesv1beta1.PolicyException, error) {
	return l.exceptions, l.err
}

func (l *fakePolicyExceptionLister) ListForPolicy(kind, name, namespace string) ([]*policiesv1beta1.PolicyException, error) {
	exceptions, err := l.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	return l.scope.Select(exceptions, kind, name, namespace), nil
}

func exceptionWithRef(namespace, kind, name string) *policiesv1beta1.PolicyException {
	return &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Kind: kind, Name: name}},
		},
	}
}

func TestExceptionScope_Select(t *testing.T) {
	tests := []struct {
		name       string
		lister     PolicyExceptionLister
		policyKind string
		policyName string
		want       []*policiesv1beta1.PolicyException
		wantErr    bool
	}{{
		name: "with error",
		lister: &fakePolicyExceptionLister{
			err:   errors.New("dummy"),
			scope: ExceptionScopeAll,
		},
		wantErr: true,
	}, {
		name: "name doesn't match",
		lister: &fakePolicyExceptionLister{
			exceptions: []*policiesv1beta1.PolicyException{exceptionWithRef("", "foo", "bar")},
			scope:      ExceptionScopeAll,
		},
		policyKind: "foo",
		policyName: "other",
	}, {
		name: "kind doesn't match",
		lister: &fakePolicyExceptionLister{
			exceptions: []*policiesv1beta1.PolicyException{exceptionWithRef("", "foo", "bar")},
			scope:      ExceptionScopeAll,
		},
		policyKind: "other",
		policyName: "bar",
	}, {
		name: "match",
		lister: &fakePolicyExceptionLister{
			exceptions: []*policiesv1beta1.PolicyException{exceptionWithRef("", "foo", "bar")},
			scope:      ExceptionScopeAll,
		},
		policyKind: "foo",
		policyName: "bar",
		want:       []*policiesv1beta1.PolicyException{exceptionWithRef("", "foo", "bar")},
	}, {
		name: "expired exception is ignored",
		lister: &fakePolicyExceptionLister{
			exceptions: []*policiesv1beta1.PolicyException{{
				Spec: policiesv1beta1.PolicyExceptionSpec{
					PolicyRefs: []policiesv1beta1.PolicyRef{{Kind: "foo", Name: "bar"}},
					ExpiresAt:  &metav1.Time{Time: time.Now().Add(-time.Minute)},
				},
			}},
			scope: ExceptionScopeAll,
		},
		policyKind: "foo",
		policyName: "bar",
	}, {
		name: "scope none rejects everything",
		lister: &fakePolicyExceptionLister{
			exceptions: []*policiesv1beta1.PolicyException{exceptionWithRef("", "foo", "bar")},
			scope:      ExceptionScopeNone,
		},
		policyKind: "foo",
		policyName: "bar",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.lister.ListForPolicy(tt.policyKind, tt.policyName, "")
			assert.Equal(t, tt.want, got)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestExceptionScope_Applies covers the trust table: whether a polex in a given namespace
// applies to a policy, cluster-scoped or namespaced, under each scope value.
func TestExceptionScope_Applies(t *testing.T) {
	refKind, refName := "ValidatingPolicy", "require-team"
	matching := func(namespace string) *policiesv1beta1.PolicyException {
		return exceptionWithRef(namespace, refKind, refName)
	}

	tests := []struct {
		name     string
		scope    ExceptionScope
		polex    *policiesv1beta1.PolicyException
		policyNS string // "" = cluster-scoped policy
		want     bool
	}{
		{name: "none scope / cluster-scoped policy / any namespace", scope: ExceptionScopeNone, polex: matching("ns-a"), policyNS: "", want: false},
		{name: "none scope / namespaced policy / own namespace", scope: ExceptionScopeNone, polex: matching("ns-a"), policyNS: "ns-a", want: false},
		{name: "all scope / cluster-scoped policy / any namespace accepted", scope: ExceptionScopeAll, polex: matching("ns-a"), policyNS: "", want: true},
		{name: "all scope / namespaced policy / own namespace", scope: ExceptionScopeAll, polex: matching("ns-a"), policyNS: "ns-a", want: true},
		{name: "all scope / namespaced policy / different namespace rejected", scope: ExceptionScopeAll, polex: matching("ns-b"), policyNS: "ns-a", want: false},
		{name: "all scope / namespaced policy / exception with empty namespace rejected", scope: ExceptionScopeAll, polex: matching(""), policyNS: "ns-a", want: false},
		{name: "concrete scope / cluster-scoped policy / matching namespace", scope: ExceptionScope("kyverno"), polex: matching("kyverno"), policyNS: "", want: true},
		{name: "concrete scope / cluster-scoped policy / non-matching namespace rejected", scope: ExceptionScope("kyverno"), polex: matching("other"), policyNS: "", want: false},
		{name: "concrete scope / namespaced policy / own namespace equal to central namespace", scope: ExceptionScope("kyverno"), polex: matching("kyverno"), policyNS: "kyverno", want: true},
		{name: "concrete scope / namespaced policy / exception in the central namespace", scope: ExceptionScope("kyverno"), polex: matching("kyverno"), policyNS: "ns-a", want: true},
		{name: "concrete scope / namespaced policy / exception namespace not accepted by scope rejected", scope: ExceptionScope("kyverno"), polex: matching("ns-a"), policyNS: "ns-a", want: false},
		{name: "concrete scope / namespaced policy / exception elsewhere rejected", scope: ExceptionScope("kyverno"), polex: matching("ns-b"), policyNS: "ns-a", want: false},
		{name: "expired exception always rejected", scope: ExceptionScopeAll, polex: func() *policiesv1beta1.PolicyException {
			p := matching("ns-a")
			p.Spec.ExpiresAt = &metav1.Time{Time: time.Now().Add(-time.Minute)}
			return p
		}(), policyNS: "ns-a", want: false},
		{name: "no matching ref rejected", scope: ExceptionScopeAll, polex: exceptionWithRef("ns-a", "other", "other"), policyNS: "ns-a", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.scope.Applies(tt.polex, refKind, refName, tt.policyNS)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestManagerPolicyExceptionLister_ReadsThroughSharedReader is a regression test
// for issue #16989: the CEL policy engines (vpol, ivpol, mpol) register their
// PolicyException watch on a controller-runtime manager cache and cache compiled
// results between triggering events. Looking exceptions up through a *different*
// cache than the one that delivered the watch event risks reading a stale view
// and permanently compiling the policy without an exception that already exists.
// NewManagerPolicyExceptionLister must read through the same reader (typically
// the manager's own client) used to drive reconciliation, so it always observes
// an exception the reconcile it triggered is meant to see.
func TestManagerPolicyExceptionLister_ReadsThroughSharedReader(t *testing.T) {
	scheme := kruntime.NewScheme()
	require.NoError(t, policiesv1beta1.Install(scheme))

	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "exempt", Namespace: "kyverno"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Kind: "ValidatingPolicy", Name: "require-team"}},
		},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(exception).Build()

	t.Run("unset scope accepts nothing", func(t *testing.T) {
		lister := NewManagerPolicyExceptionLister(fakeClient, "")
		got, err := lister.ListForPolicy("ValidatingPolicy", "require-team", "")
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("all scope is cluster-wide for a cluster-scoped policy", func(t *testing.T) {
		lister := NewManagerPolicyExceptionLister(fakeClient, "*")
		got, err := lister.ListForPolicy("ValidatingPolicy", "require-team", "")
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "exempt", got[0].Name)
	})

	t.Run("namespace scoped to a different namespace finds nothing", func(t *testing.T) {
		lister := NewManagerPolicyExceptionLister(fakeClient, "other-namespace")
		got, err := lister.ListForPolicy("ValidatingPolicy", "require-team", "")
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("namespace scoped to the matching namespace finds it", func(t *testing.T) {
		lister := NewManagerPolicyExceptionLister(fakeClient, "kyverno")
		got, err := lister.ListForPolicy("ValidatingPolicy", "require-team", "")
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "exempt", got[0].Name)
	})
}

// TestPolicyExceptionLister_ReadsThroughClientGoLister exercises the production
// client-go backed lister (not the manager/controller-runtime one) over an indexer,
// covering the "" and concrete-namespace branches of ExceptionScope against the
// real List() path used in the reports controller.
func TestPolicyExceptionLister_ReadsThroughClientGoLister(t *testing.T) {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "exempt", Namespace: "kyverno"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs: []policiesv1beta1.PolicyRef{{Kind: "ValidatingPolicy", Name: "require-team"}},
		},
	}
	require.NoError(t, indexer.Add(exception))
	lister := policiesv1beta1listers.NewPolicyExceptionLister(indexer)

	t.Run("unset scope accepts nothing", func(t *testing.T) {
		l := NewPolicyExceptionLister(lister, "")
		got, err := l.ListForPolicy("ValidatingPolicy", "require-team", "")
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("concrete scope finds its own namespace", func(t *testing.T) {
		l := NewPolicyExceptionLister(lister, "kyverno")
		got, err := l.ListForPolicy("ValidatingPolicy", "require-team", "")
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "exempt", got[0].Name)
	})

	t.Run("concrete scope finds nothing in a different namespace", func(t *testing.T) {
		l := NewPolicyExceptionLister(lister, "other")
		got, err := l.ListForPolicy("ValidatingPolicy", "require-team", "")
		require.NoError(t, err)
		assert.Empty(t, got)
	})
}
