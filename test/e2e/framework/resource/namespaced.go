package resource

import (
	"github.com/kyverno/kyverno/test/e2e/framework/gvr"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Namespaced(gvr schema.GroupVersionResource, ns string, raw []byte) Resource {
	return Resource{gvr, ns, raw}
}

func Role(ns string, raw []byte) Resource {
	return Namespaced(gvr.Role, ns, raw)
}

func RoleBinding(ns string, raw []byte) Resource {
	return Namespaced(gvr.RoleBinding, ns, raw)
}

func ConfigMap(ns string, raw []byte) Resource {
	return Namespaced(gvr.ConfigMap, ns, raw)
}
