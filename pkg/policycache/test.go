package policycache

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	podsGVR                   = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	namespacesGVR             = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	clusterrolesGVR           = schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"}
	deploymentsGVR            = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	statefulsetsGVR           = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}
	daemonsetsGVR             = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}
	jobsGVR                   = schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}
	cronjobsGVR               = schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}
	replicasetsGVR            = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}
	replicationcontrollersGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "replicationcontrollers"}
)

type TestResourceFinder struct {
}

func (TestResourceFinder) FindResources(group, version, kind, subresource string) ([]schema.GroupVersionResource, error) {
	switch kind {
	case "Pod":
		return []schema.GroupVersionResource{podsGVR}, nil
	case "Namespace":
		return []schema.GroupVersionResource{namespacesGVR}, nil
	case "ClusterRole":
		return []schema.GroupVersionResource{clusterrolesGVR}, nil
	case "Deployment":
		return []schema.GroupVersionResource{deploymentsGVR}, nil
	case "StatefulSet":
		return []schema.GroupVersionResource{statefulsetsGVR}, nil
	case "DaemonSet":
		return []schema.GroupVersionResource{daemonsetsGVR}, nil
	case "ReplicaSet":
		return []schema.GroupVersionResource{replicasetsGVR}, nil
	case "Job":
		return []schema.GroupVersionResource{jobsGVR}, nil
	case "ReplicationController":
		return []schema.GroupVersionResource{replicationcontrollersGVR}, nil
	case "CronJob":
		return []schema.GroupVersionResource{cronjobsGVR}, nil
	}
	return nil, fmt.Errorf("not found: %s", kind)
}
