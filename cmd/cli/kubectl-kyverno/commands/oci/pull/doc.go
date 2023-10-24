package pull

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#pulling`

var description = []string{
	`Pulls policie(s) that are included in an OCI image from OCI registry and saves them to a local directory.`,
}

var examples = [][]string{
	{
		`# Pull policy from an OCI image and save it to the specific directory`,
		`kyverno oci pull . -i <imgref>`,
	},
}
