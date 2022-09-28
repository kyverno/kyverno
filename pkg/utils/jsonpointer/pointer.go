package jsonpointer

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"k8s.io/utils/strings/slices"
)

// Pointer is a JSON pointer that can be retrieved as either as a RFC6901 string or as a JMESPath formatted string.
type Pointer []string

// unquoted identifiers must only contain these characters.
var unquotedFirstCharRangeTable = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: '(', Hi: '(', Stride: 1}, // Special non-standard. Used by policy documents for matching attributes.
		{Lo: 'A', Hi: 'Z', Stride: 1},
		{Lo: '_', Hi: '_', Stride: 1},
		{Lo: 'a', Hi: 'z', Stride: 1},
	},
}

// unquoted identifiers can contain any combination of these runes.
var unquotedStringRangeTable = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: ')', Hi: ')', Stride: 1}, // Special non-standard. Used by policy documents for matching attributes.
		{Lo: '0', Hi: '9', Stride: 1},
		{Lo: 'A', Hi: 'Z', Stride: 1},
		{Lo: '_', Hi: '_', Stride: 1},
		{Lo: 'a', Hi: 'z', Stride: 1},
	},
}

// a quoted identifier can contain any of these characters as is.
var unescapedCharRangeTable = &unicode.RangeTable{
	R16: []unicode.Range16{
		{Lo: 0x20, Hi: 0x21, Stride: 1},
		{Lo: 0x23, Hi: 0x5B, Stride: 1},
		{Lo: 0x5D, Hi: 0x1EFF, Stride: 1},
	},
	R32: []unicode.Range32{
		{Lo: 0x1000, Hi: 0x10FFFF, Stride: 1},
	},
	LatinOffset: 0x1EFF - unicode.MaxLatin1,
}

// some special characters must be escaped to be possible to use inside a quoted identifier.
var escapeCharMap = map[rune]string{
	'"':  `\"`, // quotation mark
	'\\': `\\`, // reverse solidus
	'/':  `\/`, // solidus
	'\b': `\b`, // backspace
	'\f': `\f`, // form feed
	'\n': `\n`, // line feed
	'\r': `\r`, // carriage return
	'\t': `\t`, // tab
}

const initialCapacity = 10 // pointers should start with a non-zero capacity to lower the amount of re-allocations done by append.

// New will return an empty Pointer.
func New() Pointer {
	return make([]string, 0, initialCapacity)
}

// Parse will parse the string as a JSON pointer according to RFC 6901.
func Parse(s string) Pointer {
	pointer := New()

	replacer := strings.NewReplacer("~1", "/", "~0", "~", `\\`, `\`, `\"`, `"`)

	for _, component := range strings.FieldsFunc(s, func(r rune) bool {
		return r == '/'
	}) {
		pointer = append(pointer, replacer.Replace(component))
	}

	return pointer
}

// ParsePath will parse the raw path and return it in the form of a Pointer.
func ParsePath(rawPath string) Pointer {
	// Start with a slice with a non-zero capacity to avoid reallocation for most paths.
	pointer := New()

	// Use a string builder and a flush function to append path components to the slice.
	sb := strings.Builder{}

	flush := func() {
		s := sb.String()
		if s != "" {
			pointer = append(pointer, s)
		}
		sb.Reset()
	}

	var pos int
	var escaped, quoted bool

	for i, width := 0, 0; i <= len(rawPath); i += width {
		var r rune
		r, width = utf8.DecodeRuneInString(rawPath[i:])
		if r == utf8.RuneError && width == 1 {
			break
		}

		switch {
		case escaped: // previous character was a backslash.
			sb.WriteRune(r)
			escaped = !escaped
		case r == '\\': // escape character
			escaped = !escaped
		case r == '"': // quoted strings
			if quoted {
				s, _ := strconv.Unquote(rawPath[pos : i+width])
				sb.WriteString(s)
			}
			quoted = !quoted
		case r == '/' && !quoted:
			flush()
		case r == utf8.RuneError: // end of string
			flush()
			return pointer
		default:
			sb.WriteRune(r)
		}

		pos = i + width
	}

	// This is unreachable but we must return something.
	return pointer
}

// JMESPath will return the Pointer in the form of a JMESPath string.
func (p Pointer) JMESPath() string {
	sb := strings.Builder{}

	for _, component := range p {
		// Components that are valid unsigned integers are treated as indices.
		if _, err := strconv.ParseUint(component, 10, 64); err == nil {
			sb.WriteRune('[')
			sb.WriteString(component)
			sb.WriteRune(']')
			continue
		}

		// Write a dot before we write anything, as long as buffer is not empty.
		if sb.Len() > 0 {
			sb.WriteRune('.')
		}

		// If the component starts with a character that is valid as an initial character for an identifier
		// and the remaining characters are also valid for an unquoted identifier then we can append it to
		// the JMESPath as is.
		if ch, _ := utf8.DecodeRuneInString(component); unicode.Is(unquotedFirstCharRangeTable, ch) &&
			strings.IndexFunc(component, func(r rune) bool {
				return !unicode.Is(unquotedStringRangeTable, r)
			}) == -1 {
			sb.WriteString(component)
			continue
		}

		// The component contains characters that are not allowed for unquoted identifiers, so we need to take some extra
		// steps to ensure that it's a valid, quoted identifier.
		sb.WriteRune('"')
		for _, r := range component {
			// Any character in the range table of allowed runes can be written as is.
			if unicode.Is(unescapedCharRangeTable, r) {
				sb.WriteRune(r)
				continue
			}

			// Convert special characters to their escaped sequence.
			if escaped, ok := escapeCharMap[r]; ok {
				sb.WriteString(escaped)
				continue
			}

			// All other characters must be written as unicode escape sequences ay 16 bits a piece.
			if i := utf8.RuneLen(r); i <= 2 {
				// Rune is 1 or 2 bytes.
				_, _ = fmt.Fprintf(&sb, "\\u%04x", r&0xffff)
			} else {
				_, _ = fmt.Fprintf(&sb, "\\u%04x", r&0xffff)
				_, _ = fmt.Fprintf(&sb, "\\u%04x", r>>16)
			}
		}
		sb.WriteRune('"')
	}

	// Return the JMESPath.
	return sb.String()
}

// String will return the pointer as a string (RFC6901).
func (p Pointer) String() string {
	sb := strings.Builder{}

	replacer := strings.NewReplacer("~", "~0", "/", "~1", `\`, `\\`, `"`, `\"`)

	for _, component := range p {
		if sb.Len() > 0 {
			sb.WriteRune('/')
		}

		_, _ = replacer.WriteString(&sb, component)
	}

	// Return the pointer.
	return sb.String()
}

// Append will return a Pointer with the strings appended.
func (p Pointer) Append(s ...string) Pointer {
	return append(p, s...)
}

// Prepend will return a Pointer prefixed with the specified strings.
func (p Pointer) Prepend(s ...string) Pointer {
	return append(s, p...)
}

// AppendPath will parse the string as a JSON pointer and return a new pointer.
func (p Pointer) AppendPath(s string) Pointer {
	return append(p, ParsePath(s)...)
}

// SkipN will return a new Pointer where the first N element are stripped.
func (p Pointer) SkipN(n int) Pointer {
	if n > len(p)-1 {
		return []string{}
	}

	return p[n:]
}

// SkipPast will return a new Pointer where every element upto and including the specified string has been stripped off.
func (p Pointer) SkipPast(s string) Pointer {
	return p[slices.Index(p, s)+1:]
}
