package context

type ConfigMapReference struct {
	Name      string `cel:"name"`
	Namespace string `cel:"namespace"`
}
