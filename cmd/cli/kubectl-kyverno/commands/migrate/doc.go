package migrate

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#migrate`

var description = []string{
	`Migrate one or more resources to the stored version.`,
}

var examples = [][]string{
	{
		`# Migrate policy exceptions`,
		`kyverno migrate --resource policyexceptions.kyverno.io`,
	},
}
