package source

import (
	"regexp"
)

// URI schemes are case-insensitive (RFC 3986, section 3.1).
var isHTTPRegex = regexp.MustCompile("(?i)^(http|https)://")

func IsHttp(in string) bool {
	return isHTTPRegex.MatchString(in)
}
