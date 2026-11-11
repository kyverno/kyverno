package source

import "regexp"

var isHTTPRegex = regexp.MustCompile("(?i)^(http|https)://")

func IsHttp(in string) bool {
	return isHTTPRegex.MatchString(in)
}
