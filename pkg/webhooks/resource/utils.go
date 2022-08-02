package resource

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/engine"
	enginectx "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"github.com/kyverno/kyverno/pkg/utils"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	engineutils2 "github.com/kyverno/kyverno/pkg/utils/engine"
	"github.com/kyverno/kyverno/pkg/webhooks/updaterequest"
	"github.com/pkg/errors"
	yamlv2 "gopkg.in/yaml.v2"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type updateRequestResponse struct {
	ur  kyvernov1beta1.UpdateRequestSpec
	err error
}

func excludeKyvernoResources(kind string) bool {
	switch kind {
	case "ClusterPolicyReport":
		return true
	case "PolicyReport":
		return true
	case "ReportChangeRequest":
		return true
	case "GenerateRequest":
		return true
	case "ClusterReportChangeRequest":
		return true
	default:
		return false
	}
}

func errorResponse(logger logr.Logger, err error, message string) *admissionv1.AdmissionResponse {
	logger.Error(err, message)
	return admissionutils.ResponseFailure(message + ": " + err.Error())
}

func patchRequest(patches []byte, request *admissionv1.AdmissionRequest, logger logr.Logger) *admissionv1.AdmissionRequest {
	patchedResource := processResourceWithPatches(patches, request.Object.Raw, logger)
	newRequest := request.DeepCopy()
	newRequest.Object.Raw = patchedResource
	return newRequest
}

func processResourceWithPatches(patch []byte, resource []byte, log logr.Logger) []byte {
	if patch == nil {
		return resource
	}
	resource, err := engineutils.ApplyPatchNew(resource, patch)
	if err != nil {
		log.Error(err, "failed to patch resource:", "patch", string(patch), "resource", string(resource))
		return nil
	}
	log.V(6).Info("", "patchedResource", string(resource))
	return resource
}

func newVariablesContext(request *admissionv1.AdmissionRequest, userRequestInfo *kyvernov1beta1.RequestInfo) (enginectx.Interface, error) {
	ctx := enginectx.NewContext()
	if err := ctx.AddRequest(request); err != nil {
		return nil, errors.Wrap(err, "failed to load incoming request in context")
	}
	if err := ctx.AddUserInfo(*userRequestInfo); err != nil {
		return nil, errors.Wrap(err, "failed to load userInfo in context")
	}
	if err := ctx.AddServiceAccount(userRequestInfo.AdmissionUserInfo.Username); err != nil {
		return nil, errors.Wrap(err, "failed to load service account in context")
	}
	return ctx, nil
}

func containsRBACInfo(policies ...[]kyvernov1.PolicyInterface) bool {
	for _, policySlice := range policies {
		for _, policy := range policySlice {
			for _, rule := range autogen.ComputeRules(policy) {
				if checkForRBACInfo(rule) {
					return true
				}
			}
		}
	}
	return false
}

func checkForRBACInfo(rule kyvernov1.Rule) bool {
	if len(rule.MatchResources.Roles) > 0 || len(rule.MatchResources.ClusterRoles) > 0 || len(rule.ExcludeResources.Roles) > 0 || len(rule.ExcludeResources.ClusterRoles) > 0 {
		return true
	}
	if len(rule.MatchResources.All) > 0 {
		for _, rf := range rule.MatchResources.All {
			if len(rf.UserInfo.Roles) > 0 || len(rf.UserInfo.ClusterRoles) > 0 {
				return true
			}
		}
	}
	if len(rule.MatchResources.Any) > 0 {
		for _, rf := range rule.MatchResources.Any {
			if len(rf.UserInfo.Roles) > 0 || len(rf.UserInfo.ClusterRoles) > 0 {
				return true
			}
		}
	}
	if len(rule.ExcludeResources.All) > 0 {
		for _, rf := range rule.ExcludeResources.All {
			if len(rf.UserInfo.Roles) > 0 || len(rf.UserInfo.ClusterRoles) > 0 {
				return true
			}
		}
	}
	if len(rule.ExcludeResources.Any) > 0 {
		for _, rf := range rule.ExcludeResources.Any {
			if len(rf.UserInfo.Roles) > 0 || len(rf.UserInfo.ClusterRoles) > 0 {
				return true
			}
		}
	}
	return false
}

func buildDeletionPrInfo(oldR unstructured.Unstructured) policyreport.Info {
	return policyreport.Info{
		Namespace: oldR.GetNamespace(),
		Results: []policyreport.EngineResponseResult{
			{Resource: response.ResourceSpec{
				Kind:       oldR.GetKind(),
				APIVersion: oldR.GetAPIVersion(),
				Namespace:  oldR.GetNamespace(),
				Name:       oldR.GetName(),
				UID:        string(oldR.GetUID()),
			}},
		},
	}
}

