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

// HandleValidation handles validating webhook admission request
// If there are no errors in validating rule we apply generation rules
func (ws *WebhookServer) HandleValidation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	// var patches [][]byte
	var policyInfos []*info.PolicyInfo

	glog.V(4).Infof("Receive request in validating webhook: Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
		request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation)

	policies, err := ws.policyLister.List(labels.NewSelector())
	if err != nil {
		//TODO check if the CRD is created ?
		// Unable to connect to policy Lister to access policies
		glog.Error("Unable to connect to policy controller to access policies. Validation Rules are NOT being applied")
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

		if !StringInSlice(request.Kind.Kind, getApplicableKindsForPolicy(policy)) {
			continue
		}

		policyInfo := info.NewPolicyInfo(policy.Name, resource.GetKind(), resource.GetName(), resource.GetNamespace(), policy.Spec.ValidationFailureAction)

		glog.V(4).Infof("Handling validation for Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
			resource.GetKind(), resource.GetNamespace(), resource.GetName(), request.UID, request.Operation)

		glog.V(4).Infof("Applying policy %s with %d rules\n", policy.ObjectMeta.Name, len(policy.Spec.Rules))

		ruleInfos, err := engine.Validate(*policy, request.Object.Raw, request.Kind)
		if err != nil {
			// This is not policy error
			// but if unable to parse request raw resource
			// TODO : create event ? dont think so
			glog.Error(err)
			continue
		}
		policyInfo.AddRuleInfos(ruleInfos)

		if !policyInfo.IsSuccessful() {
			glog.Infof("Failed to apply policy %s on resource %s/%s", policy.Name, resource.GetNamespace(), resource.GetName())
			glog.V(4).Info("Failed rule details")
			for _, r := range ruleInfos {
				glog.V(4).Infof("%s: %s\n", r.Name, r.Msgs)
			}
			continue
		}
		if len(ruleInfos) > 0 {
			glog.V(4).Infof("Validation from policy %s has applied succesfully to %s %s/%s", policy.Name, request.Kind.Kind, resource.GetNamespace(), resource.GetName())
		}
		policyInfos = append(policyInfos, policyInfo)
	}

	// ADD EVENTS
	// ADD POLICY VIOLATIONS
	ok, msg := isAdmSuccesful(policyInfos)
	if !ok && toBlock(policyInfos) {
		return &v1beta1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: msg,
			},
		}
	}

	return &v1beta1.AdmissionResponse{
		Allowed: true,
	}
	// Generation rules applied via generation controller
}
