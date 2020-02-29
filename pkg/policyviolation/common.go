package policyviolation

import (
	"fmt"
	"time"

	"github.com/nirmata/kyverno/pkg/policyStatus"

	backoff "github.com/cenkalti/backoff"
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
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

func updatePolicyStatusWithViolationCount(policyName string, violatedRules []kyverno.ViolatedRule) *violationCount {
	return &violationCount{
		policyName:    policyName,
		violatedRules: violatedRules,
	}
}

func (vc *violationCount) UpdateStatus(s *policyStatus.Sync) {
	s.Cache.Mutex.Lock()
	status, exist := s.Cache.Data[vc.policyName]
	if !exist {
		policy, _ := s.PolicyStore.Get(vc.policyName)
		if policy != nil {
			status = policy.Status
		}
	}

	var ruleNameToViolations = make(map[string]int)
	for _, rule := range vc.violatedRules {
		ruleNameToViolations[rule.Name]++
	}

	for i := range status.Rules {
		status.ViolationCount += ruleNameToViolations[status.Rules[i].Name]
		status.Rules[i].ViolationCount += ruleNameToViolations[status.Rules[i].Name]
	}

	s.Cache.Data[vc.policyName] = status
	s.Cache.Mutex.Unlock()
}
