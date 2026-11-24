package autogen

import (
	"strings"
)

// ScanBalancedBraces scans content (starting immediately after an initial '{')
// and returns the index of the matching closing '}'.
// It is syntax-aware: '{' and '}' inside CEL string literals (including raw and
// multi-line strings) and comments (both line comments and block comments)
// are ignored.
// It returns (endIndex, true) if the matching closing brace was found, or (-1, false) otherwise.
func ScanBalancedBraces(content string) (int, bool) {
	braceCount := 1
	inString := false
	var stringQuote string
	escaped := false
	n := len(content)

	for i := 0; i < n; {
		if !inString {
			// Skip single-line comments: // ...
			if strings.HasPrefix(content[i:], "//") {
				newlineIdx := strings.IndexByte(content[i:], '\n')
				if newlineIdx == -1 {
					break
				}
				i += newlineIdx + 1
				continue
			}

			// Skip block comments: /* ... */
			if strings.HasPrefix(content[i:], "/*") {
				endCommentIdx := strings.Index(content[i+2:], "*/")
				if endCommentIdx == -1 {
					break
				}
				i += 2 + endCommentIdx + 2
				continue
			}

			// Check for string literal prefixes (check raw triple quotes before raw single quotes)
			if strings.HasPrefix(content[i:], "r'''") || strings.HasPrefix(content[i:], "R'''") {
				inString = true
				stringQuote = "r'''"
				i += 4
				continue
			} else if strings.HasPrefix(content[i:], `r"""`) || strings.HasPrefix(content[i:], `R"""`) {
				inString = true
				stringQuote = `r"""`
				i += 4
				continue
			} else if strings.HasPrefix(content[i:], "'''") {
				inString = true
				stringQuote = "'''"
				i += 3
				continue
			} else if strings.HasPrefix(content[i:], `"""`) {
				inString = true
				stringQuote = `"""`
				i += 3
				continue
			} else if strings.HasPrefix(content[i:], `r"`) || strings.HasPrefix(content[i:], `R"`) {
				inString = true
				stringQuote = `r"`
				i += 2
				continue
			} else if strings.HasPrefix(content[i:], `r'`) || strings.HasPrefix(content[i:], `R'`) {
				inString = true
				stringQuote = `r'`
				i += 2
				continue
			} else if content[i] == '`' {
				inString = true
				stringQuote = "`"
				i++
				continue
			} else if content[i] == '"' {
				inString = true
				stringQuote = `"`
				i++
				continue
			} else if content[i] == '\'' {
				inString = true
				stringQuote = `'`
				i++
				continue
			}

			if content[i] == '{' {
				braceCount++
			} else if content[i] == '}' {
				braceCount--
				if braceCount == 0 {
					return i, true
				}
			}
			i++
		} else {
			// Inside string literal
			isRaw := strings.HasPrefix(stringQuote, "r") || strings.HasPrefix(stringQuote, "R") || stringQuote == "`"
			closingQuote := strings.TrimPrefix(strings.TrimPrefix(stringQuote, "r"), "R")

			if !isRaw && content[i] == '\\' && !escaped {
				escaped = true
				i++
				continue
			}

			if escaped {
				escaped = false
				i++
				continue
			}

			if strings.HasPrefix(content[i:], closingQuote) {
				inString = false
				i += len(closingQuote)
				continue
			}

			i++
		}
	}

	return -1, false
}
