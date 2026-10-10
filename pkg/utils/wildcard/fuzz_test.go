package wildcard

import (
	"regexp"
	"strings"
	"testing"
)

// reference is the specification Match is checked against. It translates the pattern
// into the equivalent regular expression and returns a function matching names with it.
// It lives in this file because OSS-Fuzz builds a fuzz target from its file alone.
func reference(pattern string) (func(name string) bool, error) {
	var expr strings.Builder
	expr.WriteString(`(?s)\A`)
	// ranging over the pattern turns invalid UTF-8 bytes into U+FFFD
	for _, r := range pattern {
		switch r {
		case '*':
			expr.WriteString(`.*`)
		case '?':
			expr.WriteString(`.`)
		default:
			expr.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	expr.WriteString(`\z`)
	re, err := regexp.Compile(expr.String())
	if err != nil {
		return nil, err
	}
	return func(name string) bool {
		// going through []rune turns invalid UTF-8 bytes into U+FFFD
		return re.MatchString(string([]rune(name)))
	}, nil
}

func FuzzMatch(f *testing.F) {
	f.Add("*", "kube-system")
	f.Add("kube-*", "kube-system")
	f.Add("my-?-bucket/abc*", "my-1-bucket/abc")
	f.Add("a*a*a*a*a*a*a*a*b", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	f.Add("名前-?-*", "名前-世-界")
	f.Add("\xff?*", "\xfe\xc3�")
	f.Fuzz(func(t *testing.T, pattern, name string) {
		got := Match(pattern, name)
		matches, err := reference(pattern)
		if err != nil {
			// the regexp package refuses expressions that are too large
			t.Skip(err)
		}
		if want := matches(name); got != want {
			t.Errorf("Match(%q, %q) = %v, want %v", pattern, name, got, want)
		}
	})
}
