package values

// TODO
var websiteUrl = ``

var description = []string{
	`Create a Kyverno values file.`,
}

var examples = [][]string{
	{
		"# Create values file",
		"kyverno create values -g request.mode=dev -n prod,env=prod --rule policy,rule,env=demo --resource policy,resource,env=demo",
	},
}
