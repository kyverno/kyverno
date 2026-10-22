package aggregate

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/policyreport/v1alpha2"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	metafake "k8s.io/client-go/metadata/fake"
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

// TestBackReconcile_WithEphemeralReport_StillWrites pins the
// `len(ephemeralReports) == 0` guard on C1: with an owned ephemeral report
// present (even one whose result is identical to what's already stored),
// backReconcile must still PUT, since C1 only applies when there is nothing
// left to merge in.
func TestBackReconcile_WithEphemeralReport_StillWrites(t *testing.T) {
	stablePolr := &v1alpha2.PolicyReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "stable-polr-with-ephr",
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
	ephr := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ephr-for-stable-polr",
			Namespace: "default",
			Labels: map[string]string{
				reportutils.LabelResourceUid: "stable-polr-with-ephr",
			},
			OwnerReferences: []metav1.OwnerReference{
				{APIVersion: "v1", Kind: "Pod", Name: "test-pod", UID: "stable-polr-with-ephr"},
			},
		},
		Spec: reportsv1.EphemeralReportSpec{
			Results: []openreportsv1alpha1.ReportResult{
				{Source: reportutils.SourceValidatingPolicy, Policy: "require-app-label"},
			},
		},
	}
	client := versionedfake.NewSimpleClientset(stablePolr, ephr)

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

	err := c.backReconcile(context.TODO(), logr.Discard(), "", "default", "stable-polr-with-ephr")
	assert.NoError(t, err)

	var sawUpdate bool
	for _, a := range client.Fake.Actions() {
		if a.GetResource().Resource != "policyreports" {
			continue
		}
		if _, ok := a.(k8stesting.UpdateAction); ok {
			sawUpdate = true
		}
	}
	assert.True(t, sawUpdate, "expected an update even though the merged result matches the stored one, because an ephemeral report was present")
}

// TestLegacyPoliciesPresent covers every branch of the startup-sweep gate. Run
// sweeps only on (true, nil); a non-nil error must close the gate, not open it.
// In-package because legacyPoliciesPresent is unexported.
func TestLegacyPoliciesPresent(t *testing.T) {
	legacyGVR := kyvernov1.SchemeGroupVersion.WithResource("clusterpolicies")
	cpol := &metav1.PartialObjectMetadata{
		TypeMeta:   metav1.TypeMeta{APIVersion: "kyverno.io/v1", Kind: "ClusterPolicy"},
		ObjectMeta: metav1.ObjectMeta{Name: "require-labels"},
	}

	tests := []struct {
		name        string
		seed        bool
		reactor     func() (bool, runtime.Object, error)
		wantPresent bool
		wantErr     bool
	}{{
		name:        "a legacy policy exists",
		seed:        true,
		wantPresent: true,
	}, {
		name:        "no legacy policies exist",
		wantPresent: false,
	}, {
		name: "the CRD is gone",
		reactor: func() (bool, runtime.Object, error) {
			return true, nil, apierrors.NewNotFound(legacyGVR.GroupResource(), "")
		},
	}, {
		name: "listing is forbidden",
		reactor: func() (bool, runtime.Object, error) {
			return true, nil, apierrors.NewForbidden(legacyGVR.GroupResource(), "", errors.New("no list verb"))
		},
		wantErr: true,
	}, {
		name:    "the API server is unavailable",
		reactor: func() (bool, runtime.Object, error) { return true, nil, errors.New("connection refused") },
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := metafake.NewTestScheme()
			metav1.AddMetaToScheme(s)
			var objects []runtime.Object
			if tt.seed {
				objects = append(objects, cpol)
			}
			client := metafake.NewSimpleMetadataClient(s, objects...)
			if tt.reactor != nil {
				client.PrependReactor("list", "clusterpolicies", func(k8stesting.Action) (bool, runtime.Object, error) {
					return tt.reactor()
				})
			}

			present, err := (&controller{metaClient: client}).legacyPoliciesPresent(context.TODO())

			if tt.wantErr {
				assert.Error(t, err)
				assert.False(t, present, "an indeterminate gate must never report present")
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantPresent, present)
		})
	}
}
