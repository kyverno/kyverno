package parse

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#jp`

var description = []string{
	`Parses jmespath expression and shows corresponding AST.`,
}

var examples = [][]string{
	{
		"# Parse expression",
		"kyverno jp parse 'request.object.metadata.name | truncate(@, `9`)'",
	},
	{
		"# Parse expression from a file",
		"kyverno jp parse -f my-file",
	},
	{
		"# Parse expression from stdin",
		"kyverno jp parse",
	},
	{
		"# Parse multiple expressionxs",
		"kyverno jp parse -f my-file1 -f my-file-2 'request.object.metadata.name | truncate(@, `9`)'",
	},
	{
		"# Cat into",
		"cat my-file | kyverno jp parse",
	},
}
