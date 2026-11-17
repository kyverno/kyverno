package oci

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#oci`

var description = []string{
	`Pushes, pulls, validates, inspects, and resolves Kyverno CEL Policy Bundles in OCI registries. See "kyverno oci <subcommand> --help" for each subcommand's own flags and examples.`,
}

var examples = [][]string{
	{
		`# Push a policy bundle from a directory to an OCI image`,
		`kyverno oci push policies -i ghcr.io/acme/baseline:v1.0.0`,
	},
	{
		`# Pull a pushed bundle into a directory`,
		`kyverno oci pull policies -i ghcr.io/acme/baseline:v1.0.0`,
	},
	{
		`# Validate a pushed bundle without pulling it`,
		`kyverno oci validate -i ghcr.io/acme/baseline:v1.0.0`,
	},
	{
		`# Inspect a pushed bundle's metadata and Kyverno compatibility`,
		`kyverno oci inspect -i ghcr.io/acme/baseline:v1.0.0`,
	},
	{
		`# Resolve a tag to its digest-pinned reference`,
		`kyverno oci resolve -i ghcr.io/acme/baseline:v1.0.0`,
	},
}
