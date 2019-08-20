package webhooks

import (
	"github.com/golang/glog"
	engine "github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/info"
	policyctr "github.com/nirmata/kyverno/pkg/policy"
	"github.com/nirmata/kyverno/pkg/policyviolation"
	"github.com/nirmata/kyverno/pkg/utils"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// HandleValidation handles validating webhook admission request
// If there are no errors in validating rule we apply generation rules
func (ws *WebhookServer) HandleValidation(request *v1beta1.AdmissionRequest, resource unstructured.Unstructured) *v1beta1.AdmissionResponse {
	var policyInfos []info.PolicyInfo
	var policyStats []policyctr.PolicyStat
	// gather stats from the engine response
	gatherStat := func(policyName string, er engine.EngineResponse) {
		ps := policyctr.PolicyStat{}
		ps.PolicyName = policyName
		ps.ValidationExecutionTime = er.ExecutionTime
		ps.RulesAppliedCount = er.RulesAppliedCount
		policyStats = append(policyStats, ps)
	}
	// send stats for aggregation
	sendStat := func(blocked bool) {
		for _, stat := range policyStats {
			stat.ResourceBlocked = blocked
			//SEND
			ws.policyStatus.SendStat(stat)
		}
	}

	glog.V(5).Infof("Receive request in validating webhook: Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
		request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation)

	policies, err := ws.pLister.List(labels.NewSelector())
	if err != nil {
		//TODO check if the CRD is created ?
		// Unable to connect to policy Lister to access policies
		glog.Error("Unable to connect to policy controller to access policies. Validation Rules are NOT being applied")
		glog.Warning(err)
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	//TODO: check if resource gvk is available in raw resource,
	// if not then set it from the api request
	resource.SetGroupVersionKind(schema.GroupVersionKind{Group: request.Kind.Group, Version: request.Kind.Version, Kind: request.Kind.Kind})
	//TODO: check if the name and namespace is also passed right in the resource?
	// all the patches to be applied on the resource

	for _, policy := range policies {

		if !utils.Contains(getApplicableKindsForPolicy(policy), request.Kind.Kind) {
			continue
		}

		policyInfo := info.NewPolicyInfo(policy.Name, resource.GetKind(), resource.GetName(), resource.GetNamespace(), policy.Spec.ValidationFailureAction)

		glog.V(4).Infof("Handling validation for Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
			resource.GetKind(), resource.GetNamespace(), resource.GetName(), request.UID, request.Operation)

		glog.V(4).Infof("Validating resource %s/%s/%s with policy %s with %d rules\n", resource.GetKind(), resource.GetNamespace(), resource.GetName(), policy.ObjectMeta.Name, len(policy.Spec.Rules))

		engineResponse := engine.Validate(*policy, resource)
		if len(engineResponse.RuleInfos) == 0 {
			continue
		}
		gatherStat(policy.Name, engineResponse)

		if len(engineResponse.RuleInfos) > 0 {
			glog.V(4).Infof("Validation from policy %s has applied succesfully to %s %s/%s", policy.Name, request.Kind.Kind, resource.GetNamespace(), resource.GetName())
		}

		policyInfo.AddRuleInfos(engineResponse.RuleInfos)

		if !policyInfo.IsSuccessful() {
			glog.Infof("Failed to apply policy %s on resource %s/%s", policy.Name, resource.GetNamespace(), resource.GetName())
			for _, r := range engineResponse.RuleInfos {
				glog.Warningf("%s: %s\n", r.Name, r.Msgs)
			}
		}

		policyInfos = append(policyInfos, policyInfo)

	}

	// ADD EVENTS
	if len(policyInfos) > 0 && len(policyInfos[0].Rules) != 0 {
		eventsInfo := newEventInfoFromPolicyInfo(policyInfos, (request.Operation == v1beta1.Update), info.Validation)
		// If the validationFailureAction flag is set "audit",
		// then we dont block the request and report the violations
		ws.eventGen.Add(eventsInfo...)
	}

	// If Validation fails then reject the request
	// violations are created if "audit" flag is set
	// and if there are any then we dont block the resource creation
	// Even if one the policy being applied
	ok, msg := isAdmSuccesful(policyInfos)
	if !ok && toBlock(policyInfos) {
		sendStat(true)
		return &v1beta1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: msg,
			},
		}
	}

	// ADD POLICY VIOLATIONS
	policyviolation.GeneratePolicyViolations(ws.pvListerSynced, ws.pvLister, ws.kyvernoClient, policyInfos)

	sendStat(false)
	return &v1beta1.AdmissionResponse{
		Allowed: true,
	}
}
