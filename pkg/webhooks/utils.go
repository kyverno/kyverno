package webhooks

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/golang/glog"
	"github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	"github.com/nirmata/kyverno/pkg/info"
	v1beta1 "k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const policyKind = "Policy"

func isAdmSuccesful(policyInfos []*info.PolicyInfo) (bool, string) {
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

//StringInSlice checks if string is present in slice of strings
func StringInSlice(kind string, list []string) bool {
	for _, b := range list {
		if b == kind {
			return true
		}
	}
	return false
}

//parseKinds parses the kinds if a single string contains comma seperated kinds
// {"1,2,3","4","5"} => {"1","2","3","4","5"}
func parseKinds(list []string) []string {
	kinds := []string{}
	for _, k := range list {
		args := strings.Split(k, ",")
		for _, arg := range args {
			if arg != "" {
				kinds = append(kinds, strings.TrimSpace(arg))
			}
		}
	}
	return kinds
}

type ArrayFlags []string

func (i *ArrayFlags) String() string {
	var sb strings.Builder
	for _, str := range *i {
		sb.WriteString(str)
	}
	return sb.String()
}

func (i *ArrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

// extract the kinds that the policy rules apply to
func getApplicableKindsForPolicy(p *v1alpha1.Policy) []string {
	kindsMap := map[string]interface{}{}
	kinds := []string{}
	// iterate over the rules an identify all kinds
	for _, rule := range p.Spec.Rules {
		for _, k := range rule.ResourceDescription.Kinds {
			kindsMap[k] = nil
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
	BlockChanges    = "block"
	ReportViolation = "report"
)

// returns true -> if there is even one policy that blocks resource requst
// returns false -> if all the policies are meant to report only, we dont block resource request
func toBlock(pis []*info.PolicyInfo) bool {
	for _, pi := range pis {
		if pi.ValidationFailureAction != ReportViolation {
			return true
		}
	}
	return false
}

func checkIfOnlyAnnotationsUpdate(request *v1beta1.AdmissionRequest) bool {
	// process only if its for existing resources
	if request.Operation != v1beta1.Update {
		return false
	}
	// updated resoruce
	obj := request.Object
	objUnstr := unstructured.Unstructured{}
	err := objUnstr.UnmarshalJSON(obj.Raw)
	if err != nil {
		glog.Error(err)
		return false
	}
	objUnstr.SetAnnotations(nil)
	objUnstr.SetGeneration(0)
	oldobj := request.OldObject
	oldobjUnstr := unstructured.Unstructured{}
	err = oldobjUnstr.UnmarshalJSON(oldobj.Raw)
	if err != nil {
		glog.Error(err)
		return false
	}
	oldobjUnstr.SetAnnotations(nil)
	oldobjUnstr.SetGeneration(0)
	if reflect.DeepEqual(objUnstr, oldobjUnstr) {
		return true
	}

	return false
}
