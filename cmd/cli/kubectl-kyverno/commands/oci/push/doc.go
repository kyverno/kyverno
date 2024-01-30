package push

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#pushing`

var description = []string{
	`Push policie(s) that are included in an OCI image to OCI registry.`,
}

var examples = [][]string{
	{
		`# Push policy to an OCI image from a given policy file`,
		`kyverno oci push ./policy.yaml -i <imgref>`,
	},
	{
		`# Push multiple policies to an OCI image from a given directory that includes policies`,
		`kyverno oci push . -i <imgref>`,
	},
}
