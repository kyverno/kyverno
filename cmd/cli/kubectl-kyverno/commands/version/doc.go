package version

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#version`

var description = []string{
	`Prints the version of Kyverno CLI.`,
}

var examples = [][]string{
	{
		`# Print Kyverno CLI version`,
		`kyverno version`,
	},
	{
		`# Print Kyverno CLI version in JSON format`,
		`kyverno version -o json`,
	},
	{
		`# Print Kyverno CLI version in YAML format`,
		`kyverno version --output yaml`,
	},
}
