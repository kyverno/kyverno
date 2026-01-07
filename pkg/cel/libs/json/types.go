package json

import (
	"encoding/json"

	"github.com/google/cel-go/common/types"
)

var JsonType = types.NewOpaqueType("json.Json")

type JsonIface interface {
	Unmarshal([]byte) (any, error)
}

type Json struct {
	JsonIface
}

type JsonImpl struct{}

func (j *JsonImpl) Unmarshal(content []byte) (any, error) {
	var v any
	if err := json.Unmarshal(content, &v); err != nil {
		return nil, err
	}
	return v, nil
}
