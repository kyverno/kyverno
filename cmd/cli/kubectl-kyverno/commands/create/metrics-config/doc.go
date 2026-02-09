package metricsconfig

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#create`

var description = []string{
	`Create a Kyverno metrics-config file.`,
}

var examples = [][]string{
	{
		"# Create metrics config file",
		"kyverno create metrics-config -i ns-included-1 -i ns-included-2 -e ns-excluded",
	},
}
