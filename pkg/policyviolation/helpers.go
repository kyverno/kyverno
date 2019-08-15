package policyviolation

import (
	"fmt"
	"reflect"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/clientNew/clientset/versioned"
	lister "github.com/nirmata/kyverno/pkg/clientNew/listers/kyverno/v1alpha1"
	"github.com/nirmata/kyverno/pkg/info"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

//BuildPolicyViolation returns an value of type PolicyViolation
func BuildPolicyViolation(policy string, resource kyverno.ResourceSpec, fRules []kyverno.ViolatedRule) kyverno.PolicyViolation {
	pv := kyverno.PolicyViolation{
		Spec: kyverno.PolicyViolationSpec{
			Policy:        policy,
			ResourceSpec:  resource,
			ViolatedRules: fRules,
		},
	}
	//TODO: check if this can be removed or use unstructured?
	// pv.Kind = "PolicyViolation"
	pv.SetGenerateName("pv-")
	return pv
}

// buildPolicyViolationsForAPolicy returns a policy violation object if there are any rules that fail
func buildPolicyViolationsForAPolicy(pi info.PolicyInfo) kyverno.PolicyViolation {
	var fRules []kyverno.ViolatedRule
	var pv kyverno.PolicyViolation
	for _, r := range pi.Rules {
		if !r.IsSuccessful() {
			fRules = append(fRules, kyverno.ViolatedRule{Name: r.Name, Message: r.GetErrorString(), Type: r.RuleType.String()})
		}
	}
	if len(fRules) > 0 {
		glog.V(4).Infof("building policy violation for policy %s on resource %s/%s/%s", pi.Name, pi.RKind, pi.RNamespace, pi.RName)
		// there is an error
		pv = BuildPolicyViolation(pi.Name, kyverno.ResourceSpec{
			Kind:      pi.RKind,
			Namespace: pi.RNamespace,
			Name:      pi.RName,
		},
			fRules,
		)

	}
	return pv
}

//generatePolicyViolations generate policyViolation resources for the rules that failed
//TODO: check if pvListerSynced is needed
func GeneratePolicyViolations(pvListerSynced cache.InformerSynced, pvLister lister.PolicyViolationLister, client *kyvernoclient.Clientset, policyInfos []info.PolicyInfo) {
	var pvs []kyverno.PolicyViolation
	for _, policyInfo := range policyInfos {
		if !policyInfo.IsSuccessful() {
			if pv := buildPolicyViolationsForAPolicy(policyInfo); !reflect.DeepEqual(pv, kyverno.PolicyViolation{}) {
				pvs = append(pvs, pv)
			}
		}
	}

	if len(pvs) > 0 {
		for _, newPv := range pvs {
			// generate PolicyViolation objects
			glog.V(4).Infof("creating policyViolation resource for policy %s and resource %s/%s/%s", newPv.Spec.Policy, newPv.Spec.Kind, newPv.Spec.Namespace, newPv.Spec.Name)

			// check if there was a previous violation for policy & resource combination
			curPv, err := getExistingPolicyViolationIfAny(pvListerSynced, pvLister, newPv)
			if err != nil {
				continue
			}
			if curPv == nil {
				// no existing policy violation, create a new one
				_, err := client.KyvernoV1alpha1().PolicyViolations().Create(&newPv)
				if err != nil {
					glog.Error(err)
				}
				continue
			}
			// compare the policyviolation spec for existing resource if present else
			if reflect.DeepEqual(curPv.Spec, newPv.Spec) {
				// if they are equal there has been no change so dont update the polivy violation
				glog.Infof("policy violation spec %v did not change so not updating it", newPv.Spec)
				continue
			}
			// spec changed so update the policyviolation
			//TODO: wont work, as name is not defined yet
			_, err = client.KyvernoV1alpha1().PolicyViolations().Update(&newPv)
			if err != nil {
				glog.Error(err)
				continue
			}
		}
	}
}

//TODO: change the name
func getExistingPolicyViolationIfAny(pvListerSynced cache.InformerSynced, pvLister lister.PolicyViolationLister, newPv kyverno.PolicyViolation) (*kyverno.PolicyViolation, error) {
	// TODO: check for existing ov using label selectors on resource and policy
	labelMap := map[string]string{"policy": newPv.Spec.Policy, "resource": newPv.Spec.ResourceSpec.ToKey()}
	ls := &metav1.LabelSelector{}
	err := metav1.Convert_Map_string_To_string_To_v1_LabelSelector(&labelMap, ls, nil)
	if err != nil {
		glog.Errorf("failed to generate label sector of Policy name %s: %v", newPv.Spec.Policy, err)
		return nil, err
	}
	policyViolationSelector, err := metav1.LabelSelectorAsSelector(ls)
	if err != nil {
		glog.Errorf("invalid label selector: %v", err)
		return nil, err
	}

	//TODO: sync the cache before reading from it ?
	// check is this is needed ?
	// stopCh := make(chan struct{}, 0)
	// if !cache.WaitForCacheSync(stopCh, pvListerSynced) {
	// 	//TODO: can this be handled or avoided ?
	// 	glog.Info("unable to sync policy violation shared informer cache, might be out of sync")
	// }

	pvs, err := pvLister.List(policyViolationSelector)
	if err != nil {
		glog.Errorf("unable to list policy violations with label selector %v: %v", policyViolationSelector, err)
		return nil, err
	}
	//TODO: ideally there should be only one policy violation returned
	if len(pvs) > 1 {
		glog.Errorf("more than one policy violation exists  with labels %v", labelMap)
		return nil, fmt.Errorf("more than one policy violation exists  with labels %v", labelMap)
	}

	if len(pvs) == 0 {
		glog.Infof("policy violation does not exist with labels %v", labelMap)
		return nil, nil
	}
	return pvs[0], nil
}
