package wildcard

import "testing"

var benchPatterns = []string{
	"kyverno-*",
	"default",
	"kube-*",
	"prod-*-ns",
	"*",
}

var benchNames = []string{
	"kyverno-admission",
	"default",
	"kube-system",
	"prod-east-ns",
	"other-namespace",
}

func BenchmarkMatch_Exact(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Match("my-namespace", "my-namespace")
	}
}

func BenchmarkMatch_WildcardSuffix(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Match("kyverno-*", "kyverno-admission")
	}
}

func BenchmarkMatch_WildcardPrefix(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Match("*-system", "kube-system")
	}
}

func BenchmarkMatch_WildcardBoth(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Match("*-admission-*", "kyverno-admission-controller")
	}
}

func BenchmarkMatch_NoMatch(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Match("prod-*", "staging-namespace")
	}
}

func BenchmarkMatchPatterns_OnePattern(b *testing.B) {
	patterns := []string{"kyverno-*"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatchPatterns(patterns, benchNames...)
	}
}

func BenchmarkMatchPatterns_ManyPatterns(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MatchPatterns(benchPatterns, benchNames...)
	}
}

func BenchmarkCheckPatterns(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CheckPatterns(benchPatterns, benchNames...)
	}
}

func BenchmarkContainsWildcard_WithWildcard(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ContainsWildcard("kyverno-*")
	}
}

func BenchmarkContainsWildcard_WithoutWildcard(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ContainsWildcard("kyverno-admission")
	}
}

func BenchmarkSeperateWildcards(b *testing.B) {
	input := []string{"kyverno-*", "default", "kube-*", "prod-ns", "staging-*"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SeperateWildcards(input)
	}
}
