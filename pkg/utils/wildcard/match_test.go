package wildcard

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatch(t *testing.T) {
	testCases := []struct {
		name    string
		pattern string
		text    string
		matched bool
	}{
		{
			name:    "star pattern matches any text",
			pattern: "*",
			text:    "s3:GetObject",
			matched: true,
		},
		{
			name:    "empty pattern matches nothing on non-empty text",
			pattern: "",
			text:    "s3:GetObject",
			matched: false,
		},
		{
			name:    "empty pattern matches empty text",
			pattern: "",
			text:    "",
			matched: true,
		},
		{
			name:    "single star wildcard at suffix matches prefix",
			pattern: "s3:*",
			text:    "s3:ListMultipartUploadParts",
			matched: true,
		},
		{
			name:    "no wildcard exact mismatch on suffix",
			pattern: "s3:ListBucketMultipartUploads",
			text:    "s3:ListBucket",
			matched: false,
		},
		{
			name:    "no wildcard exact match succeeds",
			pattern: "s3:ListBucket",
			text:    "s3:ListBucket",
			matched: true,
		},
		{
			name:    "no wildcard exact match succeeds long suffix",
			pattern: "s3:ListBucketMultipartUploads",
			text:    "s3:ListBucketMultipartUploads",
			matched: true,
		},
		{
			name:    "prefix with wildcard matches prefix alone",
			pattern: "my-bucket/oo*",
			text:    "my-bucket/oo",
			matched: true,
		},
		{
			name:    "star wildcard at suffix matches multiple subdirectories",
			pattern: "my-bucket/In*",
			text:    "my-bucket/India/Karnataka/",
			matched: true,
		},
		{
			name:    "star wildcard at suffix fails when prefixes are shuffled",
			pattern: "my-bucket/In*",
			text:    "my-bucket/Karnataka/India/",
			matched: false,
		},
		{
			name:    "multiple wildcards match correctly expanded path segments",
			pattern: "my-bucket/In*/Ka*/Ban",
			text:    "my-bucket/India/Karnataka/Ban",
			matched: true,
		},
		{
			name:    "multiple wildcards match when path segments are repeated",
			pattern: "my-bucket/In*/Ka*/Ban",
			text:    "my-bucket/India/Karnataka/Ban/Ban/Ban/Ban/Ban",
			matched: true,
		},
		{
			name:    "wildcard expands to match multiple intermediate subdirectories",
			pattern: "my-bucket/In*/Ka*/Ban",
			text:    "my-bucket/India/Karnataka/Area1/Area2/Area3/Ban",
			matched: true,
		},
		{
			name:    "wildcard expands to match multiple subdirectories at multiple levels",
			pattern: "my-bucket/In*/Ka*/Ban",
			text:    "my-bucket/India/State1/State2/Karnataka/Area1/Area2/Area3/Ban",
			matched: true,
		},
		{
			name:    "multiple wildcards fail when trailing segment is modified",
			pattern: "my-bucket/In*/Ka*/Ban",
			text:    "my-bucket/India/Karnataka/Bangalore",
			matched: false,
		},
		{
			name:    "multiple wildcards match when trailing segment contains wildcard",
			pattern: "my-bucket/In*/Ka*/Ban*",
			text:    "my-bucket/India/Karnataka/Bangalore",
			matched: true,
		},
		{
			name:    "star wildcard matches single folder path suffix",
			pattern: "my-bucket/*",
			text:    "my-bucket/India",
			matched: true,
		},
		{
			name:    "prefix wildcard fails on incorrect match prefix",
			pattern: "my-bucket/oo*",
			text:    "my-bucket/odo",
			matched: false,
		},
		{
			name:    "question mark wildcard fails on missing character",
			pattern: "my-bucket?/abc*",
			text:    "mybucket/abc",
			matched: false,
		},
		{
			name:    "question mark wildcard matches single numeric character",
			pattern: "my-bucket?/abc*",
			text:    "my-bucket1/abc",
			matched: true,
		},
		{
			name:    "question mark fails on missing middle character",
			pattern: "my-?-bucket/abc*",
			text:    "my--bucket/abc",
			matched: false,
		},
		{
			name:    "question mark matches middle number",
			pattern: "my-?-bucket/abc*",
			text:    "my-1-bucket/abc",
			matched: true,
		},
		{
			name:    "question mark matches middle character letter",
			pattern: "my-?-bucket/abc*",
			text:    "my-k-bucket/abc",
			matched: true,
		},
		{
			name:    "two question marks fail on missing characters",
			pattern: "my??bucket/abc*",
			text:    "mybucket/abc",
			matched: false,
		},
		{
			name:    "two question marks match two characters",
			pattern: "my??bucket/abc*",
			text:    "my4abucket/abc",
			matched: true,
		},
		{
			name:    "question mark matches path separator slash",
			pattern: "my-bucket?abc*",
			text:    "my-bucket/abc",
			matched: true,
		},
		{
			name:    "question mark matches middle character in filename",
			pattern: "my-bucket/abc?efg",
			text:    "my-bucket/abcdefg",
			matched: true,
		},
		{
			name:    "question mark matches slash separator in middle",
			pattern: "my-bucket/abc?efg",
			text:    "my-bucket/abc/efg",
			matched: true,
		},
		{
			name:    "four question marks fail on too short suffix",
			pattern: "my-bucket/abc????",
			text:    "my-bucket/abc",
			matched: false,
		},
		{
			name:    "four question marks fail on two short characters suffix",
			pattern: "my-bucket/abc????",
			text:    "my-bucket/abcde",
			matched: false,
		},
		{
			name:    "four question marks match exact character suffix length",
			pattern: "my-bucket/abc????",
			text:    "my-bucket/abcdefg",
			matched: true,
		},
		{
			name:    "single question mark fails on empty match character",
			pattern: "my-bucket/abc?",
			text:    "my-bucket/abc",
			matched: false,
		},
		{
			name:    "single question mark matches single character suffix",
			pattern: "my-bucket/abc?",
			text:    "my-bucket/abcd",
			matched: true,
		},
		{
			name:    "single question mark fails on too long suffix text",
			pattern: "my-bucket/abc?",
			text:    "my-bucket/abcde",
			matched: false,
		},
		{
			name:    "wildcard and question mark fails on missing single character",
			pattern: "my-bucket/mnop*?",
			text:    "my-bucket/mnop",
			matched: false,
		},
		{
			name:    "wildcard and question mark matches multi segment text",
			pattern: "my-bucket/mnop*?",
			text:    "my-bucket/mnopqrst/mnopqr",
			matched: true,
		},
		{
			name:    "wildcard and question mark matches multi segment text long",
			pattern: "my-bucket/mnop*?",
			text:    "my-bucket/mnopqrst/mnopqrs",
			matched: true,
		},
		{
			name:    "wildcard and question mark fails on missing character duplicate case",
			pattern: "my-bucket/mnop*?",
			text:    "my-bucket/mnop",
			matched: false,
		},
		{
			name:    "wildcard and question mark matches single character suffix",
			pattern: "my-bucket/mnop*?",
			text:    "my-bucket/mnopq",
			matched: true,
		},
		{
			name:    "wildcard and question mark matches two character suffix",
			pattern: "my-bucket/mnop*?",
			text:    "my-bucket/mnopqr",
			matched: true,
		},
		{
			name:    "wildcard and question mark with final suffix matches",
			pattern: "my-bucket/mnop*?and",
			text:    "my-bucket/mnopqand",
			matched: true,
		},
		{
			name:    "wildcard and question mark with final suffix fails on missing character",
			pattern: "my-bucket/mnop*?and",
			text:    "my-bucket/mnopand",
			matched: false,
		},
		{
			name:    "wildcard and question mark with final suffix matches duplicate case",
			pattern: "my-bucket/mnop*?and",
			text:    "my-bucket/mnopqand",
			matched: true,
		},
		{
			name:    "wildcard and question mark fails on prefix mismatch",
			pattern: "my-bucket/mnop*?",
			text:    "my-bucket/mn",
			matched: false,
		},
		{
			name:    "wildcard and question mark matches duplicate long case",
			pattern: "my-bucket/mnop*?",
			text:    "my-bucket/mnopqrst/mnopqrs",
			matched: true,
		},
		{
			name:    "wildcard and two question marks match long text",
			pattern: "my-bucket/mnop*??",
			text:    "my-bucket/mnopqrst",
			matched: true,
		},
		{
			name:    "wildcard in middle matches correctly",
			pattern: "my-bucket/mnop*qrst",
			text:    "my-bucket/mnopabcdegqrst",
			matched: true,
		},
		{
			name:    "wildcard and question mark with final suffix matches duplicate case 3",
			pattern: "my-bucket/mnop*?and",
			text:    "my-bucket/mnopqand",
			matched: true,
		},
		{
			name:    "wildcard and question mark with final suffix fails on missing character duplicate case 2",
			pattern: "my-bucket/mnop*?and",
			text:    "my-bucket/mnopand",
			matched: false,
		},
		{
			name:    "wildcards and question marks match trailing pattern segment",
			pattern: "my-bucket/mnop*?and?",
			text:    "my-bucket/mnopqanda",
			matched: true,
		},
		{
			name:    "wildcard and question mark fail on extra trailing suffix characters",
			pattern: "my-bucket/mnop*?and",
			text:    "my-bucket/mnopqanda",
			matched: false,
		},
		{
			name:    "question mark and wildcard fail when prefix is incorrect",
			pattern: "my-?-bucket/abc*",
			text:    "my-bucket/mnopqanda",
			matched: false,
		},
		{
			name:    "star matches empty text",
			pattern: "*",
			text:    "",
			matched: true,
		},
		{
			name:    "consecutive stars match empty text",
			pattern: "***",
			text:    "",
			matched: true,
		},
		{
			name:    "consecutive stars behave like a single star",
			pattern: "my-**-bucket",
			text:    "my-own-bucket",
			matched: true,
		},
		{
			name:    "trailing stars match after text is exhausted",
			pattern: "my-bucket**",
			text:    "my-bucket",
			matched: true,
		},
		{
			name:    "leading star matches suffix",
			pattern: "*:webhook",
			text:    "kyverno:webhook",
			matched: true,
		},
		{
			name:    "leading star fails when suffix is followed by more text",
			pattern: "*:webhook",
			text:    "kyverno:webhooks",
			matched: false,
		},
		{
			name:    "leading and trailing stars match in the middle",
			pattern: "*needle*",
			text:    "hay-needle-stack",
			matched: true,
		},
		{
			name:    "leading and trailing stars fail when text is missing",
			pattern: "*needle*",
			text:    "haystack",
			matched: false,
		},
		{
			name:    "star gives up a partial match to find a later one",
			pattern: "*abc",
			text:    "ababc",
			matched: true,
		},
		{
			name:    "last star grows until the rest of the pattern matches",
			pattern: "a*b*c",
			text:    "abbcc",
			matched: true,
		},
		{
			name:    "text exhausted before the pattern following a star",
			pattern: "my-*-bucket",
			text:    "my-",
			matched: false,
		},
		{
			name:    "no wildcard fails on text longer than pattern",
			pattern: "s3:ListBucket",
			text:    "s3:ListBucketMultipartUploads",
			matched: false,
		},
		{
			name:    "question mark fails on empty text",
			pattern: "?",
			text:    "",
			matched: false,
		},
		{
			name:    "star and question mark fail on empty text",
			pattern: "*?",
			text:    "",
			matched: false,
		},
		{
			name:    "stars and question marks fail on too short text",
			pattern: "*?*?*",
			text:    "a",
			matched: false,
		},
		{
			name:    "stars and question marks match long enough text",
			pattern: "*?*?*",
			text:    "ab",
			matched: true,
		},
		{
			name:    "matching is case sensitive",
			pattern: "Pod*",
			text:    "pods",
			matched: false,
		},
		{
			name:    "wildcards match newlines",
			pattern: "first?second*",
			text:    "first\nsecond\nthird",
			matched: true,
		},
		{
			name:    "backslash does not escape wildcards",
			pattern: `my-bucket\*`,
			text:    `my-bucket\abc`,
			matched: true,
		},
		{
			name:    "backslash is matched literally",
			pattern: `my-bucket\*`,
			text:    "my-bucket*",
			matched: false,
		},
		{
			name:    "brackets are matched literally",
			pattern: "my-bucket[a-z]",
			text:    "my-bucket[a-z]",
			matched: true,
		},
		{
			name:    "brackets are not a character class",
			pattern: "my-bucket[a-z]",
			text:    "my-bucketa",
			matched: false,
		},
		{
			name:    "question mark matches single multi-byte character",
			pattern: "my-?-bucket",
			text:    "my-世-bucket",
			matched: true,
		},
		{
			name:    "question mark fails on two multi-byte characters",
			pattern: "my-?-bucket",
			text:    "my-世界-bucket",
			matched: false,
		},
		{
			name:    "two question marks match two multi-byte characters",
			pattern: "my-??-bucket",
			text:    "my-世界-bucket",
			matched: true,
		},
		{
			name:    "two question marks fail on single multi-byte character",
			pattern: "??",
			text:    "世",
			matched: false,
		},
		{
			name:    "star does not split a multi-byte character",
			pattern: "*??-bucket*",
			text:    "世-bucket-1",
			matched: false,
		},
		{
			name:    "star matches multi-byte characters",
			pattern: "名前-*-bucket",
			text:    "名前-世界-bucket",
			matched: true,
		},
		{
			name:    "multi-byte characters sharing their first byte are different",
			pattern: "café",
			text:    "cafè",
			matched: false,
		},
		{
			name:    "question mark matches single invalid byte",
			pattern: "my-?-bucket",
			text:    "my-\xff-bucket",
			matched: true,
		},
		{
			name:    "question mark fails on two invalid bytes",
			pattern: "my-?-bucket",
			text:    "my-\xff\xfe-bucket",
			matched: false,
		},
		{
			name:    "star matches invalid bytes",
			pattern: "my-*-bucket",
			text:    "my-\xff\xfe-bucket",
			matched: true,
		},
		{
			name:    "invalid bytes all match each other",
			pattern: "my-\xff-bucket",
			text:    "my-\xfe-bucket",
			matched: true,
		},
		{
			name:    "invalid byte matches the replacement character",
			pattern: "my-�-bucket",
			text:    "my-\xff-bucket",
			matched: true,
		},
		{
			name:    "many stars fail without exponential backtracking",
			pattern: strings.Repeat("a*", 64) + "b",
			text:    strings.Repeat("a", 4096),
			matched: false,
		},
		{
			name:    "many stars match without exponential backtracking",
			pattern: strings.Repeat("a*", 64) + "b",
			text:    strings.Repeat("a", 4096) + "b",
			matched: true,
		},
		{
			name:    "long partial matches after a star fail in reasonable time",
			pattern: "*" + strings.Repeat("a", 64) + "b",
			text:    strings.Repeat("a", 4096),
			matched: false,
		},
	}

	// Iterating over the test cases, call the function under test and assert the output.
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actualResult := Match(testCase.pattern, testCase.text)
			assert.Equal(t, testCase.matched, actualResult)
		})
	}
}

