package webhooks

import (
	"encoding/json"
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
	var err error
	// process only if its for existing resources
	if request.Operation != v1beta1.Update {
		return false
	}

	// approach : we only compare if the addition contains annotations the are added with prefix "policies.kyverno.io"
	// get annotations for the old resource
	oldObj := request.OldObject
	oldObjUnstr := unstructured.Unstructured{}
	// need to set kind as some request dont contain kind meta-data raw resource but in the api request
	oldObj.Raw = setKindForObject(oldObj.Raw, request.Kind.Kind)
	err = oldObjUnstr.UnmarshalJSON(oldObj.Raw)
	if err != nil {
		glog.Error(err)
		return false
	}
	oldAnn := oldObjUnstr.GetAnnotations()

	// get annotations for the new resource
	newObj := request.Object
	newObjUnstr := unstructured.Unstructured{}
	// need to set kind as some request dont contain kind meta-data raw resource but in the api request
	newObj.Raw = setKindForObject(newObj.Raw, request.Kind.Kind)
	err = newObjUnstr.UnmarshalJSON(newObj.Raw)
	if err != nil {
		glog.Error(err)
		return false
	}
	newAnn := newObjUnstr.GetAnnotations()
	policiesAppliedNew := 0
	newAnnPolicy := map[string]string{}
	// check if annotations changed
	// assuming that we only add an annotation with the given prefix
	for k, v := range newAnn {
		// check prefix
		policyName := strings.Split(k, "/")
		if len(policyName) == 1 {
			continue
		}
		if policyName[0] == "policies.kyverno.io" {
			newAnnPolicy[policyName[1]] = v
			policiesAppliedNew++
		}
	}

	oldAnnPolicy := map[string]string{}
	policiesAppliedOld := 0
	// check if annotations changed
	// assuming that we only add an annotation with the given prefix
	for k, v := range oldAnn {
		// check prefix
		policyName := strings.Split(k, "/")
		if len(policyName) == 1 {
			continue
		}
		if policyName[0] == "policies.kyverno.io" {
			oldAnnPolicy[policyName[1]] = v
			policiesAppliedOld++
		}
	}

	diffCount := policiesAppliedNew - policiesAppliedOld
	switch diffCount {
	case 1: // policy applied
		return true
	case -1: // policy removed
		return true
	case 0: // no new policy added or remove
		// need to check if the policy was updated
		if !reflect.DeepEqual(newAnnPolicy, oldAnnPolicy) {
			return true
		}
	}
	// then there is some other change and we should process it
	return false
}

func setKindForObject(bytes []byte, kind string) []byte {
	var objectJSON map[string]interface{}
	json.Unmarshal(bytes, &objectJSON)
	objectJSON["kind"] = kind
	data, err := json.Marshal(objectJSON)
	if err != nil {
		glog.Error(err)
		glog.Error("unable to marshall, not setting the kind")
		return bytes
	}
	return data
}

func setObserverdGenerationAsZero(bytes []byte) []byte {
	var objectJSON map[string]interface{}
	json.Unmarshal(bytes, &objectJSON)
	status, ok := objectJSON["status"].(map[string]interface{})
	if !ok {
		// glog.Error("status block not found, not setting observed generation")
		return bytes
	}
	status["observedGeneration"] = 0
	objectJSON["status"] = status
	data, err := json.Marshal(objectJSON)
	if err != nil {
		glog.Error(err)
		glog.Error("unable to marshall, not setting observed generation")
		return bytes
	}
	return data
}
