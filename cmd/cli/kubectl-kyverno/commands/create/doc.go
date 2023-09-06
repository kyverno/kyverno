package create

// TODO
var websiteUrl = ``

var description = []string{
	`Helps with the creation of various Kyverno resources.`,
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
