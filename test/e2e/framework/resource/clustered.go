package resource

import (
	"github.com/kyverno/kyverno/test/e2e/framework/gvr"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Clustered(gvr schema.GroupVersionResource, raw []byte) Resource {
	return Resource{gvr, "", raw}
}

func ClusterPolicy(raw []byte) Resource {
	return Clustered(gvr.ClusterPolicy, raw)
}

func ClusterRole(raw []byte) Resource {
	return Clustered(gvr.ClusterRole, raw)
}

func ClusterRoleBinding(raw []byte) Resource {
	return Clustered(gvr.ClusterRoleBinding, raw)
}

func Namespace(raw []byte) Resource {
	return Clustered(gvr.Namespace, raw)
}
