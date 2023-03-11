package policycache

import (
	"fmt"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
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

	podsGVRS                   = dclient.GroupVersionResourceSubresource{GroupVersionResource: podsGVR}
	namespacesGVRS             = dclient.GroupVersionResourceSubresource{GroupVersionResource: namespacesGVR}
	clusterrolesGVRS           = dclient.GroupVersionResourceSubresource{GroupVersionResource: clusterrolesGVR}
	deploymentsGVRS            = dclient.GroupVersionResourceSubresource{GroupVersionResource: deploymentsGVR}
	statefulsetsGVRS           = dclient.GroupVersionResourceSubresource{GroupVersionResource: statefulsetsGVR}
	daemonsetsGVRS             = dclient.GroupVersionResourceSubresource{GroupVersionResource: daemonsetsGVR}
	jobsGVRS                   = dclient.GroupVersionResourceSubresource{GroupVersionResource: jobsGVR}
	cronjobsGVRS               = dclient.GroupVersionResourceSubresource{GroupVersionResource: cronjobsGVR}
	replicasetsGVRS            = dclient.GroupVersionResourceSubresource{GroupVersionResource: replicasetsGVR}
	replicationcontrollersGVRS = dclient.GroupVersionResourceSubresource{GroupVersionResource: replicationcontrollersGVR}
)

type TestResourceFinder struct{}

func (TestResourceFinder) FindResources(group, version, kind, subresource string) ([]dclient.GroupVersionResourceSubresource, error) {
	switch kind {
	case "Pod":
		return []dclient.GroupVersionResourceSubresource{podsGVRS}, nil
	case "Namespace":
		return []dclient.GroupVersionResourceSubresource{namespacesGVRS}, nil
	case "ClusterRole":
		return []dclient.GroupVersionResourceSubresource{clusterrolesGVRS}, nil
	case "Deployment":
		return []dclient.GroupVersionResourceSubresource{deploymentsGVRS}, nil
	case "StatefulSet":
		return []dclient.GroupVersionResourceSubresource{statefulsetsGVRS}, nil
	case "DaemonSet":
		return []dclient.GroupVersionResourceSubresource{daemonsetsGVRS}, nil
	case "ReplicaSet":
		return []dclient.GroupVersionResourceSubresource{replicasetsGVRS}, nil
	case "Job":
		return []dclient.GroupVersionResourceSubresource{jobsGVRS}, nil
	case "ReplicationController":
		return []dclient.GroupVersionResourceSubresource{replicationcontrollersGVRS}, nil
	case "CronJob":
		return []dclient.GroupVersionResourceSubresource{cronjobsGVRS}, nil
	}
	return nil, fmt.Errorf("not found: %s", kind)
}
