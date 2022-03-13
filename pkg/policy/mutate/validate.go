package mutate

import (
	"fmt"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
)

// Mutate provides implementation to validate 'mutate' rule
type Mutate struct {
	mutation kyverno.Mutation
}

//NewMutateFactory returns a new instance of Mutate validation checker
func NewMutateFactory(m kyverno.Mutation) *Mutate {
	return &Mutate{
		mutation: m,
	}
}

//Validate validates the 'mutate' rule
func (m *Mutate) Validate() (string, error) {
	if m.hasForEach() {
		return m.validateForEach()
	}

	if m.hasPatchesJSON6902() && m.hasPatchStrategicMerge() {
		return "foreach", fmt.Errorf("only one of `patchStrategicMerge` or `patchesJson6902` is allowed")
	}

	return "", nil
}

func (m *Mutate) validateForEach() (string, error) {
	if m.hasPatchStrategicMerge() || m.hasPatchesJSON6902() {
		return "foreach", fmt.Errorf("only one of `foreach`, `patchStrategicMerge`, or `patchesJson6902` is allowed")
	}

	for i, fe := range m.mutation.ForEachMutation {
		psm := fe.GetPatchStrategicMerge()
		if (fe.PatchesJSON6902 == "" && psm == nil) || (fe.PatchesJSON6902 != "" && psm != nil) {
			return fmt.Sprintf("foreach[%d]", i), fmt.Errorf("only one of `patchStrategicMerge` or `patchesJson6902` is allowed")
		}
	}

	return "", nil
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
