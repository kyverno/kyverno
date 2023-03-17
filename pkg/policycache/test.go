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

func (TestResourceFinder) FindResources(group, version, kind, subresource string) (map[dclient.GroupVersionResourceSubresource]metav1.APIResource, error) {
	var dummy metav1.APIResource
	switch kind {
	case "Pod":
		return map[dclient.GroupVersionResourceSubresource]metav1.APIResource{podsGVRS: dummy}, nil
	case "Namespace":
		return map[dclient.GroupVersionResourceSubresource]metav1.APIResource{namespacesGVRS: dummy}, nil
	case "ClusterRole":
		return map[dclient.GroupVersionResourceSubresource]metav1.APIResource{clusterrolesGVRS: dummy}, nil
	case "Deployment":
		return map[dclient.GroupVersionResourceSubresource]metav1.APIResource{deploymentsGVRS: dummy}, nil
	case "StatefulSet":
		return map[dclient.GroupVersionResourceSubresource]metav1.APIResource{statefulsetsGVRS: dummy}, nil
	case "DaemonSet":
		return map[dclient.GroupVersionResourceSubresource]metav1.APIResource{daemonsetsGVRS: dummy}, nil
	case "ReplicaSet":
		return map[dclient.GroupVersionResourceSubresource]metav1.APIResource{replicasetsGVRS: dummy}, nil
	case "Job":
		return map[dclient.GroupVersionResourceSubresource]metav1.APIResource{jobsGVRS: dummy}, nil
	case "ReplicationController":
		return map[dclient.GroupVersionResourceSubresource]metav1.APIResource{replicationcontrollersGVRS: dummy}, nil
	case "CronJob":
		return map[dclient.GroupVersionResourceSubresource]metav1.APIResource{cronjobsGVRS: dummy}, nil
	}
	return nil, fmt.Errorf("not found: %s", kind)
}
