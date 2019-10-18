package policyviolation

import (
	"fmt"
	"reflect"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1alpha1"
	"github.com/nirmata/kyverno/pkg/engine"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

//BuildPolicyViolation returns an value of type PolicyViolation
func BuildPolicyViolation(policy string, resource kyverno.ResourceSpec, fRules []kyverno.ViolatedRule) kyverno.ClusterPolicyViolation {
	pv := kyverno.ClusterPolicyViolation{
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

func buildPVForPolicy(er engine.EngineResponse) kyverno.ClusterPolicyViolation {
	var violatedRules []kyverno.ViolatedRule
	glog.V(4).Infof("building policy violation for engine response %v", er)
	for _, r := range er.PolicyResponse.Rules {
		// filter failed/violated rules
		if !r.Success {
			vrule := kyverno.ViolatedRule{
				Name:    r.Name,
				Message: r.Message,
				Type:    r.Type,
			}
			violatedRules = append(violatedRules, vrule)
		}
	}
	pv := BuildPolicyViolation(er.PolicyResponse.Policy,
		kyverno.ResourceSpec{
			Kind:      er.PolicyResponse.Resource.Kind,
			Namespace: er.PolicyResponse.Resource.Namespace,
			Name:      er.PolicyResponse.Resource.Name,
		},
		violatedRules,
	)
	return pv
}

//CreatePV creates policy violation resource based on the engine responses
func CreatePV(pvLister kyvernolister.ClusterPolicyViolationLister, client *kyvernoclient.Clientset, engineResponses []engine.EngineResponse) {
	var pvs []kyverno.ClusterPolicyViolation
	for _, er := range engineResponses {
		// ignore creation of PV for resoruces that are yet to be assigned a name
		if er.PolicyResponse.Resource.Name == "" {
			glog.V(4).Infof("resource %v, has not been assigned a name. not creating a policy violation for it", er.PolicyResponse.Resource)
			continue
		}
		if !er.IsSuccesful() {
			if pv := buildPVForPolicy(er); !reflect.DeepEqual(pv, kyverno.ClusterPolicyViolation{}) {
				pvs = append(pvs, pv)
			}
		}
	}
	if len(pvs) == 0 {
		return
	}
	for _, newPv := range pvs {
		glog.V(4).Infof("creating policyViolation resource for policy %s and resource %s/%s/%s", newPv.Spec.Policy, newPv.Spec.Kind, newPv.Spec.Namespace, newPv.Spec.Name)
		// check if there was a previous policy voilation for policy & resource combination
		curPv, err := getExistingPolicyViolationIfAny(nil, pvLister, newPv)
		if err != nil {
			glog.Error(err)
			continue
		}
		if curPv == nil {
			glog.V(4).Infof("creating new policy violation for policy %s & resource %s/%s/%s", newPv.Spec.Policy, newPv.Spec.ResourceSpec.Kind, newPv.Spec.ResourceSpec.Namespace, newPv.Spec.ResourceSpec.Name)
			// no existing policy violation, create a new one
			_, err := client.KyvernoV1alpha1().ClusterPolicyViolations().Create(&newPv)
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
		glog.V(4).Infof("creating new policy violation for policy %s & resource %s/%s/%s", curPv.Spec.Policy, curPv.Spec.ResourceSpec.Kind, curPv.Spec.ResourceSpec.Namespace, curPv.Spec.ResourceSpec.Name)
		//TODO: using a generic name, but would it be helpful to have naming convention for policy violations
		// as we can only have one policy violation for each (policy + resource) combination
		_, err = client.KyvernoV1alpha1().ClusterPolicyViolations().Update(&newPv)
		if err != nil {
			glog.Error(err)
			continue
		}
	}
}

//TODO: change the name
func getExistingPolicyViolationIfAny(pvListerSynced cache.InformerSynced, pvLister kyvernolister.ClusterPolicyViolationLister, newPv kyverno.ClusterPolicyViolation) (*kyverno.ClusterPolicyViolation, error) {
	// TODO: check for existing ov using label selectors on resource and policy
	// TODO: there can be duplicates, as the labels have not been assigned to the policy violation yet
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
