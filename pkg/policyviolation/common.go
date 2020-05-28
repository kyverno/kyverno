package policyviolation

import (
	"fmt"
	"time"

	backoff "github.com/cenkalti/backoff"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func createOwnerReference(resource *unstructured.Unstructured) (metav1.OwnerReference, bool) {
	controllerFlag := true
	blockOwnerDeletionFlag := true

	apiversion := resource.GetAPIVersion()
	kind := resource.GetKind()
	name := resource.GetName()
	uid := resource.GetUID()

	if apiversion == "" || kind == "" || name == "" || uid == "" {
		return metav1.OwnerReference{}, false
	}

	ownerRef := metav1.OwnerReference{
		APIVersion:         resource.GetAPIVersion(),
		Kind:               resource.GetKind(),
		Name:               resource.GetName(),
		UID:                resource.GetUID(),
		Controller:         &controllerFlag,
		BlockOwnerDeletion: &blockOwnerDeletionFlag,
	}
	return ownerRef, true
}

func retryGetResource(client *client.Client, rspec kyverno.ResourceSpec) (*unstructured.Unstructured, error) {
	var i int
	var obj *unstructured.Unstructured
	var err error
	getResource := func() error {
		obj, err = client.GetResource(rspec.Kind, rspec.Namespace, rspec.Name)
		log.Log.V(4).Info(fmt.Sprintf("retry %v getting %s/%s/%s", i, rspec.Kind, rspec.Namespace, rspec.Name))
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

type violationCount struct {
	policyName    string
	violatedRules []v1.ViolatedRule
}

func (vc violationCount) PolicyName() string {
	return vc.policyName
}

func (vc violationCount) UpdateStatus(status kyverno.PolicyStatus) kyverno.PolicyStatus {

	var ruleNameToViolations = make(map[string]int)
	for _, rule := range vc.violatedRules {
		ruleNameToViolations[rule.Name]++
	}

	for i := range status.Rules {
		status.ViolationCount += ruleNameToViolations[status.Rules[i].Name]
		status.Rules[i].ViolationCount += ruleNameToViolations[status.Rules[i].Name]
	}

	return status
}
