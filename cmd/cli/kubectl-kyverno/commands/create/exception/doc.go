package exception

// TODO
var websiteUrl = ``

var description = []string{
	`Create a Kyverno policy exception file.`,
}

var examples = [][]string{
	{
		"# Create a policy exception file",
		`kyverno create exception my-exception --namespace my-ns --policy-rules "policy,rule-1,rule-2" --any "kind=Pod,kind=Deployment,name=test-*"`,
	},
}
