package id

import (
	"github.com/kyverno/kyverno/test/e2e/framework/gvr"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Clustered(gvr schema.GroupVersionResource, name string) Id {
	return New(gvr, "", name)
}

func ClusterPolicy(name string) Id {
	return Clustered(gvr.ClusterPolicy, name)
}

func ClusterRole(name string) Id {
	return Clustered(gvr.ClusterRole, name)
}

func ClusterRoleBinding(name string) Id {
	return Clustered(gvr.ClusterRoleBinding, name)
}

func Namespace(name string) Id {
	return Clustered(gvr.Namespace, name)
}
