package policyviolation

import (
	"fmt"
	"reflect"
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	deployutil "k8s.io/kubernetes/pkg/controller/deployment/util"
)

func converLabelToSelector(labelMap map[string]string) (labels.Selector, error) {
	ls := &metav1.LabelSelector{}
	err := metav1.Convert_Map_string_To_string_To_v1_LabelSelector(&labelMap, ls, nil)
	if err != nil {
		return nil, err
	}

	policyViolationSelector, err := metav1.LabelSelectorAsSelector(ls)
	if err != nil {
		return nil, fmt.Errorf("invalid label selector: %v", err)
	}

	return policyViolationSelector, nil
}

// validDependantForDeployment checks if resource (pod) matches the intent of the given deployment
// explicitly handles deployment-replicaset-pod relationship
func validDependantForDeployment(client appsv1.AppsV1Interface, pvResourceSpec kyverno.ResourceSpec, resource unstructured.Unstructured) bool {
	if resource.GetKind() != "Pod" {
		return false
	}

	// only handles deploymeny-replicaset-pod relationship
	if pvResourceSpec.Kind != "Deployment" {
		return false
	}

	owner := kyverno.ResourceSpec{
		Kind: pvResourceSpec.Kind,
		Name: pvResourceSpec.Name,
	}

	start := time.Now()
	deploy, err := client.Deployments(resource.GetNamespace()).Get(owner.Name, metav1.GetOptions{})
	if err != nil {
		glog.Errorf("failed to get resourceOwner deployment %s/%s/%s: %v", owner.Kind, resource.GetNamespace(), owner.Name, err)
		return false
	}
	glog.V(4).Infof("Time getting deployment %v", time.Since(start))

	// TODO(shuting): replace typed client AppsV1Interface
	expectReplicaset, err := deployutil.GetNewReplicaSet(deploy, client)
	if err != nil {
		glog.Errorf("failed to get replicaset owned by %s/%s/%s: %v", owner.Kind, resource.GetNamespace(), owner.Name, err)
		return false
	}

	if reflect.DeepEqual(expectReplicaset, v1.ReplicaSet{}) {
		glog.V(2).Infof("no replicaset found for deploy %s/%s/%s", resource.GetNamespace(), owner.Kind, owner.Name)
		return false
	}
	var actualReplicaset *v1.ReplicaSet
	for _, podOwner := range resource.GetOwnerReferences() {
		if podOwner.Kind != "ReplicaSet" {
			continue
		}

		actualReplicaset, err = client.ReplicaSets(resource.GetNamespace()).Get(podOwner.Name, metav1.GetOptions{})
		if err != nil {
			glog.Errorf("failed to get replicaset from %s/%s/%s: %v", resource.GetKind(), resource.GetNamespace(), resource.GetName(), err)
			return false
		}

		if reflect.DeepEqual(actualReplicaset, v1.ReplicaSet{}) {
			glog.V(2).Infof("no replicaset found for Pod/%s/%s", resource.GetNamespace(), podOwner.Name)
			return false
		}

		if expectReplicaset.Name == actualReplicaset.Name {
			return true
		}
	}
	return false
}

// GetOwner of a resource by iterating over ownerReferences
func GetOwner(dclient *client.Client, ownerMap map[kyverno.ResourceSpec]interface{}, resource unstructured.Unstructured) {
	var emptyInterface interface{}
	resourceSpec := kyverno.ResourceSpec{
		Kind: resource.GetKind(),
		Name: resource.GetName(),
	}
	if _, ok := ownerMap[resourceSpec]; ok {
		// owner seen before
		// breaking loop
		return
	}
	rOwners := resource.GetOwnerReferences()
	// if there are no resource owners then its top level resource
	if len(rOwners) == 0 {
		// add resource to map
		ownerMap[resourceSpec] = emptyInterface
		return
	}
	for _, rOwner := range rOwners {
		// lookup resource via client
		// owner has to be in same namespace
		owner, err := dclient.GetResource(rOwner.Kind, resource.GetNamespace(), rOwner.Name)
		if err != nil {
			glog.Errorf("Failed to get resource owner for %s/%s/%s, err: %v", rOwner.Kind, resource.GetNamespace(), rOwner.Name, err)
			// as we want to process other owners
			continue
		}
		GetOwner(dclient, ownerMap, *owner)
	}
}
