package resources

import (
	_ "embed"
)

//go:embed default-config.yaml
var DefaultConfigBytes []byte
