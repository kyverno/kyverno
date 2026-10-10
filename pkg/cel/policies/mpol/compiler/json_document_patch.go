package compiler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	"k8s.io/apiserver/pkg/cel/mutation"
)

func applyDocumentPatch(ctx context.Context, document json.RawMessage, value ref.Val, operations *int) (json.RawMessage, bool, error) {
	list, ok := value.(traits.Lister)
	if !ok {
		return nil, false, fmt.Errorf("expected a list of JSONPatch values")
	}
	envelope, err := json.Marshal(map[string]json.RawMessage{"document": document})
	if err != nil {
		return nil, false, err
	}
	options := jsonpatch.NewApplyOptions()
	options.SupportNegativeIndices = false
	options.AccumulatedCopySizeLimit = MaxJSONDocumentBytes
	for iter := list.Iterator(); iter.HasNext() == types.True; {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		*operations++
		if *operations > MaxJSONPatchOperations {
			return nil, false, fmt.Errorf("policy exceeds %d patch operations", MaxJSONPatchOperations)
		}
		native, err := iter.Next().ConvertToNative(reflect.TypeFor[*mutation.JSONPatchVal]())
		if err != nil {
			return nil, false, err
		}
		op, ok := native.(*mutation.JSONPatchVal)
		if !ok {
			return nil, false, fmt.Errorf("expected JSONPatch value, got %T", native)
		}
		path, err := documentPointer(op.Path)
		if err != nil {
			return nil, false, fmt.Errorf("operation %d path: %w", *operations-1, err)
		}
		translated := map[string]any{"op": op.Op, "path": path}
		switch op.Op {
		case "add", "replace", "test":
			if op.Val == nil {
				return nil, false, fmt.Errorf("operation %d (%s %q) requires a value", *operations-1, op.Op, op.Path)
			}
			remaining := MaxJSONDocumentBytes
			translated["value"], err = jsonNativeValue(op.Val, &remaining)
			if err != nil {
				return nil, false, err
			}
		case "copy", "move":
			translated["from"], err = documentPointer(op.From)
			if err != nil {
				return nil, false, fmt.Errorf("operation %d from: %w", *operations-1, err)
			}
			if err := checkCanonicalIndexes(document, op.From); err != nil {
				return nil, false, fmt.Errorf("operation %d from: %w", *operations-1, err)
			}
			if op.Op == "move" && strings.HasPrefix(op.Path, op.From+"/") {
				return nil, false, fmt.Errorf("operation %d cannot move a value into its descendant", *operations-1)
			}
		case "remove":
			if op.Path == "" {
				return nil, false, fmt.Errorf("operation %d cannot remove the document root", *operations-1)
			}
		default:
			return nil, false, fmt.Errorf("operation %d has unknown op %q", *operations-1, op.Op)
		}
		if err := checkCanonicalIndexes(document, op.Path); err != nil {
			return nil, false, fmt.Errorf("operation %d path: %w", *operations-1, err)
		}
		if op.Op == "test" {
			// RFC 6902 §4.6: test compares JSON values structurally, numbers by value
			// and a missing target location is a failed test, not an error. The
			// library compares compacted bytes (1 != 1.0) and errors on missing
			// containers, so the comparison is done natively.
			equal, err := testDocumentValue(document, op.Path, translated["value"])
			if err != nil {
				return nil, false, fmt.Errorf("operation %d (test %q): %w", *operations-1, op.Path, err)
			}
			if !equal {
				return nil, true, nil
			}
			continue
		}
		raw, err := json.Marshal([]map[string]any{translated})
		if err != nil {
			return nil, false, err
		}
		if len(raw) > MaxJSONDocumentBytes {
			return nil, false, fmt.Errorf("operation %d exceeds %d bytes", *operations-1, MaxJSONDocumentBytes)
		}
		patch, err := jsonpatch.DecodePatch(raw)
		if err != nil {
			return nil, false, err
		}
		// A private envelope gives every JSON root (including scalars and null)
		// an addressable location without depending on library root-type support.
		envelope, err = patch.ApplyWithOptions(envelope, options)
		if errors.Is(err, jsonpatch.ErrTestFailed) {
			return nil, true, nil
		}
		if err != nil {
			return nil, false, fmt.Errorf("operation %d (%s %q): %w", *operations-1, op.Op, op.Path, err)
		}
		var result map[string]json.RawMessage
		if err := json.Unmarshal(envelope, &result); err != nil {
			return nil, false, err
		}
		document, ok = result["document"]
		if !ok {
			return nil, false, fmt.Errorf("operation %d removed the document root", *operations-1)
		}
		if len(document) > MaxJSONDocumentBytes {
			return nil, false, fmt.Errorf("patched JSON document exceeds %d bytes", MaxJSONDocumentBytes)
		}
	}
	return document, false, nil
}

func documentPointer(pointer string) (string, error) {
	if pointer != "" && !strings.HasPrefix(pointer, "/") {
		return "", fmt.Errorf("JSON pointer %q must be empty or start with '/'", pointer)
	}
	for i := 0; i < len(pointer); i++ {
		if pointer[i] == '~' {
			i++
			if i == len(pointer) || (pointer[i] != '0' && pointer[i] != '1') {
				return "", fmt.Errorf("invalid escape in JSON pointer %q", pointer)
			}
		}
	}
	return "/document" + pointer, nil
}