func convertResource(request *admissionv1.AdmissionRequest, resourceRaw []byte) (unstructured.Unstructured, error) {
	resource, err := utils.ConvertResource(resourceRaw, request.Kind.Group, request.Kind.Version, request.Kind.Kind, request.Namespace)
	if err != nil {
		return unstructured.Unstructured{}, errors.Wrap(err, "failed to convert raw resource to unstructured format")
	}
	if request.Kind.Kind == "Secret" && request.Operation == admissionv1.Update {
		resource, err = utils.NormalizeSecret(&resource)
		if err != nil {
			return unstructured.Unstructured{}, errors.Wrap(err, "failed to convert secret to unstructured format")
		}
	}
	return resource, nil
}

// returns true -> if there is even one policy that blocks resource request
// returns false -> if all the policies are meant to report only, we dont block resource request
func blockRequest(engineReponses []*response.EngineResponse, failurePolicy kyvernov1.FailurePolicyType, log logr.Logger) bool {
	for _, er := range engineReponses {
		if engineutils2.BlockRequest(er, failurePolicy) {
			log.Info("blocking admission request", "policy", er.PolicyResponse.Policy.Name)
			return true
		}
	}

	log.V(4).Info("allowing admission request")
	return false
}

// getBlockedMessages gets the error messages for rules with error or fail status
func getBlockedMessages(engineResponses []*response.EngineResponse) string {
	if len(engineResponses) == 0 {
		return ""
	}

	failures := make(map[string]interface{})
	hasViolations := false
	for _, er := range engineResponses {
		ruleToReason := make(map[string]string)
		for _, rule := range er.PolicyResponse.Rules {
			if rule.Status != response.RuleStatusPass {
				ruleToReason[rule.Name] = rule.Message
				if rule.Status == response.RuleStatusFail {
					hasViolations = true
				}
			}
		}

		failures[er.PolicyResponse.Policy.Name] = ruleToReason
	}

	if len(failures) == 0 {
		return ""
	}

	r := engineResponses[0].PolicyResponse.Resource
	resourceName := fmt.Sprintf("%s/%s/%s", r.Kind, r.Namespace, r.Name)
	action := getAction(hasViolations, len(failures))

	results, _ := yamlv2.Marshal(failures)
	msg := fmt.Sprintf("\n\npolicy %s for resource %s: \n\n%s", resourceName, action, results)
	return msg
}

func getWarningMessages(engineResponses []*response.EngineResponse) []string {
	var warnings []string
	for _, er := range engineResponses {
		for _, rule := range er.PolicyResponse.Rules {
			if rule.Status != response.RuleStatusPass {
				msg := fmt.Sprintf("policy %s.%s: %s", er.Policy.GetName(), rule.Name, rule.Message)
				warnings = append(warnings, msg)
			}
		}
	}

	return warnings
}

func getAction(hasViolations bool, i int) string {
	action := "error"
	if hasViolations {
		action = "violation"
	}

	if i > 1 {
		action = action + "s"
	}

	return action
}

func getErrorMsg(engineReponses []*response.EngineResponse) string {
	var str []string
	var resourceInfo string
	for _, er := range engineReponses {
		if !er.IsSuccessful() {
			// resource in engineReponses is identical as this was called per admission request
			resourceInfo = fmt.Sprintf("%s/%s/%s", er.PolicyResponse.Resource.Kind, er.PolicyResponse.Resource.Namespace, er.PolicyResponse.Resource.Name)
			str = append(str, fmt.Sprintf("failed policy %s:", er.PolicyResponse.Policy.Name))
			for _, rule := range er.PolicyResponse.Rules {
				if rule.Status != response.RuleStatusPass {
					str = append(str, rule.ToString())
				}
			}
		}
	}
	return fmt.Sprintf("Resource %s %s", resourceInfo, strings.Join(str, ";"))
}

func hasAnnotations(context *engine.PolicyContext) bool {
	annotations := context.NewResource.GetAnnotations()
	return len(annotations) != 0
}

func getGeneratedByResource(newRes *unstructured.Unstructured, resLabels map[string]string, client dclient.Interface, rule kyvernov1.Rule, logger logr.Logger) (kyvernov1.Rule, error) {
	var apiVersion, kind, name, namespace string
	sourceRequest := &admissionv1.AdmissionRequest{}
	kind = resLabels["kyverno.io/generated-by-kind"]
	name = resLabels["kyverno.io/generated-by-name"]
	if kind != "Namespace" {
		namespace = resLabels["kyverno.io/generated-by-namespace"]
	}
	obj, err := client.GetResource(apiVersion, kind, namespace, name)
	if err != nil {
		logger.Error(err, "source resource not found.")
		return rule, err
	}
	rawObj, err := json.Marshal(obj)
	if err != nil {
		logger.Error(err, "failed to marshal resource")
		return rule, err
	}
	sourceRequest.Object.Raw = rawObj
	sourceRequest.Operation = "CREATE"
	ctx := enginectx.NewContext()
	if err := ctx.AddRequest(sourceRequest); err != nil {
		logger.Error(err, "failed to load incoming request in context")
		return rule, err
	}
	if rule, err = variables.SubstituteAllInRule(logger, ctx, rule); err != nil {
		logger.Error(err, "variable substitution failed for rule %s", rule.Name)
		return rule, err
	}
	return rule, nil
}