// combinations returns all the strings made of at most n of the given parts.
func combinations(parts []string, n int) []string {
	all := []string{""}
	for last := all; n > 0; n-- {
		var next []string
		for _, prefix := range last {
			for _, part := range parts {
				next = append(next, prefix+part)
			}
		}
		all = append(all, next...)
		last = next
	}
	return all
}

func TestMatchReference(t *testing.T) {
	// "é" and "è" share their first byte, "\xff" and "\xfe" are not valid UTF-8 and
	// have to match each other
	patterns := combinations([]string{"a", "é", "è", "\xff", "*", "?"}, 4)
	names := combinations([]string{"a", "é", "è", "\xfe"}, 4)
	for _, pattern := range patterns {
		matches, err := reference(pattern)
		require.NoError(t, err)
		for _, name := range names {
			if got, want := Match(pattern, name), matches(name); got != want {
				t.Errorf("Match(%q, %q) = %v, want %v", pattern, name, got, want)
			}
		}
	}
}

func BenchmarkMatch(b *testing.B) {
	benchmarks := []struct {
		name    string
		pattern string
		text    string
	}{
		{name: "star", pattern: "*", text: "kube-system"},
		{name: "literal", pattern: "kube-system", text: "kube-system"},
		{name: "prefix", pattern: "kube-*", text: "kube-system"},
		{name: "suffix", pattern: "*-system", text: "kube-system"},
		{name: "image", pattern: "registry.io/*/app:v?.*", text: "registry.io/team/nested/app:v1.22.3"},
		{name: "unicode", pattern: "名前-?-*", text: "名前-世-界"},
		{name: "many stars", pattern: strings.Repeat("a*", 64) + "b", text: strings.Repeat("a", 4096)},
	}
	for _, benchmark := range benchmarks {
		b.Run(benchmark.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				Match(benchmark.pattern, benchmark.text)
			}
		})
	}
}