// pointerTokens splits an already-validated JSON pointer into unescaped tokens.
func pointerTokens(pointer string) []string {
	if pointer == "" {
		return nil
	}
	tokens := strings.Split(pointer[1:], "/")
	for i, token := range tokens {
		tokens[i] = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
	}
	return tokens
}

// canonicalIndex reports whether token is an RFC 6901 array index: "0" or a
// non-zero digit followed by digits. Leading zeros, signs and whitespace are
// rejected even though strconv would accept some of them.
func canonicalIndex(token string) bool {
	if token == "" || (len(token) > 1 && token[0] == '0') {
		return false
	}
	for i := 0; i < len(token); i++ {
		if token[i] < '0' || token[i] > '9' {
			return false
		}
	}
	return true
}

// suspiciousIndex reports whether token looks like an integer the patch
// library would accept as an array index even though it is not canonical.
func suspiciousIndex(token string) bool {
	if canonicalIndex(token) || token == "" {
		return false
	}
	if token[0] == '+' || token[0] == '-' {
		token = token[1:]
	}
	if token == "" {
		return false
	}
	for i := 0; i < len(token); i++ {
		if token[i] < '0' || token[i] > '9' {
			return false
		}
	}
	return true
}

func decodeLenient(document json.RawMessage) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

// checkCanonicalIndexes rejects pointers that address an array element with a
// non-canonical index such as "01", "+1" or "-0". The document is only decoded
// when a suspicious token is present, so ordinary operations pay nothing.
func checkCanonicalIndexes(document json.RawMessage, pointer string) error {
	tokens := pointerTokens(pointer)
	suspicious := false
	for _, token := range tokens {
		if suspiciousIndex(token) {
			suspicious = true
			break
		}
	}
	if !suspicious {
		return nil
	}
	current, err := decodeLenient(document)
	if err != nil {
		return err
	}
	for _, token := range tokens {
		switch container := current.(type) {
		case []any:
			if suspiciousIndex(token) {
				return fmt.Errorf("JSON pointer %q uses non-canonical array index %q", pointer, token)
			}
			if !canonicalIndex(token) {
				return nil
			}
			index, err := strconv.Atoi(token)
			if err != nil || index >= len(container) {
				return nil
			}
			current = container[index]
		case map[string]any:
			next, ok := container[token]
			if !ok {
				return nil
			}
			current = next
		default:
			return nil
		}
	}
	return nil
}

// testDocumentValue resolves pointer in document and compares the value found
// with expected using JSON equality. A missing location yields false.
func testDocumentValue(document json.RawMessage, pointer string, expected any) (bool, error) {
	current, err := decodeLenient(document)
	if err != nil {
		return false, err
	}
	for _, token := range pointerTokens(pointer) {
		switch container := current.(type) {
		case []any:
			if !canonicalIndex(token) {
				return false, nil
			}
			index, err := strconv.Atoi(token)
			if err != nil || index >= len(container) {
				return false, nil
			}
			current = container[index]
		case map[string]any:
			next, ok := container[token]
			if !ok {
				return false, nil
			}
			current = next
		default:
			return false, nil
		}
	}
	return jsonEqual(current, expected), nil
}

func jsonRat(value any) (*big.Rat, bool) {
	switch value := value.(type) {
	case json.Number:
		return new(big.Rat).SetString(string(value))
	case int64:
		return new(big.Rat).SetInt64(value), true
	case uint64:
		return new(big.Rat).SetUint64(value), true
	case float64:
		// A CEL double stands for the decimal literal it was written as, so use
		// its shortest round-trip form: 0.1 is 1/10, not float64(0.1)'s binary
		// expansion, while integers stay exact.
		if math.IsInf(value, 0) || math.IsNaN(value) {
			return nil, false
		}
		return new(big.Rat).SetString(strconv.FormatFloat(value, 'g', -1, 64))
	}
	return nil, false
}

// jsonEqual implements RFC 6902 test equality: numbers compare by numeric
// value regardless of representation, objects by key set and member equality,
// arrays by order, and null only equals null.
func jsonEqual(actual, expected any) bool {
	if a, ok := jsonRat(actual); ok {
		b, ok := jsonRat(expected)
		return ok && a.Cmp(b) == 0
	}
	switch actual := actual.(type) {
	case nil:
		return expected == nil
	case bool:
		expected, ok := expected.(bool)
		return ok && actual == expected
	case string:
		expected, ok := expected.(string)
		return ok && actual == expected
	case []any:
		expected, ok := expected.([]any)
		if !ok || len(actual) != len(expected) {
			return false
		}
		for i := range actual {
			if !jsonEqual(actual[i], expected[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		expected, ok := expected.(map[string]any)
		if !ok || len(actual) != len(expected) {
			return false
		}
		for key, value := range actual {
			other, ok := expected[key]
			if !ok || !jsonEqual(value, other) {
				return false
			}
		}
		return true
	}
	return false
}
