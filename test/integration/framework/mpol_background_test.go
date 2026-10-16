package framework

import (
	"context"
	"strings"
	"testing"
	"time"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apiserver/pkg/admission"
)

func TestUniqueName(t *testing.T) {
	t.Parallel()
	first, second := UniqueName(t, "ns"), UniqueName(t, "ns")
	assert.NotEqual(t, first, second)
	assert.True(t, strings.HasPrefix(first, "ns-"))
	assert.Empty(t, validation.IsDNS1123Label(first))

	long := UniqueName(t, strings.Repeat("a", 70))
	assert.LessOrEqual(t, len(long), 63)
	assert.Empty(t, validation.IsDNS1123Label(long))
}

// staticProvider serves whatever compiled policies a test sets.
type staticProvider struct{ policies []mpolengine.Policy }

func (p *staticProvider) Fetch(context.Context, bool) []mpolengine.Policy { return p.policies }

func (p *staticProvider) MatchesMutateExisting(context.Context, admission.Attributes, *admissionv1.AdmissionRequest, *corev1.Namespace, func() (map[string]any, error)) []string {
	return nil
}

func compiledMpol(name string, generation int64) mpolengine.Policy {
	return mpolengine.Policy{
		Policy:         &policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: name, Generation: generation}},
		CompiledPolicy: &mpolcompiler.Policy{},
	}
}

func TestDelayedMpolProvider(t *testing.T) {
	t.Parallel()
	inner := &staticProvider{policies: []mpolengine.Policy{compiledMpol("pol", 1)}}
	delayed := NewDelayedMpolProvider(inner, 100*time.Millisecond)
	ctx := context.Background()

	assert.Empty(t, delayed.Fetch(ctx, false), "a new policy is hidden until the delay passes")
	require.Eventually(t, func() bool { return len(delayed.Fetch(ctx, false)) == 1 }, time.Second, 10*time.Millisecond)

	// an update: the previous version is served until the new one has been seen for the delay
	inner.policies = []mpolengine.Policy{compiledMpol("pol", 2)}
	got := delayed.Fetch(ctx, false)
	require.Len(t, got, 1)
	assert.Equal(t, int64(1), got[0].Policy.GetGeneration())
	require.Eventually(t, func() bool {
		got := delayed.Fetch(ctx, false)
		return len(got) == 1 && got[0].Policy.GetGeneration() == 2
	}, time.Second, 10*time.Millisecond)

	// a deletion is visible at once
	inner.policies = nil
	assert.Empty(t, delayed.Fetch(ctx, false))

	delayed.SetDelay(0)
	inner.policies = []mpolengine.Policy{compiledMpol("pol", 3)}
	assert.Len(t, delayed.Fetch(ctx, false), 1, "without a delay a version is published when first fetched")
	assert.Empty(t, delayed.Fetch(ctx, true), "mutateExisting filters out policies without mutateExisting")
}
