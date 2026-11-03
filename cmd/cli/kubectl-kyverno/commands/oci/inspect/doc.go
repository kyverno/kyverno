package inspect

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#oci`

var description = []string{
	`Show a pushed bundle's resolved digest, resource index, sets, source and revision metadata, manifest annotations, and Kyverno compatibility.`,
	`The command reads the manifest and config blob only; it doesn't download or verify the content layer. Use "kyverno oci validate -i" to re-run the full validation over the archive.`,
	`--output json or yaml prints the same information in a stable, scriptable form; the config section is the config blob verbatim.`,
}

var examples = [][]string{
	{
		`# Inspect a pushed bundle`,
		`kyverno oci inspect -i ghcr.io/acme/baseline:v1.0.0`,
	},
	{
		`# Inspect a digest-pinned bundle as JSON`,
		`kyverno oci inspect -i ghcr.io/acme/baseline@sha256:... -o json`,
	},
	{
		`# Check compatibility against a specific target Kyverno version`,
		`kyverno oci inspect -i ghcr.io/acme/baseline:v1.0.0 --kyverno-version 1.20.0`,
	},
}
