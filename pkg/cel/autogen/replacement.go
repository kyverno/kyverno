package autogen

import (
	"bytes"
	"fmt"
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
	[]byte(".?namespace"),
	[]byte(".?name"),
	[]byte("['name']"),
	[]byte("[\"name\"]"),
	[]byte("[\\\"name\\\"]"),
	[]byte("[?'name']"),
	[]byte("[?\"name\"]"),
	[]byte("[?\\\"name\\\"]"),
	[]byte("['namespace']"),
	[]byte("[\"namespace\"]"),
	[]byte("[\\\"namespace\\\"]"), // Handle JSON-escaped namespace paths
	[]byte("[?'namespace']"),
	[]byte("[?\"namespace\"]"),
	[]byte("[?\\\"namespace\\\"]"),
	[]byte("[? 'name']"),
	[]byte("[? \"name\"]"),
	[]byte("[? \\\"name\\\"]"),
	[]byte("[? 'namespace']"),
	[]byte("[? \"namespace\"]"),
	[]byte("[? \\\"namespace\\\"]"),
}

type Replacement struct {
	From string
	To   string
}

// Apply rewrites the configured field paths in the given data,
// replacing both the "object." and "oldObject." prefixes (including optional traversal ".?" and bracket selectors),
// while skipping CEL string literals.
func (r *Replacement) Apply(data []byte) []byte {
	// dot syntax
	data = replaceCEL(data, []byte("object."+r.From), []byte("object."+r.To))
	data = replaceCEL(data, []byte("object.?"+r.From), []byte("object.?"+r.To))
	data = replaceCEL(data, []byte("oldObject."+r.From), []byte("oldObject."+r.To))
	data = replaceCEL(data, []byte("oldObject.?"+r.From), []byte("oldObject.?"+r.To))

	// equivalent selector forms (bracket syntax)
	data = replaceCEL(data, []byte("object['"+r.From+"']"), []byte("object."+r.To))
	data = replaceCEL(data, []byte("oldObject['"+r.From+"']"), []byte("oldObject."+r.To))

	data = replaceCEL(data, []byte("object[\""+r.From+"\"]"), []byte("object."+r.To))
	data = replaceCEL(data, []byte("oldObject[\""+r.From+"\"]"), []byte("oldObject."+r.To))

	data = replaceCEL(data, []byte("object[\\\""+r.From+"\\\"]"), []byte("object."+r.To))
	data = replaceCEL(data, []byte("oldObject[\\\""+r.From+"\\\"]"), []byte("oldObject."+r.To))

	// optional indexing (bracket syntax with ?)
	data = replaceCEL(data, []byte("object[?'"+r.From+"']"), []byte("object.?"+r.To))
	data = replaceCEL(data, []byte("oldObject[?'"+r.From+"']"), []byte("oldObject.?"+r.To))

	data = replaceCEL(data, []byte("object[?\""+r.From+"\"]"), []byte("object.?"+r.To))
	data = replaceCEL(data, []byte("oldObject[?\""+r.From+"\"]"), []byte("oldObject.?"+r.To))

	data = replaceCEL(data, []byte("object[?\\\""+r.From+"\\\"]"), []byte("object.?"+r.To))
	data = replaceCEL(data, []byte("oldObject[?\\\""+r.From+"\\\"]"), []byte("oldObject.?"+r.To))

	// optional indexing with whitespace tolerance: object[? 'spec']
	data = replaceCEL(data, []byte("object[? '"+r.From+"']"), []byte("object.?"+r.To))
	data = replaceCEL(data, []byte("oldObject[? '"+r.From+"']"), []byte("oldObject.?"+r.To))
	data = replaceCEL(data, []byte("object[? \""+r.From+"\" ]"), []byte("object.?"+r.To))
	data = replaceCEL(data, []byte("oldObject[? \""+r.From+"\" ]"), []byte("oldObject.?"+r.To))
	data = replaceCEL(data, []byte("object[? \""+r.From+"\"]"), []byte("object.?"+r.To))
	data = replaceCEL(data, []byte("oldObject[? \""+r.From+"\"]"), []byte("oldObject.?"+r.To))
	data = replaceCEL(data, []byte("object[? \\\""+r.From+"\\\"]"), []byte("object.?"+r.To))
	data = replaceCEL(data, []byte("oldObject[? \\\""+r.From+"\\\"]"), []byte("oldObject.?"+r.To))

	return data
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

// isValidTrailingBoundary reports whether rest starts at a valid identifier boundary
// when from ends with an identifier byte.
func isValidTrailingBoundary(from, rest []byte) bool {
	if len(from) > 0 && isIdentifierByte(from[len(from)-1]) {
		return len(rest) == 0 || !isIdentifierByte(rest[0])
	}
	return true
}

// isProtected reports whether rest (the bytes immediately following a match)
// begins with any of the protected suffixes as a complete path segment. The
// suffix must either end the expression or be followed by a non-identifier
// character so that fields like `metadata.namespace` are protected while
// hypothetical fields like `metadata.namespaceFoo` are not.
func isProtected(from, rest []byte) bool {
	// The protected suffixes (like .name and .namespace) only apply when rewriting direct metadata paths (e.g. object.metadata),
	// not subfields like object.metadata.labels or object.metadata.annotations, and not object.spec.
	if !bytes.HasSuffix(from, []byte("metadata")) &&
		!bytes.HasSuffix(from, []byte("metadata']")) &&
		!bytes.HasSuffix(from, []byte("metadata\"]")) &&
		!bytes.HasSuffix(from, []byte("metadata\\\"]")) {
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
	return isProtectedBracket(rest)
}

// isProtectedBracket reports whether rest starts with a bracket selector for a protected field
// ('name' or 'namespace'), tolerating optional '?' and arbitrary whitespace within the brackets.
func isProtectedBracket(rest []byte) bool {
	if len(rest) < 3 || rest[0] != '[' {
		return false
	}
	i := 1
	for i < len(rest) && (rest[i] == ' ' || rest[i] == '\t' || rest[i] == '\r' || rest[i] == '\n') {
		i++
	}
	if i < len(rest) && rest[i] == '?' {
		i++
	}
	for i < len(rest) && (rest[i] == ' ' || rest[i] == '\t' || rest[i] == '\r' || rest[i] == '\n') {
		i++
	}
	if i >= len(rest) {
		return false
	}
	var quote []byte
	if rest[i] == '\'' {
		quote = []byte("'")
		i++
	} else if rest[i] == '"' {
		quote = []byte("\"")
		i++
	} else if bytes.HasPrefix(rest[i:], []byte(`\"`)) {
		quote = []byte(`\"`)
		i += 2
	} else {
		return false
	}
	if bytes.HasPrefix(rest[i:], []byte("namespace")) {
		i += 9
	} else if bytes.HasPrefix(rest[i:], []byte("name")) {
		i += 4
	} else {
		return false
	}
	if !bytes.HasPrefix(rest[i:], quote) {
		return false
	}
	i += len(quote)
	for i < len(rest) && (rest[i] == ' ' || rest[i] == '\t' || rest[i] == '\r' || rest[i] == '\n') {
		i++
	}
	return i < len(rest) && rest[i] == ']'
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
		data = replacement.Apply(data)
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
			// Skip CEL single-line comments so apostrophes inside comments do not trigger inString
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

			if bytes.HasPrefix(data[i:], from) {
				if isValidBoundary(data, i) {
					rest := data[i+len(from):]
					if isValidTrailingBoundary(from, rest) {
						if isProtected(from, rest) || isAlreadyRewritten(from, rest) {
							buf.Write(from)
						} else {
							buf.Write(to)
						}
						i += len(from)
						continue
					}
				}
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
			} else if data[i] == '"' {
				inString = true
				stringQuote = "\""
			} else if data[i] == '`' {
				inString = true
				stringQuote = "`"
			}
		} else {
			// Check for string end
			if (stringQuote == "'''" || stringQuote == "r'''") && bytes.HasPrefix(data[i:], []byte("'''")) {
				if stringQuote == "r'''" || !escaped {
					inString = false
					buf.Write(data[i : i+3])
					i += 3
					continue
				}
			} else if (stringQuote == "\"\"\"" || stringQuote == "r\"\"\"") && bytes.HasPrefix(data[i:], []byte("\"\"\"")) {
				if stringQuote == "r\"\"\"" || !escaped {
					inString = false
					buf.Write(data[i : i+3])
					i += 3
					continue
				}
			} else if (stringQuote == "'" || stringQuote == "r'") && data[i] == '\'' && (stringQuote == "r'" || !escaped) {
				inString = false
			} else if (stringQuote == "\"" || stringQuote == "r\"") && data[i] == '"' && (stringQuote == "r\"" || !escaped) {
				inString = false
			} else if stringQuote == "`" && data[i] == '`' {
				inString = false
			}

			// Manage escapes (raw strings and backtick strings do not process escape sequences)
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

// isAlreadyRewritten reports whether rest begins with .template / .jobTemplate,
// preventing redundant double-rewriting on spec paths.
// isAlreadyRewritten reports whether rest begins with .template / .jobTemplate as a complete path segment,
// preventing redundant double-rewriting on spec paths while preserving distinct fields like templateSpec.
func isAlreadyRewritten(from, rest []byte) bool {
	if bytes.HasSuffix(from, []byte("spec")) || bytes.HasSuffix(from, []byte("spec']")) || bytes.HasSuffix(from, []byte("spec\"]")) {
		suffixes := [][]byte{
			[]byte(".template"),
			[]byte(".?template"),
			[]byte(".jobTemplate"),
			[]byte(".?jobTemplate"),
		}
		for _, s := range suffixes {
			if bytes.HasPrefix(rest, s) {
				next := rest[len(s):]
				if len(next) == 0 || !isIdentifierByte(next[0]) {
					return true
				}
			}
		}
	}
	return false
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

	roots := []string{"object", "oldObject", "Object"}

	// 1. Rewrite metadata.labels and metadata.annotations first
	for _, root := range roots {
		// dot and optional dot syntax for labels
		for _, optMeta := range []string{".", ".?"} {
			for _, optLabels := range []string{".", ".?"} {
				from := fmt.Sprintf("%s%smetadata%slabels", root, optMeta, optLabels)
				to := fmt.Sprintf("%s%s%s%slabels", root, optMeta, metaRepl, optLabels)
				data = replaceCEL(data, []byte(from), []byte(to))
			}
			// bracket labels with dot/optional-dot metadata
			for _, q := range []string{"'", "\""} {
				fromOpt := fmt.Sprintf("%s%smetadata[?%slabels%s]", root, optMeta, q, q)
				fromNoOpt := fmt.Sprintf("%s%smetadata[%slabels%s]", root, optMeta, q, q)
				data = replaceCEL(data, []byte(fromOpt), []byte(fmt.Sprintf("%s%s%s.?labels", root, optMeta, metaRepl)))
				data = replaceCEL(data, []byte(fromNoOpt), []byte(fmt.Sprintf("%s%s%s.labels", root, optMeta, metaRepl)))
			}
		}
		// bracket metadata
		for _, qM := range []string{"'", "\""} {
			for _, optMeta := range []bool{false, true} {
				metaPrefix := fmt.Sprintf("%s[%s%smetadata%s]", root, map[bool]string{false: "", true: "?"}[optMeta], qM, qM)
				metaDelim := map[bool]string{false: ".", true: ".?"}[optMeta]

				for _, optLabels := range []string{".", ".?"} {
					from := fmt.Sprintf("%s%slabels", metaPrefix, optLabels)
					to := fmt.Sprintf("%s%s%s%slabels", root, metaDelim, metaRepl, optLabels)
					data = replaceCEL(data, []byte(from), []byte(to))
				}
				for _, qL := range []string{"'", "\""} {
					fromOpt := fmt.Sprintf("%s[?%slabels%s]", metaPrefix, qL, qL)
					fromNoOpt := fmt.Sprintf("%s[%slabels%s]", metaPrefix, qL, qL)
					data = replaceCEL(data, []byte(fromOpt), []byte(fmt.Sprintf("%s%s%s.?labels", root, metaDelim, metaRepl)))
					data = replaceCEL(data, []byte(fromNoOpt), []byte(fmt.Sprintf("%s%s%s.labels", root, metaDelim, metaRepl)))
				}
			}
		}

		// dot and optional dot syntax for annotations
		for _, optMeta := range []string{".", ".?"} {
			for _, optAnn := range []string{".", ".?"} {
				from := fmt.Sprintf("%s%smetadata%sannotations", root, optMeta, optAnn)
				to := fmt.Sprintf("%s%s%s%sannotations", root, optMeta, metaRepl, optAnn)
				data = replaceCEL(data, []byte(from), []byte(to))
			}
			for _, q := range []string{"'", "\""} {
				fromOpt := fmt.Sprintf("%s%smetadata[?%sannotations%s]", root, optMeta, q, q)
				fromNoOpt := fmt.Sprintf("%s%smetadata[%sannotations%s]", root, optMeta, q, q)
				data = replaceCEL(data, []byte(fromOpt), []byte(fmt.Sprintf("%s%s%s.?annotations", root, optMeta, metaRepl)))
				data = replaceCEL(data, []byte(fromNoOpt), []byte(fmt.Sprintf("%s%s%s.annotations", root, optMeta, metaRepl)))
			}
		}
		for _, qM := range []string{"'", "\""} {
			for _, optMeta := range []bool{false, true} {
				metaPrefix := fmt.Sprintf("%s[%s%smetadata%s]", root, map[bool]string{false: "", true: "?"}[optMeta], qM, qM)
				metaDelim := map[bool]string{false: ".", true: ".?"}[optMeta]

				for _, optAnn := range []string{".", ".?"} {
					from := fmt.Sprintf("%s%sannotations", metaPrefix, optAnn)
					to := fmt.Sprintf("%s%s%s%sannotations", root, metaDelim, metaRepl, optAnn)
					data = replaceCEL(data, []byte(from), []byte(to))
				}
				for _, qL := range []string{"'", "\""} {
					fromOpt := fmt.Sprintf("%s[?%sannotations%s]", metaPrefix, qL, qL)
					fromNoOpt := fmt.Sprintf("%s[%sannotations%s]", metaPrefix, qL, qL)
					data = replaceCEL(data, []byte(fromOpt), []byte(fmt.Sprintf("%s%s%s.?annotations", root, metaDelim, metaRepl)))
					data = replaceCEL(data, []byte(fromNoOpt), []byte(fmt.Sprintf("%s%s%s.annotations", root, metaDelim, metaRepl)))
				}
			}
		}

		// 2. Rewrite spec
		data = replaceCEL(data, []byte(root+".spec"), []byte(root+"."+specRepl))
		data = replaceCEL(data, []byte(root+".?spec"), []byte(root+".?"+specRepl))
		for _, q := range []string{"'", "\""} {
			for _, ws := range []string{"", " "} {
				data = replaceCEL(data, []byte(fmt.Sprintf("%s[%s%sspec%s]", root, ws, q, q)), []byte(root+"."+specRepl))
				data = replaceCEL(data, []byte(fmt.Sprintf("%s[?%s%sspec%s]", root, ws, q, q)), []byte(root+".?"+specRepl))
				data = replaceCEL(data, []byte(fmt.Sprintf("%s[ %s?%sspec%s]", root, ws, q, q)), []byte(root+".?"+specRepl))
				data = replaceCEL(data, []byte(fmt.Sprintf("%s[%s%sspec%s ]", root, ws, q, q)), []byte(root+"."+specRepl))
				data = replaceCEL(data, []byte(fmt.Sprintf("%s[?%s%sspec%s ]", root, ws, q, q)), []byte(root+".?"+specRepl))
			}
		}
	}

	return data
}
