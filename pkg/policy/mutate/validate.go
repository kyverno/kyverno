package mutate

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/utils/api"
	"github.com/pkg/errors"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// Mutate provides implementation to validate 'mutate' rule
type Mutate struct {
	mutation kyvernov1.Mutation
}

// NewMutateFactory returns a new instance of Mutate validation checker
func NewMutateFactory(m kyvernov1.Mutation) *Mutate {
	return &Mutate{
		mutation: m,
	}
}

// Validate validates the 'mutate' rule
func (m *Mutate) Validate() (string, error) {
	if m.hasForEach() {
		if m.hasPatchStrategicMerge() || m.hasPatchesJSON6902() {
			return "foreach", fmt.Errorf("only one of `foreach`, `patchStrategicMerge`, or `patchesJson6902` is allowed")
		}

		return m.validateForEach("", m.mutation.ForEachMutation)
	}

	if m.hasPatchesJSON6902() && m.hasPatchStrategicMerge() {
		return "foreach", fmt.Errorf("only one of `patchStrategicMerge` or `patchesJson6902` is allowed")
	}

	return "", nil
}

func (m *Mutate) validateForEach(tag string, foreach []kyvernov1.ForEachMutation) (string, error) {
	for i, fe := range foreach {
		tag = tag + fmt.Sprintf("foreach[%d]", i)
		if fe.ForEachMutation != nil {
			if fe.Context != nil || fe.AnyAllConditions != nil || fe.PatchesJSON6902 != "" || fe.RawPatchStrategicMerge != nil {
				return tag, fmt.Errorf("a nested foreach cannot contain other declarations")
			}

			return m.validateNestedForEach(tag, fe.ForEachMutation)
		}

		psm := fe.GetPatchStrategicMerge()
		if (fe.PatchesJSON6902 == "" && psm == nil) || (fe.PatchesJSON6902 != "" && psm != nil) {
			return tag, fmt.Errorf("only one of `patchStrategicMerge` or `patchesJson6902` is allowed")
		}
	}

	return "", nil
}

func (m *Mutate) validateNestedForEach(tag string, j *v1.JSON) (string, error) {
	nestedForeach, err := api.DeserializeJSONArray[kyvernov1.ForEachMutation](j)
	if err != nil {
		return tag, errors.Wrapf(err, "invalid foreach syntax")
	}

	return m.validateForEach(tag, nestedForeach)
}

func (m *Mutate) hasForEach() bool {
	return len(m.mutation.ForEachMutation) > 0
}

func (m *Mutate) hasPatchStrategicMerge() bool {
	return m.mutation.GetPatchStrategicMerge() != nil
}

func (m *Mutate) hasPatchesJSON6902() bool {
	return m.mutation.PatchesJSON6902 != ""
}
