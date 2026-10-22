package aggregate

import (
	"context"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/api/kyverno"
	"github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

// TestBackReconcile_NoEphemeralReports_NoWrite proves decision C1: when a
// permanent report already matches what merging its own (ephemeral-report-free)
// results would produce, backReconcile does not issue an update.
// This is an in-package test since backReconcile is unexported.
func TestBackReconcile_NoEphemeralReports_NoWrite(t *testing.T) {
	stablePolr := &v1alpha2.PolicyReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "stable-polr",
			Namespace:       "default",
			ResourceVersion: "1",
			Labels: map[string]string{
				kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp,
			},
		},
		Results: []v1alpha2.PolicyReportResult{
			{Source: reportutils.SourceValidatingPolicy, Policy: "require-app-label"},
		},
	}
	client := versionedfake.NewSimpleClientset(stablePolr)

	// seed a vpol lister so maps.Vpol matches the report's existing result
	vpolIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	_ = vpolIndexer.Add(&policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "require-app-label"},
	})

	c := &controller{
		client:                  client,
		vpolLister:              policiesv1beta1listers.NewValidatingPolicyLister(vpolIndexer),
		cacheMu:                 &sync.Mutex{},
		reportUUIDToPolicyCache: map[string]sets.Set[string]{},
	}

	err := c.backReconcile(context.TODO(), logr.Discard(), "", "default", "stable-polr")
	assert.NoError(t, err)

	var sawGet, sawUpdate bool
	for _, a := range client.Fake.Actions() {
		if a.GetResource().Resource != "policyreports" {
			continue
		}
		switch action := a.(type) {
		case k8stesting.GetAction:
			if action.GetName() == stablePolr.Name {
				sawGet = true
			}
		case k8stesting.UpdateAction:
			sawUpdate = true
			_ = action
		}
	}
	assert.True(t, sawGet, "expected backReconcile to fetch the report")
	assert.False(t, sawUpdate, "expected no update when merging would produce no change (C1)")
}

// TestBackReconcile_NoEphemeralReports_PrunesLegacy_Write is the C1 negative
// control: a report carrying a legacy result in addition to a matching CEL
// result is not identical after the merge (the legacy result is dropped), so
// backReconcile must still PUT, and the PUT body must hold only the vpol result.
func TestBackReconcile_NoEphemeralReports_PrunesLegacy_Write(t *testing.T) {
	mixedPolr := &v1alpha2.PolicyReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "mixed-polr",
			Namespace:       "default",
			ResourceVersion: "1",
			Labels: map[string]string{
				kyverno.LabelAppManagedBy: kyverno.ValueKyvernoApp,
			},
		},
		Results: []v1alpha2.PolicyReportResult{
			{Source: reportutils.SourceValidatingPolicy, Policy: "require-app-label"},
			{Source: reportutils.SourceKyverno, Policy: "require-app-label", Rule: "check-app-label"},
		},
	}
	client := versionedfake.NewSimpleClientset(mixedPolr)

	vpolIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	_ = vpolIndexer.Add(&policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "require-app-label"},
	})

	c := &controller{
		client:                  client,
		vpolLister:              policiesv1beta1listers.NewValidatingPolicyLister(vpolIndexer),
		cacheMu:                 &sync.Mutex{},
		reportUUIDToPolicyCache: map[string]sets.Set[string]{},
	}

	err := c.backReconcile(context.TODO(), logr.Discard(), "", "default", "mixed-polr")
	assert.NoError(t, err)

	var updates []*v1alpha2.PolicyReport
	for _, a := range client.Fake.Actions() {
		if a.GetResource().Resource != "policyreports" {
			continue
		}
		if action, ok := a.(k8stesting.UpdateAction); ok {
			updates = append(updates, action.GetObject().(*v1alpha2.PolicyReport))
		}
	}
	if assert.Len(t, updates, 1, "expected exactly one update pruning the legacy result") {
		if assert.Len(t, updates[0].Results, 1) {
			assert.Equal(t, reportutils.SourceValidatingPolicy, updates[0].Results[0].Source)
		}
	}
}
