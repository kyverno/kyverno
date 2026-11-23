package resolve

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#oci`

var description = []string{
	`Resolve a bundle tag to its immutable digest and print a digest-pinned reference you can pass to "kyverno test", "kyverno apply", "kyverno oci pull", or a Flux OCIRepository.`,
	`The command fetches the manifest and config, checks that it is a Kyverno bundle, and prints exactly one line to stdout; it never downloads the content layer. Use --digest-only for the bare digest.`,
}

var examples = [][]string{
	{
		`# Resolve a tag to its digest-pinned reference`,
		`kyverno oci resolve -i ghcr.io/acme/baseline:v1.0.0`,
	},
	{
		`# Run kyverno test against the resolved, digest-pinned bundle`,
		`kyverno test oci://$(kyverno oci resolve -i ghcr.io/acme/baseline:v1.0.0)`,
	},
	{
		`# Print only the bare digest`,
		`kyverno oci resolve -i ghcr.io/acme/baseline:v1.0.0 --digest-only`,
	},
}
