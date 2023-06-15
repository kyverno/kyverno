package policy

import (
	"fmt"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_fetchUniqueKinds(t *testing.T) {

	tests := []struct {
		name string
		rule kyverno.Rule
		want []string
	}{
		{
			name: "Unique MatchResource kinds",
			rule: kyverno.Rule{
				MatchResources: kyverno.MatchResources{
					ResourceDescription: kyverno.ResourceDescription{
						Kinds: []string{"kind1", "kind2"},
					},
				},
			},
			want: []string{"kind1", "kind2"},
		},

		{
			name: "Any with same kind are valid",
			rule: kyverno.Rule{
				MatchResources: kyverno.MatchResources{
					Any: []kyverno.ResourceFilter{
						{
							ResourceDescription: kyverno.ResourceDescription{
								Kinds: []string{"kind1", "kind2"},
							},
						},
						{
							ResourceDescription: kyverno.ResourceDescription{
								Kinds: []string{"kind1", "kind3"},
							},
						},
					},
				},
			},
			want: []string{"kind1", "kind2", "kind3"},
		},
		{
			name: "Match with All and Any kind",
			rule: kyverno.Rule{
				MatchResources: kyverno.MatchResources{
					All: []kyverno.ResourceFilter{
						{
							ResourceDescription: kyverno.ResourceDescription{
								Kinds: []string{"kind1"},
							},
						},
					},
					Any: []kyverno.ResourceFilter{
						{
							ResourceDescription: kyverno.ResourceDescription{
								Kinds: []string{"kind1", "kind2"},
							},
						},
					},
				},
			},
			want: []string{"kind1", "kind2"},
		},
		{
			name: "Match with different All and Any kind",
			rule: kyverno.Rule{
				MatchResources: kyverno.MatchResources{
					All: []kyverno.ResourceFilter{
						{
							ResourceDescription: kyverno.ResourceDescription{
								Kinds: []string{"kind4", "kind5"},
							},
						},
					},
					Any: []kyverno.ResourceFilter{
						{
							ResourceDescription: kyverno.ResourceDescription{
								Kinds: []string{"kind1", "kind2"},
							},
						},
					},
				},
			},
			want: []string{"kind1", "kind2", "kind4", "kind5"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kinds := fetchUniqueKinds(tt.rule)
			for _, want := range tt.want {
				if !kubeutils.ContainsKind(kinds, want) {
					assert.Error(t, fmt.Errorf("%s fails, expected %s", tt.name, want), "")
				}
			}
		})
	}
}

func Test_convertlist(t *testing.T) {
	tests := []struct {
		name   string
		ulists []unstructured.Unstructured
		want   []*unstructured.Unstructured
	}{
		{
			name: "Convert list",
			ulists: []unstructured.Unstructured{
				{
					Object: map[string]interface{}{
						"kind": "kind1",
					},
				},
				{
					Object: map[string]interface{}{
						"namespace": "ns-1",
					},
				},
			},
			want: []*unstructured.Unstructured{
				{
					Object: map[string]interface{}{
						"kind": "kind1",
					},
				},
				{
					Object: map[string]interface{}{
						"namespace": "ns-1",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.DeepEqual(t, convertlist(tt.ulists), tt.want)
		})
	}
}
