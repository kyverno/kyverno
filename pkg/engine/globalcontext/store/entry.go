package store

type Entry interface {
	Get() (interface{}, error)
	Stop()
}
