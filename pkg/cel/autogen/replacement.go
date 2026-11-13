package autogen

import (
	"bytes"
)

type Replacement struct {
	From string
	To   string
}

// Apply rewrites the configured field paths in the given data,
// replacing both the "object." and "oldObject." prefixes (including optional traversal ".?" and bracket selectors),
// while skipping CEL string literals.
func (r *Replacement) Apply(data []byte) []byte {
	roots := [][]byte{[]byte("object"), []byte("oldObject")}
	return rewriteField(data, roots, r.From, r.To)
}

// Apply sequentially applies a list of replacements to the given data.
func Apply(data []byte, replacements ...Replacement) []byte {
	for _, replacement := range replacements {
		data = replacement.Apply(data)
	}
	return data
}

// ApplyCEL sequentially applies a list of replacements to raw CEL strings,
// skipping replacements inside string literals.
func ApplyCEL(data []byte, replacements ...Replacement) []byte {
	for _, replacement := range replacements {
		data = replacement.Apply(data)
	}
	return data
}

// ApplyMutatingCEL applies pod-to-template expression conversions for MutatingPolicy.
// It is syntax-aware and skips CEL string literals, rewrites bracket and dot selectors
// for spec, metadata.labels, and metadata.annotations, and preserves controller metadata fields.
func ApplyMutatingCEL(data []byte, config string) []byte {
	specRepl := "spec.template.spec"
	metaRepl := "spec.template.metadata"
	if config == "cronjobs" {
		specRepl = "spec.jobTemplate.spec.template.spec"
		metaRepl = "spec.jobTemplate.spec.template.metadata"
	}
	roots := [][]byte{[]byte("object"), []byte("oldObject"), []byte("Object")}
	return rewriteMutatingCEL(data, roots, specRepl, metaRepl)
}

// rewriteField replaces occurrences of `roots` + `fromField` with `roots` + `toField`.
// It preserves protected metadata fields (.name, .namespace) and already rewritten spec paths (.template, .jobTemplate).
func rewriteField(data []byte, roots [][]byte, fromField, toField string) []byte {
	return transformCEL(data, func(data []byte, i int) (int, []byte, bool) {
		root, ok := matchRoot(data, i, roots)
		if !ok {
			return 0, nil, false
		}
		rest := data[i+len(root):]
		matchLen, isOptional, matched := scanFieldSelector(rest, fromField)
		if !matched {
			return 0, nil, false
		}
		after := rest[matchLen:]
		if fromField == "metadata" && isProtectedMetadata(after) {
			return len(root) + matchLen, data[i : i+len(root)+matchLen], true
		}
		if fromField == "spec" && isAlreadyRewrittenSpec(after) {
			return len(root) + matchLen, data[i : i+len(root)+matchLen], true
		}
		var repl bytes.Buffer
		repl.Write(root)
		if isOptional {
			repl.WriteString(".?")
		} else {
			repl.WriteString(".")
		}
		repl.WriteString(toField)
		return len(root) + matchLen, repl.Bytes(), true
	})
}

