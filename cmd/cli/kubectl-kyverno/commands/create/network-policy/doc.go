package networkpolicy

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#create`

var description = []string{
	`Create a default NetworkPolicy for Kyverno.`,
	`This generates the default network policy manifest for Kyverno.`,
}

var examples = [][]string{
	{
		"# Create a network policy and save it to a file",
		"kyverno create network-policy > kyverno-netpol.yaml",
	},
	{
		"# Print the network policy to stdout",
		"kyverno create network-policy",
	},
}
