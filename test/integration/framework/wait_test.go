package framework

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	ivpolengine "github.com/kyverno/kyverno/pkg/cel/policies/ivpol/engine"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	vpolengine "github.com/kyverno/kyverno/pkg/cel/policies/vpol/engine"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
)

func vpol(name string) vpolengine.Policy {
	return vpolengine.Policy{Policy: &policiesv1beta1.ValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: name}}}
}

func TestWaitForVpols_WaitsForEveryNamedPolicyNotForAnEntryCount(t *testing.T) {
	// The first policy compiles with its autogen entries (several entries under one name) before
	// the second one shows up. A count of two is reached early; the wait must hold until both names
	// are there.
	var calls atomic.Int32
	provider := vpolengine.ProviderFunc(func(context.Context) ([]vpolengine.Policy, error) {
		if calls.Add(1) < 3 {
			return []vpolengine.Policy{vpol("deny-latest-image"), vpol("deny-latest-image"), vpol("deny-latest-image")}, nil
		}
		return []vpolengine.Policy{vpol("deny-latest-image"), vpol("deny-latest-image"), vpol("warn-missing-limits")}, nil
	})

	WaitForVpols(t, provider, "deny-latest-image", "warn-missing-limits")

	assert.GreaterOrEqual(t, calls.Load(), int32(3), "the wait returned before the second policy was compiled")
}

func TestWaitForVpols_RetriesAfterAFetchError(t *testing.T) {
	var calls atomic.Int32
	provider := vpolengine.ProviderFunc(func(context.Context) ([]vpolengine.Policy, error) {
		if calls.Add(1) == 1 {
			return nil, errors.New("cache not synced")
		}
		return []vpolengine.Policy{vpol("require-team-label")}, nil
	})

	WaitForVpols(t, provider, "require-team-label")

	assert.GreaterOrEqual(t, calls.Load(), int32(2))
}

func TestWaitForIvpols_MatchesNamespacedPoliciesByNamespaceAndName(t *testing.T) {
	var calls atomic.Int32
	provider := ivpolengine.ProviderFunc(func(context.Context) ([]ivpolengine.Policy, error) {
		calls.Add(1)
		policies := []ivpolengine.Policy{
			// Same name in another namespace must not satisfy the wait.
			{Policy: &policiesv1beta1.NamespacedImageValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "require-signed", Namespace: "team-b"}}},
		}
		if calls.Load() >= 3 {
			policies = append(policies, ivpolengine.Policy{
				Policy: &policiesv1beta1.NamespacedImageValidatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "require-signed", Namespace: "team-a"}},
			})
		}
		return policies, nil
	})

	WaitForIvpols(t, provider, "team-a/require-signed")

	assert.GreaterOrEqual(t, calls.Load(), int32(3), "a policy with the same name in another namespace satisfied the wait")
}

// mpolProvider is a minimal mpolengine.Provider; the wait only calls Fetch.
type mpolProvider struct {
	fetch func() []mpolengine.Policy
}

func (p mpolProvider) Fetch(context.Context, bool) []mpolengine.Policy { return p.fetch() }

func (mpolProvider) MatchesMutateExisting(context.Context, admission.Attributes, *admissionv1.AdmissionRequest, *corev1.Namespace, func() (map[string]any, error)) []string {
	return nil
}

func TestWaitForMpols_WaitsForTheNamedPolicy(t *testing.T) {
	var calls atomic.Int32
	provider := mpolProvider{fetch: func() []mpolengine.Policy {
		if calls.Add(1) < 2 {
			return nil
		}
		return []mpolengine.Policy{{Policy: &policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "add-team-label"}}}}
	}}

	WaitForMpols(t, provider, "add-team-label")

	assert.GreaterOrEqual(t, calls.Load(), int32(2))
}
