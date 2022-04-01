package json

import (
	"encoding/json"

	jsonpatch "github.com/evanphx/json-patch"
)

type Patch struct {
	Path  string      `json:"path"`
	Op    string      `json:"op"`
	Value interface{} `json:"value,omitempty"`
}

func NewPatch(path, op string, value interface{}) Patch {
	return Patch{path, op, value}
}

func (p *Patch) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *Patch) ToPatchBytes() ([]byte, error) {
	if patch, err := json.Marshal(p); err != nil {
		return nil, err
	} else {
		return JoinPatches(patch), nil
	}
}

func MarshalPatch(path, op string, value interface{}) ([]byte, error) {
	p := NewPatch(path, op, value)
	return p.Marshal()
}

func CheckPatch(patch []byte) error {
	_, err := jsonpatch.DecodePatch([]byte("[" + string(patch) + "]"))
	return err
}

func UnmarshalPatch(patch []byte) (*Patch, error) {
	var p Patch
	if err := json.Unmarshal(patch, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
