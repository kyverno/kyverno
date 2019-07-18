package controller

import (
	"github.com/golang/glog"
	"github.com/minio/minio/pkg/wildcard"
	"github.com/nirmata/kyverno/pkg/annotations"
	v1alpha1 "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func cleanAnnotations(client *client.Client, obj interface{}) {
	// get the policy struct from interface
	unstr, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		glog.Error(err)
		return
	}
	policy := v1alpha1.Policy{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstr, &policy); err != nil {
		glog.Error(err)
		return
	}
	// Get the resources that apply to the policy
	// key uid
	resourceMap := map[string]unstructured.Unstructured{}
	for _, rule := range policy.Spec.Rules {
		for _, k := range rule.Kinds {
			if k == "Namespace" {
				// REWORK: will be handeled by namespace controller
				continue
			}
			// kind -> resource
			gvr := client.DiscoveryClient.GetGVRFromKind(k)
			// label selectors
			// namespace ? should it be default or allow policy to specify it
			namespace := "default"
			if rule.ResourceDescription.Namespace != nil {
				namespace = *rule.ResourceDescription.Namespace
			}
			list, err := client.ListResource(k, namespace, rule.ResourceDescription.Selector)
			if err != nil {
				glog.Errorf("unable to list resource for %s with label selector %s", gvr.Resource, rule.Selector.String())
				glog.Errorf("unable to apply policy %s rule %s. err: %s", policy.Name, rule.Name, err)
				continue
			}
			for _, res := range list.Items {
				name := rule.ResourceDescription.Name
				if name != nil {
					// wild card matching
					if !wildcard.Match(*name, res.GetName()) {
						continue
					}
				}
				resourceMap[string(res.GetUID())] = res
			}
		}
	}

	// remove annotations for the resources
	for _, obj := range resourceMap {
		// get annotations
		ann := obj.GetAnnotations()

		_, patch, err := annotations.RemovePolicyJSONPatch(ann, policy.Name)
		if err != nil {
			glog.Error(err)
			continue
		}
		// patch the resource
		_, err = client.PatchResource(obj.GetKind(), obj.GetNamespace(), obj.GetName(), patch)
		if err != nil {
			glog.Error(err)
			continue
		}
	}
}
