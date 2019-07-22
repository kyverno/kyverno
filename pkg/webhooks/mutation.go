package webhooks

import (
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/golang/glog"
	engine "github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/info"
	v1beta1 "k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// HandleMutation handles mutating webhook admission request
func (ws *WebhookServer) HandleMutation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {

	glog.V(3).Infof("Handling mutation for Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
		request.Kind.Kind, request.Namespace, request.Name, request.UID, request.Operation)

	policies, err := ws.policyLister.List(labels.NewSelector())
	if err != nil {
		// Unable to connect to policy Lister to access policies
		glog.Error("Unable to connect to policy controller to access policies. Mutation Rules are NOT being applied")
		glog.Warning(err)
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}
	rname := engine.ParseNameFromObject(request.Object.Raw)
	rns := engine.ParseNamespaceFromObject(request.Object.Raw)
	rkind := request.Kind.Kind
	if rkind == "" {
		glog.Errorf("failed to parse KIND from request: Namespace=%s Name=%s UID=%s patchOperation=%s\n", request.Namespace, request.Name, request.UID, request.Operation)
	}

	var allPatches [][]byte
	var annPatches []byte
	policyInfos := []*info.PolicyInfo{}
	for _, policy := range policies {
		// check if policy has a rule for the admission request kind
		if !StringInSlice(request.Kind.Kind, getApplicableKindsForPolicy(policy)) {
			continue
		}
		//TODO: HACK Check if an update of annotations
		if checkIfOnlyAnnotationsUpdate(request) {
			return &v1beta1.AdmissionResponse{
				Allowed: true,
			}
		}
		policyInfo := info.NewPolicyInfo(policy.Name,
			rkind,
			rname,
			rns,
			policy.Spec.ValidationFailureAction)

		glog.V(3).Infof("Handling mutation for Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
			request.Kind.Kind, rns, rname, request.UID, request.Operation)

		glog.Infof("Applying policy %s with %d rules\n", policy.ObjectMeta.Name, len(policy.Spec.Rules))

		policyPatches, ruleInfos := engine.Mutate(*policy, request.Object.Raw, request.Kind)

		policyInfo.AddRuleInfos(ruleInfos)

		if !policyInfo.IsSuccessful() {
			glog.Infof("Failed to apply policy %s on resource %s/%s", policy.Name, rname, rns)
			for _, r := range ruleInfos {
				glog.Warningf("%s: %s\n", r.Name, r.Msgs)
			}
		} else {
			// CleanUp Violations if exists
			err := ws.violationBuilder.RemoveInactiveViolation(policy.Name, request.Kind.Kind, rns, rname, info.Mutation)
			if err != nil {
				glog.Info(err)
			}
			allPatches = append(allPatches, policyPatches...)
			glog.Infof("Mutation from policy %s has applied succesfully to %s %s/%s", policy.Name, request.Kind.Kind, rname, rns)
		}
		policyInfos = append(policyInfos, policyInfo)

		annPatch := addAnnotationsToResource(request.Object.Raw, policyInfo, info.Mutation)
		if annPatch != nil {
			if annPatches == nil {
				annPatches = annPatch
			} else {
				annPatches, err = jsonpatch.MergePatch(annPatches, annPatch)
				if err != nil {
					glog.Error(err)
				}
			}
		}
	}

	if len(allPatches) > 0 {
		eventsInfo, _ := newEventInfoFromPolicyInfo(policyInfos, (request.Operation == v1beta1.Update), info.Mutation)
		ws.eventController.Add(eventsInfo...)
	}
	//	add annotations
	if annPatches != nil {
		// fmt.Println(string(annPatches))
		ws.annotationsController.Add(rkind, rns, rname, annPatches)
	}

	ok, msg := isAdmSuccesful(policyInfos)
	if ok {
		patchType := v1beta1.PatchTypeJSONPatch
		return &v1beta1.AdmissionResponse{
			Allowed:   true,
			Patch:     engine.JoinPatches(allPatches),
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
