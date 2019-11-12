package policyviolation

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1alpha1"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/engine"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/client-go/tools/cache"
	deployutil "k8s.io/kubernetes/pkg/controller/deployment/util"
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
	pv.SetGenerateName("pv-")
	return pv
}

//CreatePV creates policy violation resource based on the engine responses
func CreateClusterPV(pvLister kyvernolister.ClusterPolicyViolationLister, client *kyvernoclient.Clientset, engineResponses []engine.EngineResponse) {
	var pvs []kyverno.ClusterPolicyViolation
	for _, er := range engineResponses {
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

	createClusterPV(pvLister, client, pvs)
}

// CreatePVWhenBlocked creates pv on resource owner only when admission request is denied
func CreateClusterPVWhenBlocked(pvLister kyvernolister.ClusterPolicyViolationLister, client *kyvernoclient.Clientset,
	dclient *dclient.Client, engineResponses []engine.EngineResponse) {
	var pvs []kyverno.ClusterPolicyViolation
	for _, er := range engineResponses {
		// child resource is not created in this case thus it won't have a name
		glog.V(4).Infof("Building policy violation for denied admission request, engineResponse: %v", er)
		if pvList := buildPVWithOwner(dclient, er); len(pvList) != 0 {
			pvs = append(pvs, pvList...)
			glog.V(3).Infof("Built policy violation for denied admission request %s/%s/%s",
				er.PatchedResource.GetKind(), er.PatchedResource.GetNamespace(), er.PatchedResource.GetName())
		}
	}
	createClusterPV(pvLister, client, pvs)
}

func createClusterPV(pvLister kyvernolister.ClusterPolicyViolationLister, client *kyvernoclient.Clientset, pvs []kyverno.ClusterPolicyViolation) {
	if len(pvs) == 0 {
		return
	}

	for _, newPv := range pvs {
		glog.V(4).Infof("creating policyViolation resource for policy %s and resource %s/%s/%s", newPv.Spec.Policy, newPv.Spec.Kind, newPv.Spec.Namespace, newPv.Spec.Name)
		// check if there was a previous policy voilation for policy & resource combination
		curPv, err := getExistingPolicyViolationIfAny(nil, pvLister, newPv)
		if err != nil {
			// TODO(shuting): remove
			// glog.Error(err)
			continue
		}
		if curPv == nil {
			glog.V(4).Infof("creating new policy violation for policy %s & resource %s/%s/%s", newPv.Spec.Policy, newPv.Spec.ResourceSpec.Kind, newPv.Spec.ResourceSpec.Namespace, newPv.Spec.ResourceSpec.Name)
			// no existing policy violation, create a new one
			_, err := client.KyvernoV1alpha1().ClusterPolicyViolations().Create(&newPv)
			if err != nil {
				glog.Error(err)
			} else {
				glog.Infof("policy violation created for resource %s", newPv.Spec.ResourceSpec.ToKey())
			}
			continue
		}
		// compare the policyviolation spec for existing resource if present else
		if reflect.DeepEqual(curPv.Spec, newPv.Spec) {
			// if they are equal there has been no change so dont update the polivy violation
			glog.V(3).Infof("policy violation '%s/%s/%s' spec did not change so not updating it", newPv.Spec.Kind, newPv.Spec.Namespace, newPv.Spec.Name)
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
		glog.Infof("policy violation updated for resource %s", newPv.Spec.ResourceSpec.ToKey())
	}
}

//buildClusterPolicyViolation returns an value of type PolicyViolation
func buildClusterPolicyViolation(policy string, resource kyverno.ResourceSpec, fRules []kyverno.ViolatedRule) kyverno.ClusterPolicyViolation {
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
	pvResourceSpec := kyverno.ResourceSpec{
		Kind:      er.PolicyResponse.Resource.Kind,
		Namespace: er.PolicyResponse.Resource.Namespace,
		Name:      er.PolicyResponse.Resource.Name,
	}

	violatedRules := newViolatedRules(er, "")

	return buildClusterPolicyViolation(er.PolicyResponse.Policy, pvResourceSpec, violatedRules)
}

func buildPVWithOwner(dclient *dclient.Client, er engine.EngineResponse) (pvs []kyverno.ClusterPolicyViolation) {
	msg := fmt.Sprintf("Request Blocked for resource %s/%s; ", er.PolicyResponse.Resource.Namespace, er.PolicyResponse.Resource.Kind)
	violatedRules := newViolatedRules(er, msg)

	// create violation on resource owner (if exist) when action is set to enforce
	owners := GetOwners(dclient, er.PatchedResource)

	// standaloneresource, set pvResourceSpec with resource itself
	if len(owners) == 0 {
		pvResourceSpec := kyverno.ResourceSpec{
			Namespace: er.PolicyResponse.Resource.Namespace,
			Kind:      er.PolicyResponse.Resource.Kind,
			Name:      er.PolicyResponse.Resource.Name,
		}
		return append(pvs, buildClusterPolicyViolation(er.PolicyResponse.Policy, pvResourceSpec, violatedRules))
	}

	for _, owner := range owners {
		pvs = append(pvs, buildClusterPolicyViolation(er.PolicyResponse.Policy, owner, violatedRules))
	}
	return
}

//TODO: change the name
func getExistingPolicyViolationIfAny(pvListerSynced cache.InformerSynced, pvLister kyvernolister.ClusterPolicyViolationLister, newPv kyverno.ClusterPolicyViolation) (*kyverno.ClusterPolicyViolation, error) {
	labelMap := map[string]string{"policy": newPv.Spec.Policy, "resource": newPv.Spec.ResourceSpec.ToKey()}
	policyViolationSelector, err := converLabelToSelector(labelMap)
	if err != nil {
		return nil, fmt.Errorf("failed to generate label sector of Policy name %s: %v", newPv.Spec.Policy, err)
	}
	pvs, err := pvLister.List(policyViolationSelector)
	if err != nil {
		glog.Errorf("unable to list policy violations with label selector %v: %v", policyViolationSelector, err)
		return nil, err
	}
	//TODO: ideally there should be only one policy violation returned
	if len(pvs) > 1 {
		glog.V(4).Infof("more than one policy violation exists  with labels %v", labelMap)
		return nil, fmt.Errorf("more than one policy violation exists  with labels %v", labelMap)
	}

	if len(pvs) == 0 {
		glog.Infof("policy violation does not exist with labels %v", labelMap)
		return nil, nil
	}
	return pvs[0], nil
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

//GetOwners pass in unstr rather than using the client to get the unstr
// as if name is empty then GetResource panic as it returns a list
func GetOwners(dclient *dclient.Client, unstr unstructured.Unstructured) []kyverno.ResourceSpec {
	resourceOwners := unstr.GetOwnerReferences()
	if len(resourceOwners) == 0 {
		return []kyverno.ResourceSpec{kyverno.ResourceSpec{
			Kind:      unstr.GetKind(),
			Namespace: unstr.GetNamespace(),
			Name:      unstr.GetName(),
		}}
	}
	var owners []kyverno.ResourceSpec
	for _, resourceOwner := range resourceOwners {
		unstrParent, err := dclient.GetResource(resourceOwner.Kind, unstr.GetNamespace(), resourceOwner.Name)
		if err != nil {
			glog.Errorf("Failed to get resource owner for %s/%s/%s, err: %v", resourceOwner.Kind, unstr.GetNamespace(), resourceOwner.Name, err)
			return nil
		}

		owners = append(owners, GetOwners(dclient, *unstrParent)...)
	}
	return owners
}

func newViolatedRules(er engine.EngineResponse, msg string) (violatedRules []kyverno.ViolatedRule) {
	unstr := er.PatchedResource
	dependant := kyverno.ManagedResourceSpec{
		Kind:            unstr.GetKind(),
		Namespace:       unstr.GetNamespace(),
		CreationBlocked: true,
	}

	for _, r := range er.PolicyResponse.Rules {
		// filter failed/violated rules
		if !r.Success {
			vrule := kyverno.ViolatedRule{
				Name:    r.Name,
				Type:    r.Type,
				Message: msg + r.Message,
			}
			// resource creation blocked
			// set resource itself as dependant
			if strings.Contains(msg, "Request Blocked") {
				vrule.ManagedResource = dependant
			}

			violatedRules = append(violatedRules, vrule)
		}
	}
	return
}

func containsOwner(owners []kyverno.ResourceSpec, pv *kyverno.ClusterPolicyViolation) bool {
	curOwner := kyverno.ResourceSpec{
		Kind:      pv.Spec.ResourceSpec.Kind,
		Namespace: pv.Spec.ResourceSpec.Namespace,
		Name:      pv.Spec.ResourceSpec.Name,
	}

	for _, targetOwner := range owners {
		if reflect.DeepEqual(curOwner, targetOwner) {
			return true
		}
	}
	return false
}

// validDependantForDeployment checks if resource (pod) matches the intent of the given deployment
// explicitly handles deployment-replicaset-pod relationship
func validDependantForDeployment(client appsv1.AppsV1Interface, curPv kyverno.ClusterPolicyViolation, resource unstructured.Unstructured) bool {
	if resource.GetKind() != "Pod" {
		return false
	}

	// only handles deploymeny-replicaset-pod relationship
	if curPv.Spec.ResourceSpec.Kind != "Deployment" {
		return false
	}

	owner := kyverno.ResourceSpec{
		Kind:      curPv.Spec.ResourceSpec.Kind,
		Namespace: curPv.Spec.ResourceSpec.Namespace,
		Name:      curPv.Spec.ResourceSpec.Name,
	}

	deploy, err := client.Deployments(owner.Namespace).Get(owner.Name, metav1.GetOptions{})
	if err != nil {
		glog.Errorf("failed to get resourceOwner deployment %s/%s/%s: %v", owner.Kind, owner.Namespace, owner.Name, err)
		return false
	}

	expectReplicaset, err := deployutil.GetNewReplicaSet(deploy, client)
	if err != nil {
		glog.Errorf("failed to get replicaset owned by %s/%s/%s: %v", owner.Kind, owner.Namespace, owner.Name, err)
		return false
	}

	if reflect.DeepEqual(expectReplicaset, v1.ReplicaSet{}) {
		glog.V(2).Infof("no replicaset found for deploy %s/%s/%s", owner.Namespace, owner.Kind, owner.Name)
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
