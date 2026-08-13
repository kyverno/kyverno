package v1

import (
	"encoding/json"

	runtime "k8s.io/apimachinery/pkg/runtime"
)

// Any can be any type.
// +k8s:deepcopy-gen=false
type Any struct {
	// Value contains the value of the Any object.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	// +optional
	Value any `json:",inline"`
}

func (in *Any) DeepCopyInto(out *Any) {
	*out = *in
	out.Value = runtime.DeepCopyJSONValue(in.Value)
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
