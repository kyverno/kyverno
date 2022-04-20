package common

import (
	"regexp"
)

// RegexVariables represents regex for '{{}}'
var RegexVariables = regexp.MustCompile(`\{\{[^{}]*\}\}`)

// IsHTTPRegex represents regex for starts with http:// or https://
var IsHTTPRegex = regexp.MustCompile("^(http|https)://")
