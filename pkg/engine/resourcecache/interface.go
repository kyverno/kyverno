package resourcecache

type CacheEntry interface {
	Get() ([]byte, error)
}
