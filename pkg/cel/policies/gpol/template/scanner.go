package template

import (
	"fmt"
	"strings"
)

// segment is a fragment of a scalar value: either a literal string or a CEL
// expression extracted from a `(( ... ))` placeholder.
type segment struct {
	literal    string
	expression string
	isExpr     bool
}

// scan splits a scalar string into literal and placeholder segments.
//
// Placeholders are delimited by `((` and `))`. The scanner is aware of
// parentheses nesting and CEL string literals, so expressions like
// `(( string(f(x)) ))` or `(( ":)".size() ))` are handled correctly.
// A literal `((` can be produced with the escape sequence `\((`.
func scan(s string) ([]segment, error) {
	var segments []segment
	var literal strings.Builder
	flush := func() {
		if literal.Len() > 0 {
			segments = append(segments, segment{literal: literal.String()})
			literal.Reset()
		}
	}
	i := 0
	for i < len(s) {
		if strings.HasPrefix(s[i:], `\((`) {
			literal.WriteString("((")
			i += 3
			continue
		}
		if strings.HasPrefix(s[i:], "((") {
			expr, next, err := scanPlaceholder(s, i)
			if err != nil {
				return nil, err
			}
			flush()
			segments = append(segments, segment{expression: expr, isExpr: true})
			i = next
			continue
		}
		literal.WriteByte(s[i])
		i++
	}
	flush()
	return segments, nil
}

// scanPlaceholder scans a placeholder starting at s[start] (which must be the
// opening `((`). It returns the trimmed CEL expression and the index of the
// first byte after the closing `))`.
func scanPlaceholder(s string, start int) (string, int, error) {
	depth := 0
	var quote byte
	i := start + 2
	for i < len(s) {
		c := s[i]
		if quote != 0 {
			switch c {
			case '\\':
				i += 2
				continue
			case quote:
				quote = 0
			}
			i++
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			} else if i+1 < len(s) && s[i+1] == ')' {
				expr := strings.TrimSpace(s[start+2 : i])
				if expr == "" {
					return "", 0, fmt.Errorf("empty placeholder at offset %d", start)
				}
				return expr, i + 2, nil
			} else {
				return "", 0, fmt.Errorf("unbalanced parenthesis in placeholder starting at offset %d", start)
			}
		}
		i++
	}
	if quote != 0 {
		return "", 0, fmt.Errorf("unterminated string literal in placeholder starting at offset %d", start)
	}
	return "", 0, fmt.Errorf("unterminated placeholder starting at offset %d: missing closing '))'", start)
}

// containsPlaceholder reports whether s contains an unescaped `((`.
func containsPlaceholder(s string) bool {
	for i := 0; i < len(s); i++ {
		if strings.HasPrefix(s[i:], `\((`) {
			i += 2
			continue
		}
		if strings.HasPrefix(s[i:], "((") {
			return true
		}
	}
	return false
}

// containsEscape reports whether s contains the `\((` escape sequence.
func containsEscape(s string) bool {
	return strings.Contains(s, `\((`)
}
