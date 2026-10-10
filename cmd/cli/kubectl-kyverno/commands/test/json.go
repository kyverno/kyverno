package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/go-git/go-billy/v5"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/payload"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
)

func compareJSONDocument(actual *payload.Document, fs billy.Filesystem, path string) (bool, string, error) {
	if actual == nil {
		return false, "", fmt.Errorf("no successfully patched JSON document is available; omit patchedResources for an error result")
	}
	content, err := common.ReadFile(fs, path)
	if err != nil {
		return false, "", err
	}
	expected, err := payload.ParseDocument(path, content)
	if err != nil {
		return false, "", fmt.Errorf("invalid expected JSON document: %w", err)
	}
	decode := func(raw json.RawMessage) (any, error) {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		var value any
		err := decoder.Decode(&value)
		return value, err
	}
	left, err := decode(expected.Raw)
	if err != nil {
		return false, "", err
	}
	right, err := decode(actual.Raw)
	if err != nil {
		return false, "", err
	}
	return reflect.DeepEqual(left, right), fmt.Sprintf("expected: %s\nactual:   %s", expected.Raw, actual.Raw), nil
}
