package userinfo

var websiteUrl = `https://kyverno.io/docs/kyverno-cli/#create`

var description = []string{
	`Create a Kyverno user-info file.`,
}

var examples = [][]string{
	{
		"# Create user info file",
		"kyverno create user-info -u molybdenum@somecorp.com -g basic-user -c admin",
	},
}
