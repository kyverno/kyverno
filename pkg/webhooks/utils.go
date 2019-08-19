package webhooks

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	"github.com/nirmata/kyverno/pkg/info"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const policyKind = "Policy"

func isAdmSuccesful(policyInfos []info.PolicyInfo) (bool, string) {
	var admSuccess = true
	var errMsgs []string
	for _, pi := range policyInfos {
		if !pi.IsSuccessful() {
			admSuccess = false
			errMsgs = append(errMsgs, fmt.Sprintf("\nPolicy %s failed with following rules", pi.Name))
			// Get the error rules
			errorRules := pi.ErrorRules()
			errMsgs = append(errMsgs, errorRules)
		}
	}
	return admSuccess, strings.Join(errMsgs, ";")
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
func getApplicableKindsForPolicy(p *kyverno.Policy) []string {
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

// returns true -> if there is even one policy that blocks resource requst
// returns false -> if all the policies are meant to report only, we dont block resource request
func toBlock(pis []info.PolicyInfo) bool {
	for _, pi := range pis {
		if pi.ValidationFailureAction != ReportViolation {
			return true
		}
	}
	return false
}

func convertToUnstructured(data []byte) (*unstructured.Unstructured, error) {
	resource := &unstructured.Unstructured{}
	err := resource.UnmarshalJSON(data)
	if err != nil {
		glog.V(4).Infof("failed to unmarshall resource: %v", err)
		return nil, err
	}
	return resource, nil
}
