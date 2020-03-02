package openapi

func GetDefinitionNameFromKind(kind string) string {
	openApiGlobalState.mutex.RLock()
	defer openApiGlobalState.mutex.RUnlock()
	return openApiGlobalState.kindToDefinitionName[kind]
}
