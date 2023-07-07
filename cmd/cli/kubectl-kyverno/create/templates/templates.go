package templates

import (
	_ "embed"
)

//go:embed test.yaml
var TestTemplate string

//go:embed values.yaml
var ValuesTemplate string