// rewriteMutatingCEL performs mutating policy pod-to-template rewrites:
//  1. metadata.labels and metadata.annotations -> spec.template.metadata.labels/annotations
//  2. spec -> spec.template.spec
func rewriteMutatingCEL(data []byte, roots [][]byte, specRepl, metaRepl string) []byte {
	return transformCEL(data, func(data []byte, i int) (int, []byte, bool) {
		root, ok := matchRoot(data, i, roots)
		if !ok {
			return 0, nil, false
		}
		rest := data[i+len(root):]

		// 1. Check for metadata.labels or metadata.annotations
		if metaLen, optMeta, okMeta := scanFieldSelector(rest, "metadata"); okMeta {
			afterMeta := rest[metaLen:]
			subLen, optSub, okSub := scanFieldSelector(afterMeta, "labels")
			subName := "labels"
			if !okSub {
				subLen, optSub, okSub = scanFieldSelector(afterMeta, "annotations")
				subName = "annotations"
			}
			if okSub {
				var repl bytes.Buffer
				repl.Write(root)
				if optMeta {
					repl.WriteString(".?")
				} else {
					repl.WriteString(".")
				}
				repl.WriteString(metaRepl)
				if optSub {
					repl.WriteString(".?")
				} else {
					repl.WriteString(".")
				}
				repl.WriteString(subName)
				return len(root) + metaLen + subLen, repl.Bytes(), true
			}
		}

		// 2. Check for spec
		if specLen, optSpec, okSpec := scanFieldSelector(rest, "spec"); okSpec {
			afterSpec := rest[specLen:]
			if isAlreadyRewrittenSpec(afterSpec) {
				return len(root) + specLen, data[i : i+len(root)+specLen], true
			}
			var repl bytes.Buffer
			repl.Write(root)
			if optSpec {
				repl.WriteString(".?")
			} else {
				repl.WriteString(".")
			}
			repl.WriteString(specRepl)
			return len(root) + specLen, repl.Bytes(), true
		}

		return 0, nil, false
	})
}

// transformCEL scans through data, skipping CEL comments and string literals,
// and invokes transform at valid token boundaries.
func transformCEL(data []byte, transform func(data []byte, i int) (int, []byte, bool)) []byte {
	size := len(data)
	var buf bytes.Buffer
	buf.Grow(size)

	inString := false
	var stringQuote string
	escaped := false

	for i := 0; i < len(data); {
		if !inString {
			// Skip CEL single-line comments
			if bytes.HasPrefix(data[i:], []byte("//")) {
				newlineIdx := bytes.IndexByte(data[i:], '\n')
				if newlineIdx == -1 {
					buf.Write(data[i:])
					break
				}
				buf.Write(data[i : i+newlineIdx+1])
				i += newlineIdx + 1
				continue
			}

			// Skip CEL block comments (/* ... */)
			if bytes.HasPrefix(data[i:], []byte("/*")) {
				endCommentIdx := bytes.Index(data[i+2:], []byte("*/"))
				if endCommentIdx == -1 {
					buf.Write(data[i:])
					break
				}
				buf.Write(data[i : i+2+endCommentIdx+2])
				i += 2 + endCommentIdx + 2
				continue
			}

			// Try transformation at current position
			if advance, repl, ok := transform(data, i); ok {
				buf.Write(repl)
				i += advance
				continue
			}

			// Check for string start (check raw triple quotes before raw single quotes)
			if bytes.HasPrefix(data[i:], []byte("r'''")) || bytes.HasPrefix(data[i:], []byte("R'''")) {
				inString = true
				stringQuote = "r'''"
				buf.Write(data[i : i+4])
				i += 4
				continue
			} else if bytes.HasPrefix(data[i:], []byte("r\"\"\"")) || bytes.HasPrefix(data[i:], []byte("R\"\"\"")) {
				inString = true
				stringQuote = "r\"\"\""
				buf.Write(data[i : i+4])
				i += 4
				continue
			} else if bytes.HasPrefix(data[i:], []byte("'''")) {
				inString = true
				stringQuote = "'''"
				buf.Write(data[i : i+3])
				i += 3
				continue
			} else if bytes.HasPrefix(data[i:], []byte("\"\"\"")) {
				inString = true
				stringQuote = "\"\"\""
				buf.Write(data[i : i+3])
				i += 3
				continue
			} else if bytes.HasPrefix(data[i:], []byte("r'")) || bytes.HasPrefix(data[i:], []byte("R'")) {
				inString = true
				stringQuote = "r'"
				buf.Write(data[i : i+2])
				i += 2
				continue
			} else if bytes.HasPrefix(data[i:], []byte("r\"")) || bytes.HasPrefix(data[i:], []byte("R\"")) {
				inString = true
				stringQuote = "r\""
				buf.Write(data[i : i+2])
				i += 2
				continue
			} else if data[i] == '\'' {
				inString = true
				stringQuote = "'"
				buf.WriteByte(data[i])
				i++
				continue
			} else if data[i] == '"' {
				inString = true
				stringQuote = "\""
				buf.WriteByte(data[i])
				i++
				continue
			} else if data[i] == '`' {
				inString = true
				stringQuote = "`"
				buf.WriteByte(data[i])
				i++
				continue
			}
		} else {
			// Inside string literal
			if !escaped {
				if (stringQuote == "r'''" || stringQuote == "'''") && bytes.HasPrefix(data[i:], []byte("'''")) {
					inString = false
					buf.Write(data[i : i+3])
					i += 3
					continue
				} else if (stringQuote == "r\"\"\"" || stringQuote == "\"\"\"") && bytes.HasPrefix(data[i:], []byte("\"\"\"")) {
					inString = false
					buf.Write(data[i : i+3])
					i += 3
					continue
				} else if (stringQuote == "r'" || stringQuote == "'") && data[i] == '\'' {
					inString = false
				} else if (stringQuote == "r\"" || stringQuote == "\"") && data[i] == '"' {
					inString = false
				} else if stringQuote == "`" && data[i] == '`' {
					inString = false
				}
			}
			if data[i] == '\\' && stringQuote != "r'" && stringQuote != "r\"" && stringQuote != "r'''" && stringQuote != "r\"\"\"" && stringQuote != "`" {
				escaped = !escaped
			} else {
				escaped = false
			}
		}

		buf.WriteByte(data[i])
		i++
	}
	return buf.Bytes()
}

