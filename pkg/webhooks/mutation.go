package webhooks

import (
	"github.com/golang/glog"
	engine "github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/info"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// HandleMutation handles mutating webhook admission request
func (ws *WebhookServer) HandleMutation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	var patches [][]byte
	var policyInfos []info.PolicyInfo
	// map to store the mutation changes on the resource
	// mAnn := map[string]string{}
	glog.V(4).Infof("Receive request in mutating webhook: Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
		request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation)

	policies, err := ws.pLister.List(labels.NewSelector())
	if err != nil {
		//TODO check if the CRD is created ?
		// Unable to connect to policy Lister to access policies
		glog.Error("Unable to connect to policy controller to access policies. Mutation Rules are NOT being applied")
		glog.Warning(err)
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	resource, err := convertToUnstructured(request.Object.Raw)
	if err != nil {
		glog.Errorf("unable to convert raw resource to unstructured: %v", err)
	}
	//TODO: check if resource gvk is available in raw resource,
	// if not then set it from the api request
	resource.SetGroupVersionKind(schema.GroupVersionKind{Group: request.Kind.Group, Version: request.Kind.Version, Kind: request.Kind.Kind})
	//TODO: check if the name and namespace is also passed right in the resource?
	// all the patches to be applied on the resource

	for _, policy := range policies {
		// check if policy has a rule for the admission request kind
		if !StringInSlice(request.Kind.Kind, getApplicableKindsForPolicy(policy)) {
			continue
		}
		policyInfo := info.NewPolicyInfo(policy.Name, resource.GetKind(), resource.GetName(), resource.GetNamespace(), policy.Spec.ValidationFailureAction)

		glog.V(4).Infof("Handling mutation for Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
			resource.GetKind(), resource.GetNamespace(), resource.GetName(), request.UID, request.Operation)
		glog.V(4).Infof("Applying policy %s with %d rules\n", policy.ObjectMeta.Name, len(policy.Spec.Rules))

		// resource, err := utils.ConvertToUnstructured(request.Object.Raw)
		// if err != nil {
		// 	glog.Errorf("unable to process policy %s resource %v: %v", policy.GetName(), request.Resource, err)
		// 	continue
		// }
		//TODO: check if the GVK information is present in the request of we set it explicity here ?
		glog.V(4).Infof("GVK is %v", resource.GroupVersionKind())
		// resource.SetGroupVersionKind(schema.GroupVersionKind{Group: request.Kind.Group, Version: request.Kind.Version, Kind: request.Kind.Kind})
		//TODO: passing policy value as we dont wont to modify the policy

		policyPatches, ruleInfos := engine.Mutate(*policy, *resource)
		policyInfo.AddRuleInfos(ruleInfos)
		policyInfos = append(policyInfos, policyInfo)
		if !policyInfo.IsSuccessful() {
			glog.V(4).Infof("Failed to apply policy %s on resource %s/%s", policy.Name, resource.GetNamespace(), resource.GetName())
			glog.V(4).Info("Failed rule details")
			for _, r := range ruleInfos {
				glog.V(4).Infof("%s: %s\n", r.Name, r.Msgs)
			}
			continue
		}
		// build annotations per policy being applied to show the mutation changes
		patches = append(patches, policyPatches...)
		glog.V(4).Infof("Mutation from policy %s has applied succesfully to %s %s/%s", policy.Name, request.Kind.Kind, resource.GetNamespace(), resource.GetName())
	}

	// ADD ANNOTATIONS
	// TODO: merge the annotation patch with the patch response
	// ADD EVENTS
	if len(patches) > 0 {
		eventsInfo, _ := newEventInfoFromPolicyInfo(policyInfos, (request.Operation == v1beta1.Update), info.Mutation)
		ws.eventGen.Add(eventsInfo...)
	}
	// ADD POLICY VIOLATIONS

	ok, msg := isAdmSuccesful(policyInfos)
	if ok {
		patchType := v1beta1.PatchTypeJSONPatch
		return &v1beta1.AdmissionResponse{
			Allowed:   true,
			Patch:     engine.JoinPatches(patches),
			PatchType: &patchType,
		}
	}
	return &v1beta1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Message: msg,
		},
	}
}
