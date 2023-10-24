package templates

import (
	_ "embed"
)

//go:embed test.yaml
var TestTemplate string

//go:embed values.yaml
var ValuesTemplate string

//go:embed user-info.yaml
var UserInfoTemplate string

//go:embed exception.yaml
var ExceptionTemplate string

//go:embed metrics-config.yaml
var MetricsConfigTemplate string
