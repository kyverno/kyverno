package common

import (
	"regexp"
)

// RegexVariables represents regex for '{{}}'
var RegexVariables = regexp.MustCompile(`\{\{[^{}]*\}\}`)

// AllowedVariables represents regex for {{request.}}, {{serviceAccountName}}, {{serviceAccountNamespace}}, {{@}} and functions e.g. {{divide(<num>,<num>))}}
var AllowedVariables = regexp.MustCompile(`\{\{\s*(request\.|serviceAccountName|serviceAccountNamespace|@|([a-z_]+\())[^{}]*\}\}`)

// AllowedVariables represents regex for {{request.}}, {{serviceAccountName}}, {{serviceAccountNamespace}}
var WildCardAllowedVariables = regexp.MustCompile(`\{\{\s*(request\.|serviceAccountName|serviceAccountNamespace|userInfo)[^{}]*\}\}`)

// IsHttpRegex represents regex for starts with http:// or https://
var IsHttpRegex = regexp.MustCompile("^(http|https)://")
