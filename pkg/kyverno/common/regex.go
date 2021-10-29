package common

import (
	"regexp"
)

// RegexVariables represents regex for '{{}}'
var RegexVariables = regexp.MustCompile(`\{\{[^{}]*\}\}`)

// AllowedVariables represents regex for {{request.}}, {{serviceAccountName}}, {{serviceAccountNamespace}}, {{@}}, {{element.}}, {{images.}}
var AllowedVariables = regexp.MustCompile(`\{\{\s*(request\.|serviceAccountName|serviceAccountNamespace|element\.|@|images\.|([a-z_0-9]+\())[^{}]*\}\}`)

// WildCardAllowedVariables represents regex for the allowed fields in wildcards
var WildCardAllowedVariables = regexp.MustCompile(`\{\{\s*(request\.|serviceAccountName|serviceAccountNamespace)[^{}]*\}\}`)

// IsHTTPRegex represents regex for starts with http:// or https://
var IsHTTPRegex = regexp.MustCompile("^(http|https)://")
