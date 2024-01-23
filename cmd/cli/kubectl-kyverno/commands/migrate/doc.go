package migrate

// TODO
var websiteUrl = ``

var description = []string{
	`Migrate one or more resources to the stored version.`,
}

var examples = [][]string{
	{
		`# Migrate policy exceptions`,
		`kyverno migrate --resource policyexceptions.kyverno.io`,
	},
}
