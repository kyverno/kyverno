package fix

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#fix`

var description = []string{
	`Fix inconsistencies and deprecated usage of Kyverno resources.`,
	``,
	`The fix command provides a command-line interface to fix inconsistencies and deprecated usage of Kyverno resources.`,
	`It can be used to fix Kyverno test files.`,
}

var examples = [][]string{
	{
		`# Fix Kyverno test files`,
		`KYVERNO_EXPERIMENTAL=true kyverno fix test . --save`,
	},
}
