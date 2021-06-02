package common

import (
	"regexp"
)

// RegexVariables represents regex for '{{}}'
var RegexVariables = regexp.MustCompile(`\{\{[^{}]*\}\}`)

// AllowedVariables represents regex for {{request.}}, {{serviceAccountName}}, {{serviceAccountNamespace}} and {{@}}
var AllowedVariables = regexp.MustCompile(`\{\{\s*[request\.|serviceAccountName|serviceAccountNamespace|@][^{}]*\}\}`)
