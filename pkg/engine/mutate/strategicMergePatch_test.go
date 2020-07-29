package mutate

import (
	"bytes"
	"encoding/json"
	"testing"

	assertnew "github.com/stretchr/testify/assert"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type buffer struct {
	*bytes.Buffer
}

func (buff buffer) UnmarshalJSON(b []byte) error {
	buff.Reset()
	buff.Write(b)
	return nil
}

func (buff buffer) MarshalJSON() ([]byte, error) {
	return buff.Bytes(), nil
}

// note:         emptyDir: {} is removed from patch

func TestMergePatch(t *testing.T) {

	// out
	out, err := strategicMergePatchfilter(string(baseBytes), string(overlayBytes))
	assert.NilError(t, err)

	// expect
	var expectUnstr unstructured.Unstructured
	err = json.Unmarshal(expectBytes, &expectUnstr)
	assert.NilError(t, err)

	expectString, err := json.Marshal(expectUnstr.Object)
	assert.NilError(t, err)

	if !assertnew.Equal(t, string(expectString), string(out)) {
		t.FailNow()
	}
}
