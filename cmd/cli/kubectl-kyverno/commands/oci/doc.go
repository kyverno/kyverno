package oci

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#oci`

var description = []string{
	`Pulls/pushes images that include policie(s) from/to OCI registries.`,
}

var examples = [][]string{
	{
		`# push policy to an OCI image from a given policy file`,
		`kyverno oci push -p policy.yaml -i <imgref>`,
	},
	{
		`# pull policy from an OCI image and save it to the specific directory`,
		`kyverno oci pull -i <imgref> -d policies`,
	},
}
