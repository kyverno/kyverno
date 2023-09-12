package test

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#test`

var description = []string{
	`Run tests from a local filesystem or a remote git repository.`,
	``,
	`The test command provides a facility to test resources against policies by comparing expected results,`,
	`declared ahead of time in a test manifest file, to actual results reported by Kyverno.`,
	``,
	`Users provide the path to the folder containing a kyverno-test.yaml file where the location could be`,
	`on a local filesystem or a remote git repository.`,
}

var examples = [][]string{
	{
		`# Test a git repository containing Kyverno test cases`,
		`kyverno test https://github.com/kyverno/policies/pod-security --git-branch main`,
	},
	{
		`# Test a local folder containing test cases`,
		`kyverno test .`,
	},
	{
		`# Test some specific test cases out of many test cases in a local folder`,
		`kyverno test . --test-case-selector "policy=disallow-latest-tag, rule=require-image-tag, resource=test-require-image-tag-pass"`,
	},
}
