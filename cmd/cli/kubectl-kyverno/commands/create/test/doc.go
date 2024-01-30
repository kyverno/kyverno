package test

// TODO
var websiteUrl = ``

var description = []string{
	`Create a Kyverno test file.`,
}

var examples = [][]string{
	{
		"# Create test file",
		"kyverno create test -p policy.yaml -r resource.yaml -f values.yaml --pass policy-name,rule-name,resource-name,resource-namespace,resource-kind",
	},
}
