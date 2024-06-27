package kyverno

import (
	"encoding/json"

	"github.com/jinzhu/copier"
)

type Value any

// Any can be any type.
// +k8s:deepcopy-gen=false
type Any struct {
	// Value contains the value of the Any object.
	// +optional
	Value `json:"-"`
}

func ToAny(in any) *Any {
	var new *Any
	if in != nil {
		new = &Any{in}
	}
	return new
}

func FromAny(in *Any) any {
	if in == nil {
		return nil
	}
	return in.Value
}

func (in *Any) DeepCopyInto(out *Any) {
	if err := copier.Copy(out, in); err != nil {
		panic("deep copy failed")
	}
}

func (in *Any) DeepCopy() *Any {
	if in == nil {
		return nil
	}
	out := new(Any)
	in.DeepCopyInto(out)
	return out
}

func (a *Any) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.Value)
}

func (a *Any) UnmarshalJSON(data []byte) error {
	var v any
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}
	a.Value = v
	return nil
}
