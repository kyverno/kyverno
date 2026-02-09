package generator

type ContextMock struct {
	GenerateResourcesFunc func(string, []map[string]any) error
}

func (mock *ContextMock) GenerateResources(namespace string, dataList []map[string]any) error {
	return mock.GenerateResourcesFunc(namespace, dataList)
}
