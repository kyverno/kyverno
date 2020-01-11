package policyviolation

import (
	"fmt"
	"time"

	backoff "github.com/cenkalti/backoff"
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

func createOwnerReference(resource *unstructured.Unstructured) metav1.OwnerReference {
	controllerFlag := true
	blockOwnerDeletionFlag := true
	ownerRef := metav1.OwnerReference{
		APIVersion:         resource.GetAPIVersion(),
		Kind:               resource.GetKind(),
		Name:               resource.GetName(),
		UID:                resource.GetUID(),
		Controller:         &controllerFlag,
		BlockOwnerDeletion: &blockOwnerDeletionFlag,
	}
	return ownerRef
}

func retryGetResource(client *client.Client, rspec kyverno.ResourceSpec) (*unstructured.Unstructured, error) {
	var i int
	var obj *unstructured.Unstructured
	var err error
	getResource := func() error {
		obj, err = client.GetResource(rspec.Kind, rspec.Namespace, rspec.Name)
		glog.V(4).Infof("retry %v getting %s/%s/%s", i, rspec.Kind, rspec.Namespace, rspec.Name)
		i++
		return err
	}

	exbackoff := &backoff.ExponentialBackOff{
		InitialInterval:     500 * time.Millisecond,
		RandomizationFactor: 0.5,
		Multiplier:          1.5,
		MaxInterval:         time.Second,
		MaxElapsedTime:      3 * time.Second,
		Clock:               backoff.SystemClock,
	}

	exbackoff.Reset()
	err = backoff.Retry(getResource, exbackoff)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

// GetOwners returns a list of owners
func GetOwners(dclient *client.Client, resource unstructured.Unstructured) []kyverno.ResourceSpec {
	ownerMap := map[kyverno.ResourceSpec]interface{}{}
	GetOwner(dclient, ownerMap, resource)
	var owners []kyverno.ResourceSpec
	for owner := range ownerMap {
		owners = append(owners, owner)
	}
	return owners
}

// GetOwner of a resource by iterating over ownerReferences
func GetOwner(dclient *client.Client, ownerMap map[kyverno.ResourceSpec]interface{}, resource unstructured.Unstructured) {
	var emptyInterface interface{}
	resourceSpec := kyverno.ResourceSpec{
		Kind:      resource.GetKind(),
		Namespace: resource.GetNamespace(),
		Name:      resource.GetName(),
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
