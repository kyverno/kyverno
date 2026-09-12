package admissionpolicygenerator

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	vpolautogen "github.com/kyverno/kyverno/pkg/cel/policies/vpol/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

// TestDesiredVAPVariants covers the core fan-out decision for issue #17423: one VAP for the base
// policy, one more per translatable autogen group, and extraction-mode (custom-CRD) groups
// excluded from generation but still reported so status can note they stay webhook-only.
func TestDesiredVAPVariants(t *testing.T) {
	baseSpec := &policiesv1beta1.ValidatingPolicySpec{}

	tests := []struct {
		name        string
		configs     map[string]policiesv1beta1.ValidatingPolicyAutogen
		wantNames   []string
		wantSkipped []string
	}{
		{
			name:      "no autogen: base variant only",
			wantNames: []string{"vpol-test"},
		},
		{
			name: "one translatable group",
			configs: map[string]policiesv1beta1.ValidatingPolicyAutogen{
				"defaults": {Spec: baseSpec},
			},
			wantNames: []string{"vpol-test", "vpol-test-defaults"},
		},
		{
			name: "two translatable groups",
			configs: map[string]policiesv1beta1.ValidatingPolicyAutogen{
				"defaults": {Spec: baseSpec},
				"cronjobs": {Spec: baseSpec},
			},
			wantNames: []string{"vpol-test", "vpol-test-cronjobs", "vpol-test-defaults"},
		},
		{
			name: "extraction-mode group only: no group variant, reported as skipped",
			configs: map[string]policiesv1beta1.ValidatingPolicyAutogen{
				vpolautogen.ExtractionReplacementsRef: {Spec: baseSpec},
			},
			wantNames:   []string{"vpol-test"},
			wantSkipped: []string{vpolautogen.ExtractionReplacementsRef},
		},
		{
			name: "mixed: translatable group still generates even though a custom CRD is also autogen'd",
			configs: map[string]policiesv1beta1.ValidatingPolicyAutogen{
				"defaults":                            {Spec: baseSpec},
				vpolautogen.ExtractionReplacementsRef: {Spec: baseSpec},
			},
			wantNames:   []string{"vpol-test", "vpol-test-defaults"},
			wantSkipped: []string{vpolautogen.ExtractionReplacementsRef},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vpol := &policiesv1beta1.ValidatingPolicy{
				Status: policiesv1beta1.ValidatingPolicyStatus{
					Autogen: policiesv1beta1.ValidatingPolicyAutogenStatus{Configs: tt.configs},
				},
			}

			variants, skipped := desiredVAPVariants(vpol, "vpol-test")

			var gotNames []string
			for _, v := range variants {
				gotNames = append(gotNames, v.name)
				if v.name == "vpol-test" {
					assert.Nil(t, v.specOverride, "base variant must use the policy's own spec, not an override")
				} else {
					assert.NotNil(t, v.specOverride, "group variant must carry the autogen'd spec")
				}
			}
			assert.ElementsMatch(t, tt.wantNames, gotNames)
			assert.ElementsMatch(t, tt.wantSkipped, skipped)
		})
	}
}

// mockVAPLister is a minimal in-memory ValidatingAdmissionPolicyLister for testing GC logic
// without a full fake clientset.
type mockVAPLister struct {
	items []*admissionregistrationv1.ValidatingAdmissionPolicy
}

func (m *mockVAPLister) List(selector labels.Selector) ([]*admissionregistrationv1.ValidatingAdmissionPolicy, error) {
	return m.items, nil
}

func (m *mockVAPLister) Get(name string) (*admissionregistrationv1.ValidatingAdmissionPolicy, error) {
	for _, v := range m.items {
		if v.Name == name {
			return v, nil
		}
	}
	return nil, nil
}

// TestListOwnedVAPNames_DoesNotCollideAcrossPolicies guards against a real hazard in the GC
// design: generated VAP names share a "<baseName>-<group>" pattern, so a naive name-prefix match
// (without also checking ownership) would let policy "foo"'s GC pass accidentally delete a VAP
// that actually belongs to an unrelated policy literally named "foo-defaults". Ownership must be
// decided by OwnerReferences UID, with the name check only ever a cheap pre-filter.
func TestListOwnedVAPNames_DoesNotCollideAcrossPolicies(t *testing.T) {
	fooUID := types.UID("foo-uid")
	otherUID := types.UID("other-policy-uid")

	c := &controller{
		vapLister: &mockVAPLister{
			items: []*admissionregistrationv1.ValidatingAdmissionPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "vpol-foo",
						OwnerReferences: []metav1.OwnerReference{{UID: fooUID}},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "vpol-foo-defaults",
						OwnerReferences: []metav1.OwnerReference{{UID: fooUID}},
					},
				},
				{
					// Name collides with policy "foo"'s naming pattern but is owned by a
					// different policy (e.g. a real ValidatingPolicy literally named
					// "foo-defaults") - must never be returned for policy "foo".
					ObjectMeta: metav1.ObjectMeta{
						Name:            "vpol-foo-defaults-extra",
						OwnerReferences: []metav1.OwnerReference{{UID: otherUID}},
					},
				},
			},
		},
	}

	policy := engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", UID: fooUID},
	})

	names, err := c.listOwnedVAPNames(policy, "vpol-foo")
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"vpol-foo", "vpol-foo-defaults"}, names)
}
