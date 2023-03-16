package wildcard

import "strings"

func ContainsWildcard(v string) bool {
	return strings.Contains(v, "*") || strings.Contains(v, "?")
}

// MatchPatterns check if any text satisfies any pattern
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

// CheckPatterns check if any text satisfies any pattern
func CheckPatterns(patterns []string, names ...string) bool {
	_, _, match := MatchPatterns(patterns, names...)
	return match
}

func SeperateWildcards(l []string) (lw []string, rl []string) {
	for _, val := range l {
		if ContainsWildcard(val) {
			lw = append(lw, val)
		} else {
			rl = append(rl, val)
		}
	}
	return lw, rl
}
