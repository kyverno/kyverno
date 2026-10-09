package processor

import (
	"io"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/store"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestMockControllerCloneListResourceScope(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name             string
		kind             string
		fixture          bool
		fixtureNamespace string
		wantError        string
	}{
		{name: "empty namespaced list", kind: "Secret"},
		{name: "namespaced fixture omits namespace", kind: "Secret", fixture: true},
		{name: "cluster scoped kind", kind: "Namespace", wantError: "must be namespaced"},
		{name: "cluster scoped fixture has namespace", kind: "Namespace", fixture: true, fixtureNamespace: "target", wantError: "must be namespaced"},
		{name: "unknown kind", kind: "Unknown", wantError: "source scope"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			trigger := &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "v1", "kind": "ConfigMap",
				"metadata": map[string]interface{}{"name": "trigger", "namespace": "target"},
			}}
			objects := []runtime.Object{trigger}
			if test.fixture {
				fixture := &unstructured.Unstructured{Object: map[string]interface{}{
					"apiVersion": "v1", "kind": test.kind,
					"metadata": map[string]interface{}{"name": "fixture"},
				}}
				fixture.SetNamespace(test.fixtureNamespace)
				objects = append(objects, fixture)
			}
			policy := &kyvernov1.Policy{
				TypeMeta:   metav1.TypeMeta{APIVersion: "kyverno.io/v1", Kind: "Policy"},
				ObjectMeta: metav1.ObjectMeta{Name: "clone-list", Namespace: "target"},
				Spec: kyvernov1.Spec{Rules: []kyvernov1.Rule{{
					Name: "clone-list",
					MatchResources: kyvernov1.MatchResources{Any: kyvernov1.ResourceFilters{{
						ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{"ConfigMap"}},
					}}},
					Generation: &kyvernov1.Generation{GeneratePattern: kyvernov1.GeneratePattern{
						ResourceSpec: kyvernov1.ResourceSpec{Namespace: "target"},
						CloneList:    kyvernov1.CloneList{Namespace: "target", Kinds: []string{"v1/" + test.kind}},
					}},
				}}},
			}
			listKinds := map[schema.GroupVersionResource]string{
				{Version: "v1", Resource: strings.ToLower(test.kind) + "s"}: test.kind + "List",
			}
			controller, err := initializeMockController(io.Discard, &store.Store{}, listKinds, objects)
			require.NoError(t, err)
			cfg := config.NewDefaultConfiguration(false)
			policyContext, err := engine.NewPolicyContext(jmespath.New(cfg), *trigger, kyvernov1.Create, nil, cfg)
			require.NoError(t, err)
			policyContext = policyContext.WithPolicy(policy)
			generated, err := controller.ApplyGeneratePolicy(logr.Discard(), policyContext, []string{"clone-list"})
			if test.wantError != "" {
				require.ErrorContains(t, err, test.wantError)
				return
			}
			require.NoError(t, err)
			require.Contains(t, generated, "clone-list", "the empty list must still execute the rule")
			require.Empty(t, generated["clone-list"])
		})
	}
}