// matchRoot checks whether data[i:] begins with one of roots at a valid root boundary.
func matchRoot(data []byte, i int, roots [][]byte) ([]byte, bool) {
	if !isValidBoundary(data, i) {
		return nil, false
	}
	for _, root := range roots {
		if bytes.HasPrefix(data[i:], root) {
			after := i + len(root)
			if after == len(data) || !isIdentifierByte(data[after]) {
				return root, true
			}
		}
	}
	return nil, false
}

// scanFieldSelector reports whether data begins with a dot or bracket selector for fieldName.
// It tolerates optional traversal (.? or [?...]) and arbitrary whitespace.
func scanFieldSelector(data []byte, field string) (int, bool, bool) {
	if n, opt, ok := scanDotSelector(data, field); ok {
		return n, opt, true
	}
	if n, opt, ok := scanBracketSelector(data, field); ok {
		return n, opt, true
	}
	return 0, false, false
}

// scanDotSelector checks if data starts with "." or ".?" followed by fieldName.
// It ensures fieldName ends on a non-identifier boundary and tolerates optional whitespace around the dot.
func scanDotSelector(data []byte, field string) (int, bool, bool) {
	if len(data) == 0 {
		return 0, false, false
	}
	idx := 0
	for idx < len(data) && isWhitespaceByte(data[idx]) {
		idx++
	}
	if idx >= len(data) || data[idx] != '.' {
		return 0, false, false
	}
	idx++ // consume '.'

	isOptional := false
	if idx < len(data) && data[idx] == '?' {
		isOptional = true
		idx++
	}
	for idx < len(data) && isWhitespaceByte(data[idx]) {
		idx++
	}
	if len(data[idx:]) < len(field) || !bytes.Equal(data[idx:idx+len(field)], []byte(field)) {
		return 0, false, false
	}
	idx += len(field)
	if idx < len(data) && isIdentifierByte(data[idx]) {
		return 0, false, false
	}
	return idx, isOptional, true
}

