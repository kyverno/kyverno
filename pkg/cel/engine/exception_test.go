package engine

import (
	"errors"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type fakePolicyExceptionLister struct {
	err        error
	exceptions []*policiesv1beta1.PolicyException
}

func (l *fakePolicyExceptionLister) List(selector labels.Selector) ([]*policiesv1beta1.PolicyException, error) {
	return l.exceptions, l.err
}

func TestListExceptions(t *testing.T) {
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
			err: errors.New("dummy"),
		},
		wantErr: true,
	}, {
		name: "name doesn't match",
		lister: &fakePolicyExceptionLister{
			exceptions: []*policiesv1beta1.PolicyException{{
				Spec: policiesv1beta1.PolicyExceptionSpec{
					PolicyRefs: []policiesv1beta1.PolicyRef{{
						Kind: "foo",
						Name: "bar",
					}},
				},
			}},
		},
		policyKind: "foo",
		policyName: "other",
	}, {
		name: "kind doesn't match",
		lister: &fakePolicyExceptionLister{
			exceptions: []*policiesv1beta1.PolicyException{{
				Spec: policiesv1beta1.PolicyExceptionSpec{
					PolicyRefs: []policiesv1beta1.PolicyRef{{
						Kind: "foo",
						Name: "bar",
					}},
				},
			}},
		},
		policyKind: "other",
		policyName: "bar",
	}, {
		name: "match",
		lister: &fakePolicyExceptionLister{
			exceptions: []*policiesv1beta1.PolicyException{{
				Spec: policiesv1beta1.PolicyExceptionSpec{
					PolicyRefs: []policiesv1beta1.PolicyRef{{
						Kind: "foo",
						Name: "bar",
					}},
				},
			}},
		},
		policyKind: "foo",
		policyName: "bar",
		want: []*policiesv1beta1.PolicyException{{
			Spec: policiesv1beta1.PolicyExceptionSpec{
				PolicyRefs: []policiesv1beta1.PolicyRef{{
					Kind: "foo",
					Name: "bar",
				}},
			},
		}},
	}, {
		name: "expired exception is ignored",
		lister: &fakePolicyExceptionLister{
			exceptions: []*policiesv1beta1.PolicyException{{
				Spec: policiesv1beta1.PolicyExceptionSpec{
					PolicyRefs: []policiesv1beta1.PolicyRef{{
						Kind: "foo",
						Name: "bar",
					}},
					ExpiresAt: &metav1.Time{
						Time: time.Now().Add(-time.Minute),
					},
				},
			}},
		},
		policyKind: "foo",
		policyName: "bar",
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ListExceptions(tt.lister, tt.policyKind, "", tt.policyName)
			assert.Equal(t, tt.want, got)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
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

	t.Run("cluster-wide", func(t *testing.T) {
		lister := NewManagerPolicyExceptionLister(fakeClient, "")
		got, err := ListExceptions(lister, "ValidatingPolicy", "", "require-team")
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "exempt", got[0].Name)
	})

	t.Run("namespace scoped to a different namespace finds nothing", func(t *testing.T) {
		lister := NewManagerPolicyExceptionLister(fakeClient, "other-namespace")
		got, err := ListExceptions(lister, "ValidatingPolicy", "", "require-team")
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("namespace scoped to the matching namespace finds it", func(t *testing.T) {
		lister := NewManagerPolicyExceptionLister(fakeClient, "kyverno")
		got, err := ListExceptions(lister, "ValidatingPolicy", "", "require-team")
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "exempt", got[0].Name)
	})
}

// A PolicyRef has no namespace, so for a namespaced policy the exception's own namespace is the
// scope: only exceptions in the policy's namespace apply. A cluster-scoped policy keeps accepting
// exceptions from any namespace the lister serves.
func TestMatchExceptions_NamespaceScope(t *testing.T) {
	polex := func(namespace, kind string) *policiesv1beta1.PolicyException {
		return &policiesv1beta1.PolicyException{
			ObjectMeta: metav1.ObjectMeta{Name: "exempt", Namespace: namespace},
			Spec: policiesv1beta1.PolicyExceptionSpec{
				PolicyRefs: []policiesv1beta1.PolicyRef{{Name: "require-team", Kind: kind}},
			},
		}
	}
	tests := []struct {
		name            string
		exceptions      []*policiesv1beta1.PolicyException
		kind, namespace string
		want            []string
	}{{
		name:       "namespaced policy: exception in the same namespace applies",
		exceptions: []*policiesv1beta1.PolicyException{polex("team-a", "NamespacedValidatingPolicy")},
		kind:       "NamespacedValidatingPolicy", namespace: "team-a",
		want: []string{"team-a"},
	}, {
		name:       "namespaced policy: exception in another namespace does not apply",
		exceptions: []*policiesv1beta1.PolicyException{polex("team-b", "NamespacedValidatingPolicy")},
		kind:       "NamespacedValidatingPolicy", namespace: "team-a",
	}, {
		name: "namespaced policy: only the same-namespace exception is kept",
		exceptions: []*policiesv1beta1.PolicyException{
			polex("team-b", "NamespacedValidatingPolicy"),
			polex("team-a", "NamespacedValidatingPolicy"),
			polex("kyverno", "NamespacedValidatingPolicy"),
		},
		kind: "NamespacedValidatingPolicy", namespace: "team-a",
		want: []string{"team-a"},
	}, {
		name: "cluster policy: exceptions from any namespace apply",
		exceptions: []*policiesv1beta1.PolicyException{
			polex("team-a", "ValidatingPolicy"),
			polex("kyverno", "ValidatingPolicy"),
		},
		kind: "ValidatingPolicy", namespace: "",
		want: []string{"team-a", "kyverno"},
	}, {
		name: "an exception that lists the policy twice is returned once",
		exceptions: func() []*policiesv1beta1.PolicyException {
			dup := polex("team-a", "NamespacedValidatingPolicy")
			dup.Spec.PolicyRefs = append(dup.Spec.PolicyRefs, dup.Spec.PolicyRefs[0])
			return []*policiesv1beta1.PolicyException{dup}
		}(),
		kind: "NamespacedValidatingPolicy", namespace: "team-a",
		want: []string{"team-a"},
	}, {
		name:       "nil entries are skipped",
		exceptions: []*policiesv1beta1.PolicyException{nil, polex("team-a", "NamespacedValidatingPolicy")},
		kind:       "NamespacedValidatingPolicy", namespace: "team-a",
		want: []string{"team-a"},
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchExceptions(tt.exceptions, tt.kind, tt.namespace, "require-team")
			namespaces := make([]string, 0, len(got))
			for _, e := range got {
				namespaces = append(namespaces, e.GetNamespace())
			}
			if tt.want == nil {
				assert.Empty(t, namespaces)
				return
			}
			assert.Equal(t, tt.want, namespaces)
		})
	}
}
