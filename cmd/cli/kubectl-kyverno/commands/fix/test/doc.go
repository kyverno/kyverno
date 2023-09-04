package test

// TODO
var websiteUrl = ``

var description = []string{
	`Fix inconsistencies and deprecated usage in Kyverno test files.`,
	``,
	`NOTE: This is an experimental command, use "KYVERNO_EXPERIMENTAL=true" to enable it.`,
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
