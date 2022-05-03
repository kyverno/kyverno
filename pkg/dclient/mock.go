package client

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

//NewMockClient ---testing utilities
func NewMockClient(scheme *runtime.Scheme, gvrToListKind map[schema.GroupVersionResource]string, objects ...runtime.Object) (Interface, error) {
	c := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objects...)
	// the typed and dynamic client are initialized with similar resources
	kclient := kubefake.NewSimpleClientset(objects...)
	return &client{
		client:  c,
		kclient: kclient,
	}, nil
}
