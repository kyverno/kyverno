package controller

import (
	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/annotations"
	v1alpha1 "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/engine"
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
	resourceMap := engine.ListResourcesThatApplyToPolicy(client, &policy)
	// remove annotations for the resources
	for _, obj := range resourceMap {
		// get annotations

		ann := obj.Resource.GetAnnotations()

		_, patch, err := annotations.RemovePolicyJSONPatch(ann, annotations.BuildKey(policy.Name))
		if err != nil {
			glog.Error(err)
			continue
		}
		// patch the resource
		_, err = client.PatchResource(obj.Resource.GetKind(), obj.Resource.GetNamespace(), obj.Resource.GetName(), patch)
		if err != nil {
			glog.Error(err)
			continue
		}
	}
}
