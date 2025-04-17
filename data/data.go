package data

import (
	_ "embed"
)

//go:embed swagger.json
var SwaggerDoc string

//go:embed preferred-resources.json
var PreferredAPIResourceLists string

//go:embed api-resources.json
var APIResourceLists string
