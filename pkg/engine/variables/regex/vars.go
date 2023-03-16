package regex

import "regexp"

var (
	RegexVariables = regexp.MustCompile(`(^|[^\\])(\{\{(?:\{[^{}]*\}|[^{}])*\}\})`)

	RegexEscpVariables = regexp.MustCompile(`\\\{\{(\{[^{}]*\}|[^{}])*\}\}`)

	// RegexReferences is the Regex for '$(...)' at the beginning of the string, and 'x$(...)' where 'x' is not '\'
	RegexReferences = regexp.MustCompile(`^\$\(.[^\ ]*\)|[^\\]\$\(.[^\ ]*\)`)

	// RegexEscpReferences is the Regex for '\$(...)'
	RegexEscpReferences = regexp.MustCompile(`\\\$\(.[^\ ]*\)`)

	RegexVariableInit = regexp.MustCompile(`^\{\{(\{[^{}]*\}|[^{}])*\}\}`)

	RegexElementIndex = regexp.MustCompile(`{{\s*elementIndex\d*\s*}}`)

	RegexVariableKey = regexp.MustCompile(`\{{(.*?)\}}`)
)
