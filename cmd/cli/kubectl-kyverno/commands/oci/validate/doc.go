package validate

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#pushing`

var description = []string{
	`Validate a bundle directory or file, or a pushed OCI image, reporting exactly what` +
		` "kyverno oci push" would reject, without pushing anything.`,
	`Local mode (a directory or file argument) runs every check push runs up to the upload step: no network call is made.`,
	`Reference mode (--image) fetches a pushed image, re-runs the same strict checks over it, and can fail on content "kyverno oci pull" would accept, such as an exception referencing a policy outside the bundle; use pull to extract such a bundle instead.`,
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
