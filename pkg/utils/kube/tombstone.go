package kube

import "k8s.io/client-go/tools/cache"

func GetObjectWithTombstone(obj interface{}) interface{} {
	tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
	if ok {
		return tombstone.Obj
	}
	return obj
}
