package wildcard

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	}

	// Iterating over the test cases, call the function under test and assert the output.
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actualResult := Match(testCase.pattern, testCase.text)
			assert.Equal(t, testCase.matched, actualResult)
		})
	}
}
