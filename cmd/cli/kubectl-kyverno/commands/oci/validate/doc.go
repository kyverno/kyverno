package validate

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#oci`

var description = []string{
	`Validate a bundle directory or file, or a pushed OCI image, without pushing anything. By` +
		` default, local validate succeeds exactly when "kyverno oci push" would reach its upload step;` +
		` --kyverno-version and --strict both change that contract (see below).`,
	`Local mode (a directory or file argument) runs every check push runs up to the upload step: no network call is made.`,
	`Reference mode (--image) fetches a pushed image, re-runs the same strict checks over it, and can fail on content "kyverno oci pull" would accept, such as an exception referencing a policy outside the bundle; use pull to extract such a bundle instead.`,
	`--kyverno-version compares the bundle's kyvernoVersion constraint against the given version and fails on a mismatch; a plain push or validate only warns, since they compare against the running CLI's own version.`,
	`--strict additionally fails on conditions push and plain validate only warn about, such as an expired exception or a compensating control ignored for its policy's kind.`,
}

var examples = [][]string{
	{
		`# Validate a local bundle directory before pushing it`,
		`kyverno oci validate ./bundle`,
	},
	{
		`# Validate a local bundle, failing on warnings such as an expired exception`,
		`kyverno oci validate ./bundle --strict`,
	},
	{
		`# Validate a pushed bundle against a specific target Kyverno version`,
		`kyverno oci validate -i <imgref> --kyverno-version 1.20.0`,
	},
}
