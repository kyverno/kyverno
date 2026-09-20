package wildcard

import "strings"

// ContainsWildcard reports whether v contains a '*' or '?' wildcard.
func ContainsWildcard(v string) bool {
	return strings.ContainsAny(v, "*?")
}

// MatchPatterns returns the first name matching one of the patterns, together with
// the pattern it matched.
func MatchPatterns(patterns []string, names ...string) (string, string, bool) {
	for _, name := range names {
		for _, pattern := range patterns {
			if Match(pattern, name) {
				return pattern, name, true
			}
		}
	}
	return "", "", false
}

// CheckPatterns reports whether any of the names matches any of the patterns.
func CheckPatterns(patterns []string, names ...string) bool {
	_, _, match := MatchPatterns(patterns, names...)
	return match
}

// SeparateWildcards splits l into the elements containing a wildcard and the others.
func SeparateWildcards(l []string) (wildcards []string, others []string) {
	for _, val := range l {
		if ContainsWildcard(val) {
			wildcards = append(wildcards, val)
		} else {
			others = append(others, val)
		}
	}
	return wildcards, others
}
