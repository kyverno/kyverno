package exception

// TODO
var websiteUrl = ``

var description = []string{
	`Create a Kyverno policy exception file.`,
}

var examples = [][]string{
	{
		"# Create a policy exception file",
		`kyverno create exception -n my-exception --namespace my-ns --any "kind=Pod,kind=Deployment,name=test-*"`,
	},
}
