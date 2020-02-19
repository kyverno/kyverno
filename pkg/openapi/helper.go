package openapi

func GetDefinitionNameFromKind(kind string) string {
	return openApiGlobalState.kindToDefinitionName[kind]
}
