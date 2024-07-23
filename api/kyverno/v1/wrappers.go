package v1

import (
	"encoding/json"

	"github.com/jinzhu/copier"
)

// ForEachValidationWrapper contains a list of ForEach descriptors.
// +k8s:deepcopy-gen=false
type ForEachValidationWrapper struct {
	// Item is a descriptor on how to iterate over the list of items.
	// +optional
	Items []ForEachValidation `json:"-"`
}

func (in *ForEachValidationWrapper) DeepCopyInto(out *ForEachValidationWrapper) {
	if err := copier.Copy(out, in); err != nil {
		panic("deep copy failed")
	}
}

func (in *ForEachValidationWrapper) DeepCopy() *ForEachValidationWrapper {
	if in == nil {
		return nil
	}
	out := new(ForEachValidationWrapper)
	in.DeepCopyInto(out)
	return out
}

func (a *ForEachValidationWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.Items)
}

func (a *ForEachValidationWrapper) UnmarshalJSON(data []byte) error {
	var res []ForEachValidation
	if err := json.Unmarshal(data, &res); err != nil {
		return err
	}
	a.Items = res
	return nil
}

// ForEachMutationWrapper contains a list of ForEach descriptors.
// +k8s:deepcopy-gen=false
type ForEachMutationWrapper struct {
	// Item is a descriptor on how to iterate over the list of items.
	// +optional
	Items []ForEachMutation `json:"-"`
}

func (in *ForEachMutationWrapper) DeepCopyInto(out *ForEachMutationWrapper) {
	if err := copier.Copy(out, in); err != nil {
		panic("deep copy failed")
	}
}

func (in *ForEachMutationWrapper) DeepCopy() *ForEachMutationWrapper {
	if in == nil {
		return nil
	}
	out := new(ForEachMutationWrapper)
	in.DeepCopyInto(out)
	return out
}

func (a *ForEachMutationWrapper) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.Items)
}

func (a *ForEachMutationWrapper) UnmarshalJSON(data []byte) error {
	var res []ForEachMutation
	if err := json.Unmarshal(data, &res); err != nil {
		return err
	}
	a.Items = res
	return nil
}
