package test

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#fix`

var description = []string{
	`Fix inconsistencies and deprecated usage in Kyverno test files.`,
}

var examples = [][]string{
	{
		`# Fix Kyverno test files`,
		`KYVERNO_EXPERIMENTAL=true kyverno fix test .`,
	},
	{
		`# Fix Kyverno test files and save them back`,
		`KYVERNO_EXPERIMENTAL=true kyverno fix test . --save`,
	},
	{
		`# Fix Kyverno test files with a specific file name`,
		`KYVERNO_EXPERIMENTAL=true kyverno fix test . --file-name test.yaml --save`,
	},
}
