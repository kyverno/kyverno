package common

import (
	"regexp"
)

var REGEX_VARIABLES = regexp.MustCompile(`\{\{[^{}]*\}\}`)
var ALLOWED_VARIABLES = regexp.MustCompile(`\{\{\s*[request\.|serviceAccountName|serviceAccountNamespace][^{}]*\}\}`)