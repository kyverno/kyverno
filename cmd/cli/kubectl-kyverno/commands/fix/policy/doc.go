package policy

// TODO
var websiteUrl = ``

var description = []string{
	`Fix inconsistencies and deprecated usage in Kyverno policy files.`,
}

var examples = [][]string{
	{
		`# Fix Kyverno policy files`,
		`KYVERNO_EXPERIMENTAL=true kyverno fix policy .`,
	},
	{
		`# Fix Kyverno policy files and save them back`,
		`KYVERNO_EXPERIMENTAL=true kyverno fix policy . --save`,
	},
}
