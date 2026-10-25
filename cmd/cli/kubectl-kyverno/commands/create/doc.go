package create

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#create`

var description = []string{
	`Helps with the creation of various Kyverno resources.`,
	``,
	`Note: "kyverno create exception" (which scaffolded a legacy kyverno.io/v2 PolicyException) was removed`,
	`in 1.20 along with legacy policy execution; there is no scaffolding replacement. See the`,
	`policies.kyverno.io PolicyException docs at https://kyverno.io/docs/guides/exceptions/`,
	`for the CEL PolicyException shape.`,
}

var examples = [][]string{
	{
		"# Create metrics config file",
		"kyverno create metrics-config -i ns-included-1 -i ns-included-2 -e ns-excluded",
	},
	{
		"# Create test file",
		"kyverno create test -p policy.yaml -r resource.yaml -f values.yaml --pass policy-name,rule-name,resource-name,resource-namespace,resource-kind",
	},
	{
		"# Create user info file",
		"kyverno create user-info -u molybdenum@somecorp.com -g basic-user -c admin",
	},
	{
		"# Create values file",
		"kyverno create values -g request.mode=dev -n prod,env=prod --rule policy,rule,env=demo --resource policy,resource,env=demo",
	},
}
