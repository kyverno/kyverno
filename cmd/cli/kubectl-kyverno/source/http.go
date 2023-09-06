package source

import (
	"regexp"
)

var isHTTPRegex = regexp.MustCompile("^(http|https)://")

func IsHttp(in string) bool {
	return isHTTPRegex.MatchString(in)
}
