package webhooks

import (
	"github.com/golang/glog"
	engine "github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/info"
	v1beta1 "k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
)

// HandleMutation handles mutating webhook admission request
func (ws *WebhookServer) HandleMutation(request *v1beta1.AdmissionRequest) (bool, *engine.EngineResponse) {
	var allPatches [][]byte
	policyInfos := []*info.PolicyInfo{}
	engineResponse := &engine.EngineResponse{PatchedDocument: request.Object.Raw}

	if request.Operation == v1beta1.Delete {
		return true, engineResponse
	}

	glog.V(4).Infof("Receive request in mutating webhook: Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
		request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation)

	policies, err := ws.policyLister.List(labels.NewSelector())
	if err != nil {
		// Unable to connect to policy Lister to access policies
		glog.Errorln("Unable to connect to policy controller to access policies. Mutation Rules are NOT being applied")
		glog.Warning(err)
		return true, engineResponse
	}

	rname := engine.ParseNameFromObject(request.Object.Raw)
	rns := engine.ParseNamespaceFromObject(request.Object.Raw)
	rkind := request.Kind.Kind
	if rkind == "" {
		glog.Errorf("failed to parse KIND from request: Namespace=%s Name=%s UID=%s patchOperation=%s\n", request.Namespace, request.Name, request.UID, request.Operation)
	}

	for _, policy := range policies {

		// check if policy has a rule for the admission request kind
		if !StringInSlice(request.Kind.Kind, getApplicableKindsForPolicy(policy)) {
			continue
		}
		//TODO: HACK Check if an update of annotations
		// if checkIfOnlyAnnotationsUpdate(request) {
		// 	return true
		// }

		policyInfo := info.NewPolicyInfo(policy.Name,
			rkind,
			rname,
			rns,
			policy.Spec.ValidationFailureAction)

		glog.V(3).Infof("Handling mutation for Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
			request.Kind.Kind, rns, rname, request.UID, request.Operation)

		glog.Infof("Applying policy %s with %d rules\n", policy.ObjectMeta.Name, len(policy.Spec.Rules))

		engineResponse = engine.Mutate(*policy, engineResponse.PatchedDocument, request.Kind)
		policyInfo.AddRuleInfos(engineResponse.RuleInfos)

		if !policyInfo.IsSuccessful() {
			glog.Infof("Failed to apply policy %s on resource %s/%s", policy.Name, rname, rns)
			for _, r := range engineResponse.RuleInfos {
				glog.Warningf("%s: %s\n", r.Name, r.Msgs)
			}
		} else {
			// CleanUp Violations if exists
			err := ws.violationBuilder.RemoveInactiveViolation(policy.Name, request.Kind.Kind, rns, rname, info.Mutation)
			if err != nil {
				glog.Info(err)
			}
			allPatches = append(allPatches, engineResponse.Patches...)
			glog.Infof("Mutation from policy %s has applied succesfully to %s %s/%s", policy.Name, request.Kind.Kind, rns, rname)
		}
		policyInfos = append(policyInfos, policyInfo)

		// annPatch := addAnnotationsToResource(patchedDocument, policyInfo, info.Mutation)
		// if annPatch != nil {
		// 	// add annotations
		// 	ws.annotationsController.Add(rkind, rns, rname, annPatch)
		// }
	}

	if len(allPatches) > 0 {
		eventsInfo, _ := newEventInfoFromPolicyInfo(policyInfos, (request.Operation == v1beta1.Update), info.Mutation)
		ws.eventController.Add(eventsInfo...)
	}

	ok, msg := isAdmSuccesful(policyInfos)
	if ok {
		engineResponse.Patches = allPatches
		return true, engineResponse
	}

	glog.Errorf("Failed to mutate the resource: %s\n", msg)
	return false, engineResponse
}
