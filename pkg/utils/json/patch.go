package json

import (
	"encoding/json"

	jsonpatch "github.com/evanphx/json-patch/v5"
)

type PatchOperation struct {
	Path  string      `json:"path"`
	Op    string      `json:"op"`
	Value interface{} `json:"value,omitempty"`
}

func NewPatchOperation(path, op string, value interface{}) PatchOperation {
	return PatchOperation{path, op, value}
}

func (p *PatchOperation) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *PatchOperation) ToPatchBytes() ([]byte, error) {
	if patch, err := json.Marshal(p); err != nil {
		return nil, err
	} else {
		return JoinPatches(patch), nil
	}
}

func MarshalPatchOperation(path, op string, value interface{}) ([]byte, error) {
	p := NewPatchOperation(path, op, value)
	return p.Marshal()
}

func CheckPatch(patch []byte) error {
	_, err := jsonpatch.DecodePatch([]byte("[" + string(patch) + "]"))
	return err
}

func UnmarshalPatchOperation(patch []byte) (*PatchOperation, error) {
	var p PatchOperation
	if err := json.Unmarshal(patch, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
