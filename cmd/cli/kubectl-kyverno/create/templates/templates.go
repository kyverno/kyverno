package templates

import (
	_ "embed"
)

//go:embed test.yaml
var TestTemplate string

//go:embed user-info.yaml
var UserInfoTemplate string
