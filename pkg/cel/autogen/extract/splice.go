package extract

import (
	"fmt"
	"strconv"
	"strings"
)

// SetAtPath writes value into root at path, using the same dotted/bracket
// grammar walk produces when building an Extracted.Path: map keys are
// separated by ".", and a slice index is written as a "[N]" suffix appended
// directly to the preceding key (e.g. "spec.replicatedJobs[0].template").
//
// Every segment but the last must already exist in root and have the shape
// (object or array) its token implies - SetAtPath never creates missing
// intermediate structure. This is safe because a path produced by
// ExtractPodTemplates was necessarily discovered by walking an
// already-existing tree of that exact shape; callers writing back into a
// deep copy of the same object it was extracted from will always find that
// structure still there.
func SetAtPath(root map[string]any, path string, value map[string]any) error {
	tokens, err := parsePath(path)
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		return fmt.Errorf("empty path")
	}
	var current any = root
	for _, tok := range tokens[:len(tokens)-1] {
		next, err := step(current, tok)
		if err != nil {
			return fmt.Errorf("path %q: %w", path, err)
		}
		current = next
	}
	if err := assign(current, tokens[len(tokens)-1], value); err != nil {
		return fmt.Errorf("path %q: %w", path, err)
	}
	return nil
}

// step descends one token into current, returning the child node.
func step(current any, tok any) (any, error) {
	switch t := tok.(type) {
	case string:
		m, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected an object at %q, got %T", t, current)
		}
		v, ok := m[t]
		if !ok {
			return nil, fmt.Errorf("missing key %q", t)
		}
		return v, nil
	case int:
		s, ok := current.([]any)
		if !ok {
			return nil, fmt.Errorf("expected an array at index %d, got %T", t, current)
		}
		if t < 0 || t >= len(s) {
			return nil, fmt.Errorf("index %d out of range (len %d)", t, len(s))
		}
		return s[t], nil
	default:
		return nil, fmt.Errorf("unexpected path token %T", tok)
	}
}

// assign writes value into current at the final token.
func assign(current any, tok any, value map[string]any) error {
	switch t := tok.(type) {
	case string:
		m, ok := current.(map[string]any)
		if !ok {
			return fmt.Errorf("expected an object at %q, got %T", t, current)
		}
		m[t] = value
	case int:
		s, ok := current.([]any)
		if !ok {
			return fmt.Errorf("expected an array at index %d, got %T", t, current)
		}
		if t < 0 || t >= len(s) {
			return fmt.Errorf("index %d out of range (len %d)", t, len(s))
		}
		s[t] = value
	default:
		return fmt.Errorf("unexpected path token %T", tok)
	}
	return nil
}

// parsePath tokenizes a path produced by walk into a sequence of map keys
// (string) and slice indices (int) - the inverse of walk's joinPath/
// fmt.Sprintf("%s[%d]", ...) construction. For example
// "spec.replicatedJobs[0].template" becomes ["spec", "replicatedJobs", 0,
// "template"].
func parsePath(path string) ([]any, error) {
	var tokens []any
	i := 0
	for i < len(path) {
		j := i
		for j < len(path) && path[j] != '.' && path[j] != '[' {
			j++
		}
		if j > i {
			tokens = append(tokens, path[i:j])
		}
		switch {
		case j == len(path):
			i = j
		case path[j] == '.':
			i = j + 1
		default: // path[j] == '['
			end := strings.IndexByte(path[j:], ']')
			if end < 0 {
				return nil, fmt.Errorf("malformed path %q: unterminated '['", path)
			}
			idxStr := path[j+1 : j+end]
			idx, err := strconv.Atoi(idxStr)
			if err != nil {
				return nil, fmt.Errorf("malformed path %q: invalid index %q", path, idxStr)
			}
			tokens = append(tokens, idx)
			i = j + end + 1
		}
	}
	return tokens, nil
}
