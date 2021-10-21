package common

import (
	"regexp"
)

// RegexVariables represents regex for '{{}}'
var RegexVariables = regexp.MustCompile(`\{\{[^{}]*\}\}`)

// AllowedVariables represents regex for {{request.}}, {{serviceAccountName}}, {{serviceAccountNamespace}}, {{@}} and functions e.g. {{divide(<num>,<num>))}}
var AllowedVariables = regexp.MustCompile(`\{\{\s*(request\.|serviceAccountName|serviceAccountNamespace|element\.|@|([a-z_0-9]+\())[^{}]*\}\}`)

// AllowedVariables represents regex for {{request.}}, {{serviceAccountName}}, {{serviceAccountNamespace}}
var WildCardAllowedVariables = regexp.MustCompile(`\{\{\s*(request\.|serviceAccountName|serviceAccountNamespace)[^{}]*\}\}`)

// IsHttpRegex represents regex for starts with http:// or https://
var IsHttpRegex = regexp.MustCompile("^(http|https)://")
