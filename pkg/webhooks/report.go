package webhooks

import (
	"reflect"
	"strings"

	"github.com/nirmata/kyverno/pkg/annotations"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/clientNew/clientset/versioned"
	"github.com/nirmata/kyverno/pkg/violation"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/info"
	"github.com/nirmata/kyverno/pkg/policyviolation"
)

//TODO: change validation from bool -> enum(validation, mutation)
func newEventInfoFromPolicyInfo(policyInfoList []info.PolicyInfo, onUpdate bool, ruleType info.RuleType) ([]*event.Info, []*violation.Info) {
	var eventsInfo []*event.Info
	var violations []*violation.Info
	ok, msg := isAdmSuccesful(policyInfoList)
	// Some policies failed to apply succesfully
	if !ok {
		for _, pi := range policyInfoList {
			if pi.IsSuccessful() {
				continue
			}
			rules := pi.FailedRules()
			ruleNames := strings.Join(rules, ";")
			if !onUpdate {
				// CREATE
				eventsInfo = append(eventsInfo,
					event.NewEvent(policyKind, "", pi.Name, event.RequestBlocked, event.FPolicyApplyBlockCreate, pi.RName, ruleNames))

				glog.V(3).Infof("Rule(s) %s of policy %s blocked resource creation, error: %s\n", ruleNames, pi.Name, msg)
			} else {
				// UPDATE
				eventsInfo = append(eventsInfo,
					event.NewEvent(pi.RKind, pi.RNamespace, pi.RName, event.RequestBlocked, event.FPolicyApplyBlockUpdate, ruleNames, pi.Name))
				eventsInfo = append(eventsInfo,
					event.NewEvent(policyKind, "", pi.Name, event.RequestBlocked, event.FPolicyBlockResourceUpdate, pi.RName, ruleNames))
				glog.V(3).Infof("Request blocked events info has prepared for %s/%s and %s/%s\n", policyKind, pi.Name, pi.RKind, pi.RName)
			}
			// if report flag is set
			if pi.ValidationFailureAction == ReportViolation && ruleType == info.Validation {
				// Create Violations
				v := violation.BuldNewViolation(pi.Name, pi.RKind, pi.RNamespace, pi.RName, event.PolicyViolation.String(), pi.GetFailedRules())
				violations = append(violations, v)
			}
		}
	} else {
		if !onUpdate {
			// All policies were applied succesfully
			// CREATE
			for _, pi := range policyInfoList {
				rules := pi.SuccessfulRules()
				ruleNames := strings.Join(rules, ";")
				eventsInfo = append(eventsInfo,
					event.NewEvent(pi.RKind, pi.RNamespace, pi.RName, event.PolicyApplied, event.SRulesApply, ruleNames, pi.Name))

				glog.V(3).Infof("Success event info has prepared for %s/%s\n", pi.RKind, pi.RName)
			}
		}
	}
	return eventsInfo, violations
}

func addAnnotationsToResource(rawResource []byte, pi *info.PolicyInfo, ruleType info.RuleType) []byte {
	if len(pi.Rules) == 0 {
		return nil
	}
	// get annotations
	ann := annotations.ParseAnnotationsFromObject(rawResource)
	ann, patch, err := annotations.AddPolicyJSONPatch(ann, pi, ruleType)
	if err != nil {
		glog.Error(err)
		return nil
	}
	return patch
}

//buildAnnotation we add annotations for the successful application of JSON patches
//TODO
func buildAnnotation(mAnn map[string]string, pi *info.PolicyInfo) {
	if len(pi.Rules) == 0 {
		return
	}
	var mchanges []string

	for _, r := range pi.Rules {
		if r.Changes != "" {
			// the rule generate a patch
			// key policy name will be updated to right format during creation of annotations
			mchanges = append(mchanges)
		}
	}
}

// buildPolicyViolationsForAPolicy returns a policy violation object if there are any rules that fail
func buildPolicyViolationsForAPolicy(pi info.PolicyInfo) kyverno.PolicyViolation {
	var fRules []kyverno.ViolatedRule
	var pv kyverno.PolicyViolation
	for _, r := range pi.Rules {
		if !r.IsSuccessful() {
			fRules = append(fRules, kyverno.ViolatedRule{Name: r.Name, Message: r.GetErrorString(), Type: r.RuleType.String()})
		}
	}
	if len(fRules) > 0 {
		glog.V(4).Infof("building policy violation for policy %s on resource %s/%s/%s", pi.Name, pi.RKind, pi.RNamespace, pi.RName)
		// there is an error
		pv = policyviolation.BuildPolicyViolation(pi.Name, kyverno.ResourceSpec{
			Kind:      pi.RKind,
			Namespace: pi.RNamespace,
			Name:      pi.RName,
		},
			fRules,
		)

	}
	return pv
}

//generatePolicyViolations generate policyViolation resources for the rules that failed
func generatePolicyViolations(client *kyvernoclient.Clientset, policyInfos []info.PolicyInfo) {
	var pvs []kyverno.PolicyViolation
	for _, policyInfo := range policyInfos {
		if !policyInfo.IsSuccessful() {
			if pv := buildPolicyViolationsForAPolicy(policyInfo); !reflect.DeepEqual(pv, kyverno.PolicyViolation{}) {
				pvs = append(pvs, pv)
			}
		}
	}

	if len(pvs) > 0 {
		for _, newPv := range pvs {
			// generate PolicyViolation objects
			glog.V(4).Infof("creating policyViolation resource for policy %s and resource %s/%s/%s", newPv.Spec.Policy, newPv.Spec.Kind, newPv.Spec.Namespace, newPv.Spec.Name)

			// check if there was a previous violation for policy & resource combination
			//TODO: check for existing ov using label selectors on resource and policy
			// curPv, err:= client.KyvernoV1alpha1().PolicyViolations().

			// _, err := client.KyvernoV1alpha1().PolicyViolations().Create(&newPv)
			// // _, err := client.CreateResource("PolicyViolation", "", &pv, false)
			// if err != nil {
			// 	glog.Error(err)
			// 	continue
			// }
		}
	}
}
