package resource

import (
	"sort"
	"testing"

	restmapperutils "github.com/kyverno/kyverno/pkg/utils/restmapper"
	"gotest.tools/v3/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func matchResourcesFor(group, version, resource string) *admissionregistrationv1.MatchResources {
	return &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
			{
				RuleWithOperations: admissionregistrationv1.RuleWithOperations{
					Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.OperationAll},
					Rule: admissionregistrationv1.Rule{
						APIGroups:   []string{group},
						APIVersions: []string{version},
						Resources:   []string{resource},
					},
				},
			},
		},
	}
}

// TestKindsFromPolicyAndAutogen verifies that kindsFromPolicyAndAutogen aggregates kinds from a
// policy's own match constraints with every autogen'd config's match constraints - this is the fix
// for #17369: the IVPol/NIVPol branches of updateDynamicWatchers previously only looked at a policy's
// own match constraints and never picked up autogen'd targets (built-in controllers, or custom-CRD
// extraction-mode targets), so background-scan never watched/scanned them.
func TestKindsFromPolicyAndAutogen(t *testing.T) {
	restMapper, err := restmapperutils.GetRESTMapper(nil)
	assert.NilError(t, err)

	tests := []struct {
		name                    string
		matchConstraints        *admissionregistrationv1.MatchResources
		autogenMatchConstraints []*admissionregistrationv1.MatchResources
		wantKinds               []string
	}{
		{
			name:                    "no autogen configs, only base kinds",
			matchConstraints:        matchResourcesFor("", "v1", "pods"),
			autogenMatchConstraints: nil,
			wantKinds:               []string{"v1/Pod"},
		},
		{
			name:             "autogen'd kinds are included alongside the base kind",
			matchConstraints: matchResourcesFor("", "v1", "pods"),
			autogenMatchConstraints: []*admissionregistrationv1.MatchResources{
				matchResourcesFor("apps", "v1", "deployments"),
				matchResourcesFor("apps", "v1", "statefulsets"),
			},
			wantKinds: []string{"v1/Pod", "apps/v1/Deployment", "apps/v1/StatefulSet"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kinds := kindsFromPolicyAndAutogen(tt.matchConstraints, tt.autogenMatchConstraints, restMapper)
			sort.Strings(kinds)
			want := append([]string{}, tt.wantKinds...)
			sort.Strings(want)
			assert.DeepEqual(t, kinds, want)
		})
	}
}
