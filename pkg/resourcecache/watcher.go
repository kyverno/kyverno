package resourcecache

import (
	"k8s.io/client-go/tools/cache"
)

func startWatching(stopCh <-chan struct{}, s cache.SharedIndexInformer) {
	s.Run(stopCh)
}
