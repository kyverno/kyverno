package values

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#create`

var description = []string{
	`Create a Kyverno values file.`,
}

var examples = [][]string{
	{
		"# Create values file",
		"kyverno create values -g request.mode=dev -n prod,env=prod --rule policy,rule,env=demo --resource policy,resource,env=demo",
	},
}
