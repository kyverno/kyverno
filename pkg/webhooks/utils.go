package webhooks

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	"github.com/nirmata/kyverno/pkg/engine"
)

const policyKind = "Policy"

// func isAdmSuccesful(policyInfos []info.PolicyInfo) (bool, string) {
// 	var admSuccess = true
// 	var errMsgs []string
// 	for _, pi := range policyInfos {
// 		if !pi.IsSuccessful() {
// 			admSuccess = false
// 			errMsgs = append(errMsgs, fmt.Sprintf("\nPolicy %s failed with following rules", pi.Name))
// 			// Get the error rules
// 			errorRules := pi.ErrorRules()
// 			errMsgs = append(errMsgs, errorRules)
// 		}
// 	}
// 	return admSuccess, strings.Join(errMsgs, ";")
// }

func isResponseSuccesful(engineReponses []engine.EngineResponseNew) bool {
	for _, er := range engineReponses {
		if !er.IsSuccesful() {
			return false
		}
	}
	return true
}

// returns true -> if there is even one policy that blocks resource requst
// returns false -> if all the policies are meant to report only, we dont block resource request
func toBlockResource(engineReponses []engine.EngineResponseNew) bool {
	for _, er := range engineReponses {
		if er.PolicyResponse.ValidationFailureAction != ReportViolation {
			glog.V(4).Infof("ValidationFailureAction set to enforce for policy %s , blocking resource ceation", er.PolicyResponse.Policy)
			return true
		}
	}
	glog.V(4).Infoln("ValidationFailureAction set to audit, allowing resource creation, reporting with violation")
	return false
}

func getErrorMsg(engineReponses []engine.EngineResponseNew) string {
	var str []string
	for _, er := range engineReponses {
		if !er.IsSuccesful() {
			str = append(str, fmt.Sprintf("failed policy %s", er.PolicyResponse.Policy))
			for _, rule := range er.PolicyResponse.Rules {
				if !rule.Success {
					str = append(str, rule.ToString())
				}
			}
		}
	}
	return strings.Join(str, "\n")
}

//ArrayFlags to store filterkinds
type ArrayFlags []string

func (i *ArrayFlags) String() string {
	var sb strings.Builder
	for _, str := range *i {
		sb.WriteString(str)
	}
	return sb.String()
}

//Set setter for array flags
func (i *ArrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

// extract the kinds that the policy rules apply to
func getApplicableKindsForPolicy(p *kyverno.ClusterPolicy) []string {
	kindsMap := map[string]interface{}{}
	kinds := []string{}
	// iterate over the rules an identify all kinds
	// Matching
	for _, rule := range p.Spec.Rules {
		for _, k := range rule.MatchResources.Kinds {
			kindsMap[k] = nil
		}
		// remove excluded ones
		for _, k := range rule.ExcludeResources.Kinds {
			if _, ok := kindsMap[k]; ok {
				// delete kind
				delete(kindsMap, k)
			}
		}
	}

	// get the kinds
	for k := range kindsMap {
		kinds = append(kinds, k)
	}
	return kinds
}

// Policy Reporting Modes
const (
	BlockChanges    = "enforce"
	ReportViolation = "audit"
)

// // returns true -> if there is even one policy that blocks resource requst
// // returns false -> if all the policies are meant to report only, we dont block resource request
// func toBlock(pis []info.PolicyInfo) bool {
// 	for _, pi := range pis {
// 		if pi.ValidationFailureAction != ReportViolation {
// 			glog.V(3).Infoln("ValidationFailureAction set to enforce, blocking resource ceation")
// 			return true
// 		}
// 	}
// 	glog.V(3).Infoln("ValidationFailureAction set to audit, allowing resource creation, reporting with violation")
// 	return false
// }

func processResourceWithPatches(patch []byte, resource []byte) []byte {
	if patch == nil {
		return nil
	}
	glog.Info(string(resource))
	resource, err := engine.ApplyPatchNew(resource, patch)
	if err != nil {
		glog.Errorf("failed to patch resource: %v", err)
		return nil
	}
	return resource
}
