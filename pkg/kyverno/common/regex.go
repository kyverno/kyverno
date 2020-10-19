package common

import (
	"regexp"
	"strings"
)

var REGEX_VARIABLES = regexp.MustCompile(`\{\{[^{}]*\}\}`)

var allowedList = []string{`request\..*`, `serviceAccountName`, `serviceAccountNamespace`}
var regexStr = `\{\{\s*(` + strings.Join(allowedList, "|") + `)[^{}]\s*\}\}`
var ALLOWED_VARIABLES = regexp.MustCompile(regexStr)