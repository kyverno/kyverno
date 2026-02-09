package imagedata

type ContextMock struct {
	GetImageDataFunc func(string) (map[string]any, error)
}

func (mock *ContextMock) GetImageData(n string) (map[string]any, error) {
	return mock.GetImageDataFunc(n)
}
