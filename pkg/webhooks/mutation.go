package webhooks

import (
	"github.com/golang/glog"
	engine "github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/info"
	policyctr "github.com/nirmata/kyverno/pkg/policy"
	"github.com/nirmata/kyverno/pkg/utils"
	v1beta1 "k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// HandleMutation handles mutating webhook admission request
func (ws *WebhookServer) HandleMutation(request *v1beta1.AdmissionRequest) (bool, engine.EngineResponse) {
	var patches [][]byte
	var policyInfos []info.PolicyInfo
	var policyStats []policyctr.PolicyStat
	// gather stats from the engine response
	gatherStat := func(policyName string, er engine.EngineResponse) {
		ps := policyctr.PolicyStat{}
		ps.PolicyName = policyName
		ps.MutationExecutionTime = er.ExecutionTime
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

	glog.V(5).Infof("Receive request in mutating webhook: Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
		request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation)

	resource, err := engine.ConvertToUnstructured(request.Object.Raw)
	if err != nil {
		glog.Errorf("unable to convert raw resource to unstructured: %v", err)
	}

	//TODO: check if resource gvk is available in raw resource,
	// if not then set it from the api request
	resource.SetGroupVersionKind(schema.GroupVersionKind{Group: request.Kind.Group, Version: request.Kind.Version, Kind: request.Kind.Kind})
	//TODO: check if the name and namespace is also passed right in the resource?

	engineResponse := engine.EngineResponse{PatchedResource: *resource}

	policies, err := ws.pLister.List(labels.NewSelector())
	if err != nil {
		//TODO check if the CRD is created ?
		// Unable to connect to policy Lister to access policies
		glog.Errorln("Unable to connect to policy controller to access policies. Mutation Rules are NOT being applied")
		glog.Warning(err)
		return true, engineResponse
	}

	for _, policy := range policies {

		// check if policy has a rule for the admission request kind
		if !utils.Contains(getApplicableKindsForPolicy(policy), request.Kind.Kind) {
			continue
		}

		policyInfo := info.NewPolicyInfo(policy.Name, resource.GetKind(), resource.GetName(), resource.GetNamespace(), policy.Spec.ValidationFailureAction)

		glog.V(4).Infof("Handling mutation for Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
			resource.GetKind(), resource.GetNamespace(), resource.GetName(), request.UID, request.Operation)
		glog.V(4).Infof("Applying policy %s with %d rules\n", policy.ObjectMeta.Name, len(policy.Spec.Rules))

		engineResponse = engine.Mutate(*policy, *resource)
		policyInfo.AddRuleInfos(engineResponse.RuleInfos)
		// Gather policy application statistics
		gatherStat(policy.Name, engineResponse)

		// ps := policyctr.NewPolicyStat(policy.Name, engineResponse.ExecutionTime, nil, engineResponse.RulesAppliedCount)

		if !policyInfo.IsSuccessful() {
			glog.V(4).Infof("Failed to apply policy %s on resource %s/%s\n", policy.Name, resource.GetNamespace(), resource.GetName())
			glog.V(4).Info("Failed rule details")
			for _, r := range engineResponse.RuleInfos {
				glog.V(4).Infof("%s: %s\n", r.Name, r.Msgs)
			}
			continue
		}

		patches = append(patches, engineResponse.Patches...)
		policyInfos = append(policyInfos, policyInfo)
		glog.V(4).Infof("Mutation from policy %s has applied succesfully to %s %s/%s", policy.Name, request.Kind.Kind, resource.GetNamespace(), resource.GetName())

	}

	// ADD ANNOTATIONS
	// ADD EVENTS
	if len(patches) > 0 {
		eventsInfo := newEventInfoFromPolicyInfo(policyInfos, (request.Operation == v1beta1.Update), info.Mutation)
		ws.eventGen.Add(eventsInfo...)

		annotation := prepareAnnotationPatches(resource, policyInfos)
		patches = append(patches, annotation)
	}

	ok, msg := isAdmSuccesful(policyInfos)
	// Send policy engine Stats
	if ok {
		sendStat(false)
		engineResponse.Patches = patches
		return true, engineResponse
	}

	sendStat(true)
	glog.Errorf("Failed to mutate the resource: %s\n", msg)
	return false, engineResponse
}
