package policyviolation

import (
	"fmt"
	"reflect"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1alpha1"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/engine"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

type pvResourceOwner struct {
	kind      string
	namespace string
	name      string
}

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

//CreatePV creates policy violation resource based on the engine responses
func CreatePV(pvLister kyvernolister.ClusterPolicyViolationLister, client *kyvernoclient.Clientset,
	dclient *dclient.Client, engineResponses []engine.EngineResponse, requestBlocked bool) {
	var pvs []kyverno.ClusterPolicyViolation
	for _, er := range engineResponses {
		// create pv on resource owner only when admission request is denied
		// check before validate "er.PolicyResponse.Resource.Name" since
		// child resource is not created in this case thus it won't have a name
		if requestBlocked {
			glog.V(4).Infof("Building policy violation for denied admission request, engineResponse: %v", er)
			if pvList := buildPVWithOwner(dclient, er); len(pvList) != 0 {
				pvs = append(pvs, pvList...)
			}
			continue
		}

		// ignore creation of PV for resoruces that are yet to be assigned a name
		if er.PolicyResponse.Resource.Name == "" {
			glog.V(4).Infof("resource %v, has not been assigned a name, not creating a policy violation for it", er.PolicyResponse.Resource)
			continue
		}

		if !er.IsSuccesful() {
			glog.V(4).Infof("Building policy violation for engine response %v", er)
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
			glog.Infof("policy violation '%s/%s/%s' spec did not change so not updating it", newPv.Spec.Kind, newPv.Spec.Namespace, newPv.Spec.Name)
			glog.V(4).Infof("policy violation spec %v did not change so not updating it", newPv.Spec)
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

func buildPVForPolicy(er engine.EngineResponse) kyverno.ClusterPolicyViolation {
	pvResourceSpec := kyverno.ResourceSpec{
		Kind:      er.PolicyResponse.Resource.Kind,
		Namespace: er.PolicyResponse.Resource.Namespace,
		Name:      er.PolicyResponse.Resource.Name,
	}

	violatedRules := newViolatedRules(er, "")

	return BuildPolicyViolation(er.PolicyResponse.Policy, pvResourceSpec, violatedRules)
}

func buildPVWithOwner(dclient *dclient.Client, er engine.EngineResponse) (pvs []kyverno.ClusterPolicyViolation) {
	msg := fmt.Sprintf("Request Blocked for resource %s/%s; ", er.PolicyResponse.Resource.Kind, er.PolicyResponse.Resource.Name)
	violatedRules := newViolatedRules(er, msg)

	// create violation on resource owner (if exist) when action is set to enforce
	owners := getOwners(dclient, er.PatchedResource)

	// standaloneresource, set pvResourceSpec with resource itself
	if len(owners) == 0 {
		pvResourceSpec := kyverno.ResourceSpec{
			Namespace: er.PolicyResponse.Resource.Namespace,
			Kind:      er.PolicyResponse.Resource.Kind,
			Name:      er.PolicyResponse.Resource.Name,
		}
		return append(pvs, BuildPolicyViolation(er.PolicyResponse.Policy, pvResourceSpec, violatedRules))
	}

	for _, owner := range owners {
		// resource has owner, set pvResourceSpec with owner info
		pvResourceSpec := kyverno.ResourceSpec{
			Namespace: owner.namespace,
			Kind:      owner.kind,
			Name:      owner.name,
		}
		pvs = append(pvs, BuildPolicyViolation(er.PolicyResponse.Policy, pvResourceSpec, violatedRules))
	}
	return
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

// pass in unstr rather than using the client to get the unstr
// as if name is empty then GetResource panic as it returns a list
func getOwners(dclient *dclient.Client, unstr unstructured.Unstructured) []pvResourceOwner {
	resourceOwners := unstr.GetOwnerReferences()
	if len(resourceOwners) == 0 {
		return []pvResourceOwner{pvResourceOwner{
			kind:      unstr.GetKind(),
			namespace: unstr.GetNamespace(),
			name:      unstr.GetName(),
		}}
	}

	var owners []pvResourceOwner
	for _, resourceOwner := range resourceOwners {
		unstrParent, err := dclient.GetResource(resourceOwner.Kind, unstr.GetNamespace(), resourceOwner.Name)
		if err != nil {
			glog.Errorf("Failed to get resource owner for %s/%s/%s, err: %v", resourceOwner.Kind, unstr.GetNamespace(), resourceOwner.Name, err)
			return nil
		}

		owners = append(owners, getOwners(dclient, *unstrParent)...)
	}
	return owners
}

func newViolatedRules(er engine.EngineResponse, msg string) (violatedRules []kyverno.ViolatedRule) {
	for _, r := range er.PolicyResponse.Rules {
		// filter failed/violated rules
		if !r.Success {
			vrule := kyverno.ViolatedRule{
				Name:    r.Name,
				Type:    r.Type,
				Message: msg + r.Message,
			}
			violatedRules = append(violatedRules, vrule)
		}
	}
	return
}
