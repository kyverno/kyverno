package policycache

import (
	"fmt"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	podsGVRS                   = mapGVR(podsGVR, "Pod")
	namespacesGVRS             = mapGVR(namespacesGVR, "Namespace")
	clusterrolesGVRS           = mapGVR(clusterrolesGVR, "ClusterRole")
	deploymentsGVRS            = mapGVR(deploymentsGVR, "Deployment")
	statefulsetsGVRS           = mapGVR(statefulsetsGVR, "StatefulSet")
	daemonsetsGVRS             = mapGVR(daemonsetsGVR, "DaemonSet")
	jobsGVRS                   = mapGVR(jobsGVR, "Jon")
	cronjobsGVRS               = mapGVR(cronjobsGVR, "CronJob")
	replicasetsGVRS            = mapGVR(replicasetsGVR, "ReplicaSet")
	replicationcontrollersGVRS = mapGVR(replicationcontrollersGVR, "ReplicationController")
)

func mapGVR(gvr schema.GroupVersionResource, kind string) dclient.TopLevelApiDescription {
	return dclient.TopLevelApiDescription{
		GroupVersion: gvr.GroupVersion(),
		Kind:         kind,
		Resource:     gvr.Resource,
	}
}

type TestResourceFinder struct{}

func (TestResourceFinder) FindResources(group, version, kind, subresource string) (map[dclient.TopLevelApiDescription]metav1.APIResource, error) {
	var dummy metav1.APIResource
	switch kind {
	case "Pod":
		return map[dclient.TopLevelApiDescription]metav1.APIResource{podsGVRS: dummy}, nil
	case "Namespace":
		return map[dclient.TopLevelApiDescription]metav1.APIResource{namespacesGVRS: dummy}, nil
	case "ClusterRole":
		return map[dclient.TopLevelApiDescription]metav1.APIResource{clusterrolesGVRS: dummy}, nil
	case "Deployment":
		return map[dclient.TopLevelApiDescription]metav1.APIResource{deploymentsGVRS: dummy}, nil
	case "StatefulSet":
		return map[dclient.TopLevelApiDescription]metav1.APIResource{statefulsetsGVRS: dummy}, nil
	case "DaemonSet":
		return map[dclient.TopLevelApiDescription]metav1.APIResource{daemonsetsGVRS: dummy}, nil
	case "ReplicaSet":
		return map[dclient.TopLevelApiDescription]metav1.APIResource{replicasetsGVRS: dummy}, nil
	case "Job":
		return map[dclient.TopLevelApiDescription]metav1.APIResource{jobsGVRS: dummy}, nil
	case "ReplicationController":
		return map[dclient.TopLevelApiDescription]metav1.APIResource{replicationcontrollersGVRS: dummy}, nil
	case "CronJob":
		return map[dclient.TopLevelApiDescription]metav1.APIResource{cronjobsGVRS: dummy}, nil
	}
	return nil, fmt.Errorf("not found: %s", kind)
}
