package generate

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"gotest.tools/v3/assert"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestGenerateCloneListKindResolution(t *testing.T) {
	t.Parallel()
	core := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	v1 := schema.GroupVersionResource{Group: "example.test", Version: "v1", Resource: "widgets"}
	v2 := schema.GroupVersionResource{Group: "example.test", Version: "v2", Resource: "widgets"}
	other := schema.GroupVersionResource{Group: "other.test", Version: "v1", Resource: "widgets"}
	for _, test := range []struct {
		name       string
		kind       string
		registered []schema.GroupVersionResource
		preferred  string
		want       schema.GroupVersionResource
		denied     bool
		wantError  string
	}{
		{name: "kind-only core", kind: "ConfigMap", want: core},
		{name: "explicit core version", kind: "v1/ConfigMap", want: core},
		{name: "kind-only custom preferred version", kind: "Widget", registered: []schema.GroupVersionResource{v1, v2}, preferred: "v2", want: v2},
		{name: "explicit custom version overrides preference", kind: "example.test/v1/Widget", registered: []schema.GroupVersionResource{v2, v1}, preferred: "v2", want: v1},
		{name: "ambiguous versions", kind: "Widget", registered: []schema.GroupVersionResource{v1, v2}, wantError: "failed to get GVR"},
		{name: "ambiguous groups", kind: "Widget", registered: []schema.GroupVersionResource{other, v1}, preferred: "v1", wantError: "failed to get GVR"},
		{name: "unknown kind", kind: "UnknownCloneKind", wantError: "failed to get GVR"},
		{name: "kind-only access denied", kind: "ConfigMap", want: core, denied: true, wantError: "requires permissions"},
	} {
		for _, foreach := range []bool{false, true} {
			mode := "direct"
			if foreach {
				mode = "foreach"
			}
			t.Run(mode+"/"+test.name, func(t *testing.T) {
				t.Parallel()
				client, reviews := newGenerateAuthClient(t, func(authorizationv1.SubjectAccessReviewSpec) bool {
					return !test.denied
				})
				discovery := dclient.NewFakeDiscoveryClient(test.registered)
				for _, gvr := range test.registered {
					discovery.AddGVRToGVKMapping(gvr, gvr.GroupVersion().WithKind("Widget"))
				}
				discovery.SetPreferredVersion(v1.Group, test.preferred)
				client = dclient.NewFakeClientWithDisco(client.GetDynamicInterface(), client.GetKubeClient(), discovery)
				pattern := kyvernov1.GeneratePattern{
					ResourceSpec: kyvernov1.ResourceSpec{Namespace: "target"},
					CloneList:    kyvernov1.CloneList{Namespace: "sources", Kinds: []string{test.kind}},
				}
				generation := kyvernov1.Generation{GeneratePattern: pattern}
				if foreach {
					generation = kyvernov1.Generation{
						ForEachGeneration: []kyvernov1.ForEachGeneration{{GeneratePattern: pattern}},
					}
				}
				groups := []string{"policy-authors"}
				checker := NewGenerateFactoryWithGroups(client, &kyvernov1.Rule{Generation: &generation}, "alice", groups, "", logr.Discard())
				_, _, err := checker.Validate(context.Background(), nil)
				if test.wantError != "" {
					assert.ErrorContains(t, err, test.wantError)
				} else {
					assert.NilError(t, err)
				}
				if test.want.Empty() {
					assert.Equal(t, len(*reviews), 0, "unresolved clone kinds must not reach authorization")
					return
				}
				wantVerbs := []string{"get", "create"}
				if !test.denied {
					wantVerbs = append(wantVerbs, "list")
				}
				assert.Equal(t, len(*reviews), len(wantVerbs))
				for i, review := range *reviews {
					assert.Equal(t, review.User, "alice")
					assert.DeepEqual(t, review.Groups, groups)
					namespace := "target"
					if wantVerbs[i] == "list" {
						namespace = "sources"
					}
					assert.DeepEqual(t, review.ResourceAttributes, &authorizationv1.ResourceAttributes{
						Namespace: namespace,
						Verb:      wantVerbs[i],
						Group:     test.want.Group,
						Version:   test.want.Version,
						Resource:  test.want.Resource,
					})
				}
			})
		}
	}
}
