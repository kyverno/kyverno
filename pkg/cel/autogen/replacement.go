package autogen

import (
	"bytes"
)

// protectedSuffixes lists field paths that must remain anchored to the
// workload object's own metadata and must never be rewritten into a pod
// template path. For example, `object.metadata.namespace` and
// `object.metadata.name` must stay as-is because pod templates (e.g. on
// Deployments) usually do not carry those fields, which would otherwise
// break match conditions and message expressions.
var protectedSuffixes = [][]byte{
	[]byte(".namespace"),
	[]byte(".name"),
	[]byte("['name']"),
	[]byte("[\"name\"]"),
	[]byte("[\\\"name\\\"]"),
	[]byte("['namespace']"),
	[]byte("[\"namespace\"]"),
	[]byte("[\\\"namespace\\\"]"), // Handle JSON-escaped namespace paths
}

type Replacement struct {
	From string
	To   string
}

// Apply rewrites the configured field paths in the given data,
// replacing both the "object." and "oldObject." prefixes.
func (r *Replacement) Apply(data []byte) []byte {
	data = replace(data, []byte("object."+r.From), []byte("object."+r.To))
	data = replace(data, []byte("oldObject."+r.From), []byte("oldObject."+r.To))
	return data
}

// replace rewrites every occurrence of from with to, except occurrences that
// are immediately followed by a protected suffix (e.g. `.namespace`). Unlike a
// sentinel/placeholder swap, this never injects synthetic markers into the
// data, so it cannot collide with or corrupt user-provided content such as CEL
// expressions.
func replace(data, from, to []byte) []byte {
	if len(from) == 0 || bytes.Equal(from, to) {
		return data
	}
	// Fast path: if from never occurs, return data unchanged without
	// allocating or copying for the common no-op case.
	idx := bytes.Index(data, from)
	if idx < 0 {
		return data
	}
	// Pre-size the buffer. When the replacement expands the input (the common
	// case, e.g. `object.spec` -> `object.spec.template.spec`), grow by an
	// upper bound on the final size so the buffer never has to reallocate
	// mid-loop. Protected occurrences are left unchanged, so the real size is
	// at most this estimate.
	size := len(data)
	if len(to) > len(from) {
		size += bytes.Count(data, from) * (len(to) - len(from))
	}
	var buf bytes.Buffer
	buf.Grow(size)
	for idx >= 0 {
		buf.Write(data[:idx])
		rest := data[idx+len(from):]
		if isProtected(from, rest) {
			// Leave this occurrence untouched and continue scanning after it.
			buf.Write(from)
		} else {
			buf.Write(to)
		}
		data = rest
		idx = bytes.Index(data, from)
	}
	buf.Write(data)
	return buf.Bytes()
}

// isProtected reports whether rest (the bytes immediately following a match)
// begins with any of the protected suffixes as a complete path segment. The
// suffix must either end the expression or be followed by a non-identifier
// character so that fields like `metadata.namespace` are protected while
// hypothetical fields like `metadata.namespaceFoo` are not.
func isProtected(from, rest []byte) bool {
	// The protected suffixes (like .name and .namespace) only apply when rewriting metadata paths.
	// We do not want to protect .name if the user wrote object.spec.name.
	if !bytes.Contains(from, []byte("metadata")) {
		return false
	}
	for _, suffix := range protectedSuffixes {
		if !bytes.HasPrefix(rest, suffix) {
			continue
		}
		next := rest[len(suffix):]
		if len(next) == 0 || !isIdentifierByte(next[0]) {
			return true
		}
	}
	return false
}

// isIdentifierByte reports whether b can be part of a CEL identifier segment.
func isIdentifierByte(b byte) bool {
	return b == '_' ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		(b >= '0' && b <= '9')
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
		// dot syntax
		data = replaceCEL(data, []byte("object."+replacement.From), []byte("object."+replacement.To))
		data = replaceCEL(data, []byte("oldObject."+replacement.From), []byte("oldObject."+replacement.To))

		// equivalent selector forms (bracket syntax)
		data = replaceCEL(data, []byte("object['"+replacement.From+"']"), []byte("object."+replacement.To))
		data = replaceCEL(data, []byte("oldObject['"+replacement.From+"']"), []byte("oldObject."+replacement.To))

		data = replaceCEL(data, []byte("object[\""+replacement.From+"\"]"), []byte("object."+replacement.To))
		data = replaceCEL(data, []byte("oldObject[\""+replacement.From+"\"]"), []byte("oldObject."+replacement.To))
	}
	return data
}

// replaceCEL is a syntax-aware replacer that skips CEL string literals.
func replaceCEL(data, from, to []byte) []byte {
	if len(from) == 0 || bytes.Equal(from, to) {
		return data
	}
	idx := bytes.Index(data, from)
	if idx < 0 {
		return data
	}

	size := len(data)
	if len(to) > len(from) {
		size += bytes.Count(data, from) * (len(to) - len(from))
	}
	var buf bytes.Buffer
	buf.Grow(size)

	inString := false
	var stringQuote string
	escaped := false

	for i := 0; i < len(data); {
		if !inString {
			if bytes.HasPrefix(data[i:], from) {
				validBoundary := false
				if i == 0 {
					validBoundary = true
				} else if data[i-1] == '.' {
					if i >= 8 && bytes.Equal(data[i-8:i], []byte("request.")) {
						if i == 8 || (!isIdentifierByte(data[i-9]) && data[i-9] != '.') {
							validBoundary = true
						}
					}
				} else if !isIdentifierByte(data[i-1]) {
					validBoundary = true
				}

				if validBoundary {
					rest := data[i+len(from):]
					if len(rest) == 0 || !isIdentifierByte(rest[0]) {
						if isProtected(from, rest) {
							buf.Write(from)
						} else {
							buf.Write(to)
						}
						i += len(from)
						continue
					}
				}
			}

			// Check for string start
			if bytes.HasPrefix(data[i:], []byte("'''")) {
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
			} else if data[i] == '"' {
				inString = true
				stringQuote = "\""
			} else if data[i] == '`' {
				inString = true
				stringQuote = "`"
			}
		} else {
			// Check for string end
			if stringQuote == "'''" && bytes.HasPrefix(data[i:], []byte("'''")) && !escaped {
				inString = false
				buf.Write(data[i : i+3])
				i += 3
				continue
			} else if stringQuote == "\"\"\"" && bytes.HasPrefix(data[i:], []byte("\"\"\"")) && !escaped {
				inString = false
				buf.Write(data[i : i+3])
				i += 3
				continue
			} else if (stringQuote == "'" || stringQuote == "r'") && data[i] == '\'' && !escaped {
				inString = false
			} else if (stringQuote == "\"" || stringQuote == "r\"") && data[i] == '"' && !escaped {
				inString = false
			} else if stringQuote == "`" && data[i] == '`' {
				inString = false
			}

			// Manage escapes
			if data[i] == '\\' && stringQuote != "r'" && stringQuote != "r\"" && stringQuote != "`" {
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
