package jp

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#jp`

var description = []string{
	`Provides a command-line interface to JMESPath, enhanced with Kyverno specific custom functions.`,
}

var examples = [][]string{
	{
		"# List functions",
		"kyverno jp function",
	},
	{
		"# Evaluate query",
		"kyverno jp query -i object.yaml 'request.object.metadata.name | truncate(@, `9`)'",
	},
	{
		"# Parse expression",
		"kyverno jp parse 'request.object.metadata.name | truncate(@, `9`)'",
	},
}