// stripNonPolicyFields - remove feilds which get updated with each request by kyverno and are non policy fields
func stripNonPolicyFields(obj, newRes map[string]interface{}, logger logr.Logger) (map[string]interface{}, map[string]interface{}) {
	if metadata, found := obj["metadata"]; found {
		requiredMetadataInObj := make(map[string]interface{})
		if annotations, found := metadata.(map[string]interface{})["annotations"]; found {
			delete(annotations.(map[string]interface{}), "kubectl.kubernetes.io/last-applied-configuration")
			requiredMetadataInObj["annotations"] = annotations
		}

		if labels, found := metadata.(map[string]interface{})["labels"]; found {
			delete(labels.(map[string]interface{}), "generate.kyverno.io/clone-policy-name")
			requiredMetadataInObj["labels"] = labels
		}
		obj["metadata"] = requiredMetadataInObj
	}

	if metadata, found := newRes["metadata"]; found {
		requiredMetadataInNewRes := make(map[string]interface{})
		if annotations, found := metadata.(map[string]interface{})["annotations"]; found {
			requiredMetadataInNewRes["annotations"] = annotations
		}

		if labels, found := metadata.(map[string]interface{})["labels"]; found {
			requiredMetadataInNewRes["labels"] = labels
		}
		newRes["metadata"] = requiredMetadataInNewRes
	}

	delete(obj, "status")

	if _, found := obj["spec"]; found {
		delete(obj["spec"].(map[string]interface{}), "tolerations")
	}

	if dataMap, found := obj["data"]; found {
		keyInData := make([]string, 0)
		switch dataMap := dataMap.(type) {
		case map[string]interface{}:
			for k := range dataMap {
				keyInData = append(keyInData, k)
			}
		}

		if len(keyInData) > 0 {
			for _, dataKey := range keyInData {
				originalResourceData := dataMap.(map[string]interface{})[dataKey]
				replaceData := strings.Replace(originalResourceData.(string), "\n", "", -1)
				dataMap.(map[string]interface{})[dataKey] = replaceData

				newResourceData := newRes["data"].(map[string]interface{})[dataKey]
				replacenewResourceData := strings.Replace(newResourceData.(string), "\n", "", -1)
				newRes["data"].(map[string]interface{})[dataKey] = replacenewResourceData
			}
		} else {
			logger.V(4).Info("data is not of type map[string]interface{}")
		}
	}

	return obj, newRes
}

func applyUpdateRequest(request *admissionv1.AdmissionRequest, ruleType kyvernov1beta1.RequestType, grGenerator updaterequest.Generator, userRequestInfo kyvernov1beta1.RequestInfo,
	action admissionv1.Operation, engineResponses ...*response.EngineResponse,
) (failedUpdateRequest []updateRequestResponse) {
	admissionRequestInfo := kyvernov1beta1.AdmissionRequestInfoObject{
		AdmissionRequest: request,
		Operation:        action,
	}

	for _, er := range engineResponses {
		ur := transform(admissionRequestInfo, userRequestInfo, er, ruleType)
		if err := grGenerator.Apply(ur, action); err != nil {
			failedUpdateRequest = append(failedUpdateRequest, updateRequestResponse{ur: ur, err: err})
		}
	}

	return
}

func transform(admissionRequestInfo kyvernov1beta1.AdmissionRequestInfoObject, userRequestInfo kyvernov1beta1.RequestInfo, er *response.EngineResponse, ruleType kyvernov1beta1.RequestType) kyvernov1beta1.UpdateRequestSpec {
	var PolicyNameNamespaceKey string
	if er.PolicyResponse.Policy.Namespace != "" {
		PolicyNameNamespaceKey = er.PolicyResponse.Policy.Namespace + "/" + er.PolicyResponse.Policy.Name
	} else {
		PolicyNameNamespaceKey = er.PolicyResponse.Policy.Name
	}

	ur := kyvernov1beta1.UpdateRequestSpec{
		Type:   ruleType,
		Policy: PolicyNameNamespaceKey,
		Resource: kyvernov1.ResourceSpec{
			Kind:       er.PolicyResponse.Resource.Kind,
			Namespace:  er.PolicyResponse.Resource.Namespace,
			Name:       er.PolicyResponse.Resource.Name,
			APIVersion: er.PolicyResponse.Resource.APIVersion,
		},
		Context: kyvernov1beta1.UpdateRequestSpecContext{
			UserRequestInfo:      userRequestInfo,
			AdmissionRequestInfo: admissionRequestInfo,
		},
	}

	return ur
}
