package policyviolation

import (
	"fmt"
	"reflect"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernov1alpha1 "github.com/nirmata/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1alpha1"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	engine "github.com/nirmata/kyverno/pkg/engine"
	labels "k8s.io/apimachinery/pkg/labels"
)

func CreateNamespacePV(pvLister kyvernolister.NamespacedPolicyViolationLister, client *kyvernoclient.Clientset, engineResponses []engine.EngineResponse) {
	// var pvs []kyverno.NamespacedPolicyViolation
	// for _, er := range engineResponses {
	// 	// ignore creation of PV for resoruces that are yet to be assigned a name
	// 	if er.PolicyResponse.Resource.Name == "" {
	// 		glog.V(4).Infof("resource %v, has not been assigned a name, not creating a namespace policy violation for it", er.PolicyResponse.Resource)
	// 		continue
	// 	}

	// 	if !er.IsSuccesful() {
	// 		glog.V(4).Infof("Building namespace policy violation for engine response %v", er)
	// 		if pv := buildNamespacedPVForPolicy(er); !reflect.DeepEqual(pv, kyverno.NamespacedPolicyViolation{}) {
	// 			pvs = append(pvs, pv)
	// 		}
	// 	}
	// }

	// createNamespacedPV(pvLister, client, pvs)
}

// CreateNamespacedPVWhenBlocked creates pv on resource owner only when admission request is denied
func CreateNamespacedPVWhenBlocked(pvLister kyvernolister.NamespacedPolicyViolationLister, client *kyvernoclient.Clientset,
	dclient *dclient.Client, engineResponses []engine.EngineResponse) {
	// var pvs []kyverno.NamespacedPolicyViolation
	// for _, er := range engineResponses {
	// 	// child resource is not created in this case thus it won't have a name
	// 	glog.V(4).Infof("Building policy violation for denied admission request, engineResponse: %v", er)
	// 	if pvList := buildNamespacedPVWithOwner(dclient, er); len(pvList) != 0 {
	// 		pvs = append(pvs, pvList...)
	// 		glog.V(3).Infof("Built policy violation for denied admission request %s/%s/%s",
	// 			er.PatchedResource.GetKind(), er.PatchedResource.GetNamespace(), er.PatchedResource.GetName())
	// 	}
	// }
	// createNamespacedPV(pvLister, client, pvs)
}

func buildNamespacedPV(info Info) kyverno.NamespacedPolicyViolation {
	return buildNamespacedPVObj(info.PolicyName,
		kyverno.ResourceSpec{
			Kind:      info.Resource.GetKind(),
			Namespace: info.Resource.GetNamespace(),
			Name:      info.Resource.GetName(),
		},
		info.Rules)
}

//buildNamespacedPVObj returns an value of type PolicyViolation
func buildNamespacedPVObj(policy string, resource kyverno.ResourceSpec, fRules []kyverno.ViolatedRule) kyverno.NamespacedPolicyViolation {
	pv := kyverno.NamespacedPolicyViolation{
		Spec: kyverno.PolicyViolationSpec{
			Policy:        policy,
			ResourceSpec:  resource,
			ViolatedRules: fRules,
		},
	}
	pv.SetGenerateName("pv-")
	return pv
}

func buildNamespacedPVWithOwner(dclient *dclient.Client, info Info) (pvs []kyverno.NamespacedPolicyViolation) {
	// create violation on resource owner (if exist) when action is set to enforce
	owners := getOwnersOld(dclient, info.Resource)

	// standaloneresource, set pvResourceSpec with resource itself
	if len(owners) == 0 {
		pvResourceSpec := kyverno.ResourceSpec{
			Namespace: info.Resource.GetNamespace(),
			Kind:      info.Resource.GetKind(),
			Name:      info.Resource.GetName(),
		}
		return append(pvs, buildNamespacedPVObj(info.PolicyName, pvResourceSpec, info.Rules))
	}

	for _, owner := range owners {
		pvs = append(pvs, buildNamespacedPVObj(info.PolicyName, owner, info.Rules))
	}
	return
}

func createNamespacedPV(pvLister kyvernolister.NamespacedPolicyViolationLister, pvInterface kyvernov1alpha1.KyvernoV1alpha1Interface, pvs []kyverno.NamespacedPolicyViolation) {
	for _, newPv := range pvs {
		glog.V(4).Infof("creating namespaced policyViolation resource for policy %s and resource %s", newPv.Spec.Policy, newPv.Spec.ResourceSpec.ToKey())
		// check if there was a previous policy voilation for policy & resource combination
		curPv, err := getExistingNamespacedPVIfAny(pvLister, newPv)
		if err != nil {
			glog.Error(err)
			continue
		}

		if curPv == nil {
			glog.V(4).Infof("creating new namespaced policy violation for policy %s & resource %s", newPv.Spec.Policy, newPv.Spec.ResourceSpec.ToKey())
			// no existing policy violation, create a new one
			_, err := pvInterface.NamespacedPolicyViolations(newPv.Spec.ResourceSpec.Namespace).Create(&newPv)
			if err != nil {
				glog.Error(err)
			} else {
				glog.Infof("namespaced policy violation created for resource %s", newPv.Spec.ResourceSpec.ToKey())
			}
			continue
		}
		// compare the policyviolation spec for existing resource if present else
		if reflect.DeepEqual(curPv.Spec, newPv.Spec) {
			// if they are equal there has been no change so dont update the polivy violation
			glog.V(3).Infof("namespaced policy violation '%s' spec did not change so not updating it", newPv.Spec.ToKey())
			glog.V(4).Infof("namespaced policy violation spec %v did not change so not updating it", newPv.Spec)
			continue
		}
		// spec changed so update the policyviolation
		glog.V(4).Infof("creating new policy violation for policy %s & resource %s", curPv.Spec.Policy, curPv.Spec.ResourceSpec.ToKey())
		//TODO: using a generic name, but would it be helpful to have naming convention for policy violations
		// as we can only have one policy violation for each (policy + resource) combination
		_, err = pvInterface.NamespacedPolicyViolations(newPv.Spec.ResourceSpec.Namespace).Update(&newPv)
		if err != nil {
			glog.Error(err)
			continue
		}
		glog.Infof("namespaced policy violation updated for resource %s", newPv.Spec.ResourceSpec.ToKey())
	}
}

func getExistingNamespacedPVIfAny(nspvLister kyvernolister.NamespacedPolicyViolationLister, newPv kyverno.NamespacedPolicyViolation) (*kyverno.NamespacedPolicyViolation, error) {
	// TODO(shuting): list pvs by labels
	pvs, err := nspvLister.List(labels.NewSelector())
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaced policy violations err: %v", err)
	}

	for _, pv := range pvs {
		if pv.Spec.Policy == newPv.Spec.Policy && reflect.DeepEqual(pv.Spec.ResourceSpec, newPv.Spec.ResourceSpec) {
			return pv, nil
		}
	}

	return nil, nil
}
