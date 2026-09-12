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
			got, err := ListExceptions(tt.lister, tt.policyKind, tt.policyName)
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
		got, err := ListExceptions(lister, "ValidatingPolicy", "require-team")
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "exempt", got[0].Name)
	})

	t.Run("namespace scoped to a different namespace finds nothing", func(t *testing.T) {
		lister := NewManagerPolicyExceptionLister(fakeClient, "other-namespace")
		got, err := ListExceptions(lister, "ValidatingPolicy", "require-team")
		require.NoError(t, err)
		assert.Empty(t, got)
	})

	t.Run("namespace scoped to the matching namespace finds it", func(t *testing.T) {
		lister := NewManagerPolicyExceptionLister(fakeClient, "kyverno")
		got, err := ListExceptions(lister, "ValidatingPolicy", "require-team")
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "exempt", got[0].Name)
	})
}
