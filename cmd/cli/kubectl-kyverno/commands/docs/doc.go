package docs

// TODO
var websiteUrl = ``

var description = []string{
	`Generates reference documentation.`,
	``,
	`The docs command generates Kyverno CLI reference documentation.`,
	``,
	`It can be used to generate simple markdown files or markdown to be used for the website.`,
	``,
	`NOTE: This is an experimental command, use "KYVERNO_EXPERIMENTAL=true" to enable it.`,
}

var examples = [][]string{
	{
		`# Generate simple markdown documentation`,
		`KYVERNO_EXPERIMENTAL=true kyverno docs -o . --autogenTag=false`,
	},
	{
		`# Generate website documentation`,
		`KYVERNO_EXPERIMENTAL=true kyverno docs -o . --website`,
	},
}
