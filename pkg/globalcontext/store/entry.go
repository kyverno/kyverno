package store

type Entry interface {
	Get() (any, error)
	Stop()
}
