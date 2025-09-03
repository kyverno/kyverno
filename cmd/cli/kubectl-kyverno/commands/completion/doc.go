package completion

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#shell-autocompletion`

var description = []string{
	`Generate the autocompletion script for kyverno for the specified shell.`,
	``,
	`Shell autocompletion enables tab completion for kyverno commands, subcommands, flags, and arguments.`,
	`This significantly improves CLI usability by providing command suggestions and reducing typing.`,
	``,
	`The generated script contains shell-specific functions that integrate with your shell's`,
	`completion system to provide intelligent command completion when you press the Tab key.`,
	``,
	`To enable autocompletion, source the generated script in your shell profile or save it`,
	`to your shell's completion directory. See the examples below for shell-specific instructions.`,
}

var examples = [][]string{
	{
		"# Generate and install bash completion (Linux)",
		"kyverno completion bash > /etc/bash_completion.d/kyverno",
	},
	{
		"# Generate and source bash completion for current session",
		"source <(kyverno completion bash)",
	},
	{
		"# Generate and install zsh completion",
		"kyverno completion zsh > \"${fpath[1]}/_kyverno\"",
	},
	{
		"# Generate and source zsh completion for current session",
		"source <(kyverno completion zsh)",
	},
	{
		"# Generate and install fish completion",
		"kyverno completion fish > ~/.config/fish/completions/kyverno.fish",
	},
	{
		"# Generate PowerShell completion",
		"kyverno completion powershell | Out-String | Invoke-Expression",
	},
	{
		"# To permanently enable PowerShell completion, add to your profile:",
		"kyverno completion powershell >> $PROFILE",
	},
}