// scanBracketSelector checks if data starts with bracket access for fieldName:
//
//	[whitespace] [?] [whitespace] quote fieldName quote [whitespace] ]
//
// It tolerates arbitrary whitespace and supports single ('...'), double ("..."),
// JSON-escaped (\"...\"), and raw (r'...', r"...") quotes.
func scanBracketSelector(data []byte, field string) (int, bool, bool) {
	if len(data) == 0 {
		return 0, false, false
	}
	idx := 0
	for idx < len(data) && isWhitespaceByte(data[idx]) {
		idx++
	}
	if idx >= len(data) || data[idx] != '[' {
		return 0, false, false
	}
	idx++ // consume '['

	for idx < len(data) && isWhitespaceByte(data[idx]) {
		idx++
	}
	isOptional := false
	if idx < len(data) && data[idx] == '?' {
		isOptional = true
		idx++
	}
	for idx < len(data) && isWhitespaceByte(data[idx]) {
		idx++
	}
	if idx >= len(data) {
		return 0, false, false
	}

	var quote []byte
	if data[idx] == '\'' {
		quote = []byte("'")
		idx++
	} else if data[idx] == '"' {
		quote = []byte("\"")
		idx++
	} else if bytes.HasPrefix(data[idx:], []byte(`\"`)) {
		quote = []byte(`\"`)
		idx += 2
	} else if (data[idx] == 'r' || data[idx] == 'R') && idx+1 < len(data) {
		if data[idx+1] == '\'' {
			quote = []byte("'")
			idx += 2
		} else if data[idx+1] == '"' {
			quote = []byte("\"")
			idx += 2
		} else {
			return 0, false, false
		}
	} else {
		return 0, false, false
	}

	if len(data[idx:]) < len(field) || !bytes.Equal(data[idx:idx+len(field)], []byte(field)) {
		return 0, false, false
	}
	idx += len(field)

	if !bytes.HasPrefix(data[idx:], quote) {
		return 0, false, false
	}
	idx += len(quote)

	for idx < len(data) && isWhitespaceByte(data[idx]) {
		idx++
	}
	if idx >= len(data) || data[idx] != ']' {
		return 0, false, false
	}
	idx++ // consume ']'

	return idx, isOptional, true
}

// isProtectedMetadata reports whether rest begins with a selector for a protected field
// ('name' or 'namespace'), tolerating optional '?' and arbitrary whitespace.
func isProtectedMetadata(rest []byte) bool {
	if _, _, ok := scanFieldSelector(rest, "name"); ok {
		return true
	}
	if _, _, ok := scanFieldSelector(rest, "namespace"); ok {
		return true
	}
	return false
}

// isAlreadyRewrittenSpec reports whether rest begins with .template or .jobTemplate,
// preventing redundant double-rewriting on spec paths.
func isAlreadyRewrittenSpec(rest []byte) bool {
	if _, _, ok := scanFieldSelector(rest, "template"); ok {
		return true
	}
	if _, _, ok := scanFieldSelector(rest, "jobTemplate"); ok {
		return true
	}
	return false
}

// isValidBoundary reports whether data[i:] starts at a valid CEL root boundary.
func isValidBoundary(data []byte, i int) bool {
	if i == 0 {
		return true
	}
	if data[i-1] == '.' {
		if i >= 8 && bytes.Equal(data[i-8:i], []byte("request.")) {
			return i == 8 || (!isIdentifierByte(data[i-9]) && data[i-9] != '.')
		}
		return false
	}
	if data[i-1] == '?' && i >= 2 && data[i-2] == '.' {
		if i >= 9 && bytes.Equal(data[i-9:i], []byte("request.?")) {
			return i == 9 || (!isIdentifierByte(data[i-10]) && data[i-10] != '.')
		}
		return false
	}
	return !isIdentifierByte(data[i-1])
}

// isWhitespaceByte reports whether b is an ASCII whitespace character permitted in CEL syntax.
func isWhitespaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n'
}

// isIdentifierByte reports whether b can be part of a CEL identifier segment.
func isIdentifierByte(b byte) bool {
	return b == '_' ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
}
