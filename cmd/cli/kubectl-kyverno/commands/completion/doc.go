package completion

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#shell-autocompletion`

var description = []string{
	`Generate the autocompletion script for kyverno for the specified shell.`,
	`See each sub-command's help for details on how to use the generated script.`,
}

var examples = [][]string{
	{
		"# Generate the autocompletion script for bash",
		"kyverno completion bash",
	},
	{
		"# Generate the autocompletion script for zsh",
		"kyverno completion zsh",
	},
	{
		"# Generate the autocompletion script for fish",
		"kyverno completion fish",
	},
	{
		"# Generate the autocompletion script for powershell",
		"kyverno completion powershell",
	},
}
