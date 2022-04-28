package webhooks

import (
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/autogen"
	enginectx "github.com/kyverno/kyverno/pkg/engine/context"
	"github.com/kyverno/kyverno/pkg/engine/response"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	engineutils2 "github.com/kyverno/kyverno/pkg/utils/engine"
	"github.com/pkg/errors"
	yamlv2 "gopkg.in/yaml.v2"
	admissionv1 "k8s.io/api/admission/v1"
)

// returns true -> if there is even one policy that blocks resource request
// returns false -> if all the policies are meant to report only, we dont block resource request
func toBlockResource(engineReponses []*response.EngineResponse, log logr.Logger) bool {
	for _, er := range engineReponses {
		if engineutils2.CheckEngineResponse(er) {
			log.Info("spec.ValidationFailureAction set to enforce, blocking resource request", "policy", er.PolicyResponse.Policy.Name)
			return true
		}
	}

	log.V(4).Info("spec.ValidationFailureAction set to audit for all applicable policies, won't block resource operation")
	return false
}

// getEnforceFailureErrorMsg gets the error messages for failed enforce policy
func getEnforceFailureErrorMsg(engineResponses []*response.EngineResponse) string {
	policyToRule := make(map[string]interface{})
	var resourceName string
	for _, er := range engineResponses {
		if engineutils2.CheckEngineResponse(er) {
			ruleToReason := make(map[string]string)
			for _, rule := range er.PolicyResponse.Rules {
				if rule.Status != response.RuleStatusPass {
					ruleToReason[rule.Name] = rule.Message
				}
			}
			resourceName = fmt.Sprintf("%s/%s/%s", er.PolicyResponse.Resource.Kind, er.PolicyResponse.Resource.Namespace, er.PolicyResponse.Resource.Name)
			policyToRule[er.PolicyResponse.Policy.Name] = ruleToReason
		}
	}
	result, _ := yamlv2.Marshal(policyToRule)
	return "\n\nresource " + resourceName + " was blocked due to the following policies\n\n" + string(result)
}

// getErrorMsg gets all failed engine response message
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

// patchRequest applies patches to the request.Object and returns a new copy of the request
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

func containsRBACInfo(policies ...[]kyverno.PolicyInterface) bool {
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

func checkForRBACInfo(rule kyverno.Rule) bool {
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

func newVariablesContext(request *admissionv1.AdmissionRequest, userRequestInfo *urkyverno.RequestInfo) (enginectx.Interface, error) {
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
