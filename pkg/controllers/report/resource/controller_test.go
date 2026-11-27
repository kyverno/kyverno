package resource

import (
	"sort"
	"testing"

	restmapperutils "github.com/kyverno/kyverno/pkg/utils/restmapper"
	"gotest.tools/v3/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestUpdateResourceHashDoesNotRestoreDeletedUID(t *testing.T) {
	t.Parallel()

	const deletedUID types.UID = "deleted-uid"
	const liveUID types.UID = "live-uid"
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	deleted := Resource{Namespace: "default", Name: "settings", Hash: "deleted-hash"}
	live := Resource{Namespace: "default", Name: "settings", Hash: "live-hash"}
	c := &controller{
		dynamicWatchers: map[schema.GroupVersionResource]*watcher{
			gvr: {hashes: map[types.UID]Resource{deletedUID: deleted, liveUID: live}},
		},
	}
	obj := &unstructured.Unstructured{}
	obj.SetUID(deletedUID)
	c.deleteHash(obj, gvr)

	c.UpdateResourceHash(gvr, deletedUID, deleted)
	_, _, _, exists := c.GetResourceHash(deletedUID)
	assert.Assert(t, !exists)

	live.Hash = "updated-hash"
	c.UpdateResourceHash(gvr, liveUID, live)
	got, _, _, exists := c.GetResourceHash(liveUID)
	assert.Assert(t, exists)
	assert.Equal(t, got, live)
}

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
