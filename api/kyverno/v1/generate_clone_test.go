package v1

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestGenerateCloneSourceNamespaceValidation(t *testing.T) {
	t.Parallel()
	for _, foreach := range []bool{false, true} {
		for _, tt := range []struct {
			name, namespace     string
			namespaced, wantErr bool
		}{
			{"same namespace", "tenant", true, false},
			{"foreign namespace", "other", true, true},
			{"omitted namespace", "", true, true},
			{"variable namespace", "{{request.object.metadata.labels.source}}", true, true},
			{"cluster policy", "other", false, false},
		} {
			t.Run(fmt.Sprintf("foreach=%t/%s", foreach, tt.name), func(t *testing.T) {
				t.Parallel()
				pattern := GeneratePattern{
					ResourceSpec: ResourceSpec{APIVersion: "v1", Kind: "ConfigMap", Namespace: "tenant", Name: "copy"},
					Clone:        CloneFrom{Name: "source", Namespace: tt.namespace},
				}
				generation := Generation{GeneratePattern: pattern}
				if foreach {
					generation = Generation{ForEachGeneration: []ForEachGeneration{{List: "request.object.spec.sources", GeneratePattern: pattern}}}
				}
				_, errs := generation.Validate(field.NewPath("generate"), tt.namespaced, "tenant", sets.New[string]())
				if tt.wantErr {
					require.ErrorContains(t, errs.ToAggregate(), "a namespaced policy cannot clone resources from other namespaces")
				} else {
					require.Empty(t, errs)
				}
			})
		}
	}
}
