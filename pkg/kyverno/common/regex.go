package common

import (
	"regexp"
)

// RegexVariables represents regex for '{{}}'
var RegexVariables = regexp.MustCompile(`\{\{[^{}]*\}\}`)

// AllowedVariables represents regex for {{request.}}, {{serviceAccountName}}, {{serviceAccountNamespace}}, {{@}} and {{divide(<num>,<num>))}} (#2409)
var AllowedVariables = regexp.MustCompile(`\{\{\s*(request\.|serviceAccountName|serviceAccountNamespace|@|divide)[^{}]*\}\}`)

// IsHttpRegex represents regex for starts with http:// or https://
var IsHttpRegex = regexp.MustCompile("^(http|https)://")
