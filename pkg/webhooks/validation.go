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

// HandleValidation handles validating webhook admission request
// If there are no errors in validating rule we apply generation rules
func (ws *WebhookServer) HandleValidation(request *v1beta1.AdmissionRequest) *v1beta1.AdmissionResponse {
	policyInfos := []*info.PolicyInfo{}
	// var violations []*violation.Info
	// var eventsInfo []*event.Info
	policies, err := ws.policyLister.List(labels.NewSelector())
	if err != nil {
		// Unable to connect to policy Lister to access policies
		glog.Error("Unable to connect to policy controller to access policies. Validation Rules are NOT being applied")
		glog.Warning(err)
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	rname := engine.ParseNameFromObject(request.Object.Raw)
	rns := engine.ParseNamespaceFromObject(request.Object.Raw)
	rkind := engine.ParseKindFromObject(request.Object.Raw)

	var annPatches []byte
	for _, policy := range policies {

		if !StringInSlice(request.Kind.Kind, getApplicableKindsForPolicy(policy)) {
			continue
		}

		policyInfo := info.NewPolicyInfo(policy.Name,
			rkind,
			rname,
			rns,
			policy.Spec.ValidationFailureAction)

		glog.V(3).Infof("Handling validation for Kind=%s, Namespace=%s Name=%s UID=%s patchOperation=%s",
			request.Kind.Kind, rns, rname, request.UID, request.Operation)

		glog.Infof("Validating resource %s/%s/%s with policy %s with %d rules", rkind, rns, rname, policy.ObjectMeta.Name, len(policy.Spec.Rules))
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
			glog.Infof("Failed to apply policy %s on resource %s/%s", policy.Name, rname, rns)
			for _, r := range ruleInfos {
				glog.Warning(r.Msgs)
			}
		} else {
			// CleanUp Violations if exists
			err := ws.violationBuilder.RemoveInactiveViolation(policy.Name, request.Kind.Kind, rns, rname, info.Validation)
			if err != nil {
				glog.Info(err)
			}

			if len(ruleInfos) > 0 {
				glog.Infof("Validation from policy %s has applied succesfully to %s %s/%s", policy.Name, request.Kind.Kind, rname, rns)
			}
		}
		policyInfos = append(policyInfos, policyInfo)
		annPatch := addAnnotationsToResource(request.Object.Raw, policyInfo, info.Validation)
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

	if len(policyInfos) > 0 && len(policyInfos[0].Rules) != 0 {
		eventsInfo, violations := newEventInfoFromPolicyInfo(policyInfos, (request.Operation == v1beta1.Update), info.Validation)
		// If the validationFailureAction flag is set "report",
		// then we dont block the request and report the violations
		ws.violationBuilder.Add(violations...)
		ws.eventController.Add(eventsInfo...)
	}
	//	add annotations
	if annPatches != nil {
		ws.annotationsController.Add(rkind, rns, rname, annPatches)
	}
	// If Validation fails then reject the request
	ok, msg := isAdmSuccesful(policyInfos)
	// violations are created if "report" flag is set
	// and if there are any then we dont bock the resource creation
	// Even if one the policy being applied

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
