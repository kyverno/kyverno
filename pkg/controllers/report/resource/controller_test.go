package resource

import (
	"context"
	"fmt"
	"sort"
	"testing"

	restmapperutils "github.com/kyverno/kyverno/pkg/utils/restmapper"
	"gotest.tools/v3/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func TestListResourceHashesRestartsExpiredSnapshot(t *testing.T) {
	expired := false
	list := func(_ context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
		if opts.Limit != 500 {
			return nil, fmt.Errorf("LIST limit = %d, want 500", opts.Limit)
		}
		if opts.Continue != "" {
			expired = true
			return nil, apierrors.NewResourceExpired("expired continuation")
		}
		page := &unstructured.UnstructuredList{}
		var obj unstructured.Unstructured
		if expired {
			page.SetResourceVersion("20")
			obj.SetUID("new")
		} else {
			page.SetResourceVersion("10")
			page.SetContinue("next")
			obj.SetUID("old")
		}
		page.Items = []unstructured.Unstructured{obj}
		return page, nil
	}

	hashes, rv, err := listResourceHashes(t.Context(), list)
	assert.NilError(t, err)
	if rv != "20" || len(hashes) != 1 || hashes["new"].Hash == "" {
		t.Fatalf("expected only the new resource hash at version 20, got %v at %q", hashes, rv)
	}
}
