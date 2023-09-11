package query

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#jp`

var description = []string{
	`Provides a command-line interface to JMESPath, enhanced with Kyverno specific custom functions.`,
}

var examples = [][]string{
	{
		"# Evaluate query",
		"kyverno jp query -i object.yaml 'request.object.metadata.name | truncate(@, `9`)'",
	},
	{
		"# Evaluate query",
		"kyverno jp query -i object.yaml -q query-file",
	},
	{
		"# Evaluate multiple queries",
		"kyverno jp query -i object.yaml -q query-file-1 -q query-file-2 'request.object.metadata.name | truncate(@, `9`)'",
	},
	{
		"# Cat query into",
		"cat query-file | kyverno jp query -i object.yaml",
	},
	{
		"# Cat object into",
		"cat object.yaml | kyverno jp query -q query-file",
	},
}
