package apply

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#apply`

var description = []string{
	`Applies policies on resources.`,
}

var examples = [][]string{
	{
		"# Apply on a resource",
		"kyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --resource=/path/to/resource1 --resource=/path/to/resource2",
	},
	{
		"# Apply on a folder of resources",
		"kyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --resource=/path/to/resources/",
	},
	{
		"# Apply on a cluster",
		"kyverno apply /path/to/policy.yaml /path/to/folderOfPolicies --cluster",
	},
	{
		"# Apply policies from a gitSourceURL on a cluster",
		"kyverno apply https://github.com/kyverno/policies/openshift/ --git-branch main --cluster",
	},
	{
		"# Apply single policy with variable on single resource",
		"kyverno apply /path/to/policy.yaml --resource /path/to/resource.yaml --set <variable1>=<value1>,<variable2>=<value2>",
	},
	{
		"# Apply multiple policy with variable on multiple resource",
		"kyverno apply /path/to/policy1.yaml /path/to/policy2.yaml --resource /path/to/resource1.yaml --resource /path/to/resource2.yaml -f /path/to/value.yaml",
	},
}
