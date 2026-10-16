package fix

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#fix`

var description = []string{
	`Fix inconsistencies and deprecated usage of Kyverno resources.`,
	``,
	`The fix command provides a command-line interface to fix inconsistencies and deprecated usage of Kyverno resources.`,
	`It can be used to fix Kyverno test files.`,
	``,
	`Note: "kyverno fix policy" (for legacy kyverno.io ClusterPolicy/Policy resources) was removed in 1.20`,
	`along with legacy policy execution; there is no replacement, since Kyverno no longer supports legacy`,
	`policy authoring. See https://kyverno.io/docs/guides/migration-to-cel/ for migrating to CEL policies.`,
}

var examples = [][]string{
	{
		`# Fix Kyverno test files`,
		`KYVERNO_EXPERIMENTAL=true kyverno fix test . --save`,
	},
}
