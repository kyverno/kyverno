package resourcecache

import (
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

// TODO :- Implementation for mocking

type TestGVRCache struct {
}

func NewTestGVRCache() GenericCache {
	return &genericCache{}
}

func (tg *TestGVRCache) StopInformer() {

}
func (tg *TestGVRCache) IsNamespaced() bool {
	return true
}

func (tg *TestGVRCache) GetLister() cache.GenericLister {
	return &TestLister{}
}
func (tg *TestGVRCache) GetNamespacedLister(namespace string) cache.GenericNamespaceLister {
	return &TestLister{}
}

type TestLister struct {
}

func (tl *TestLister) List(selector labels.Selector) ([]runtime.Object, error) {
	return []runtime.Object{}, nil
}

func (tl *TestLister) Get(name string) (runtime.Object, error) {
	return nil, nil
}

func (tl *TestLister) ByNamespace(namespace string) cache.GenericNamespaceLister {
	return &TestLister{}
}
