package webhooks

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	"github.com/nirmata/kyverno/pkg/engine"
)

func isResponseSuccesful(engineReponses []engine.EngineResponseNew) bool {
	for _, er := range engineReponses {
		if !er.IsSuccesful() {
			return false
		}
	}
	return true
}

// returns true -> if there is even one policy that blocks resource request
// returns false -> if all the policies are meant to report only, we dont block resource request
func toBlockResource(engineReponses []engine.EngineResponseNew) bool {
	for _, er := range engineReponses {
		if er.PolicyResponse.ValidationFailureAction == Enforce {
			glog.V(4).Infof("ValidationFailureAction set to enforce for policy %s , blocking resource request ", er.PolicyResponse.Policy)
			return true
		}
	}
	glog.V(4).Infoln("ValidationFailureAction set to audit, allowing resource request, reporting with policy violation")
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
	kinds := []string{}
	// iterate over the rules an identify all kinds
	// Matching
	for _, rule := range p.Spec.Rules {
		for _, k := range rule.MatchResources.Kinds {
			kinds = append(kinds, k)
		}
	}
	return kinds
}

// Policy Reporting Modes
const (
	Enforce = "enforce" // blocks the request on failure
	Audit   = "audit"   // dont block the request on failure, but report failiures as policy violations
)

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
