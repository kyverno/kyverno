package utils

type Named interface {
	GetNamespace() string
	GetName() string
}

func Key(obj Named) string {
	n, ns := obj.GetName(), obj.GetNamespace()
	if ns == "" {
		return n
	}
	return ns + "/" + n
}
